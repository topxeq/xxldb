// Package mysql provides MySQL protocol server for xxldb
package mysql

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/topxeq/xxldb/executor"
)

// preparedStatement represents a prepared statement
type preparedStatement struct {
	id       uint32
	query    string
	numParams int
	params   []paramDesc
	columns  []columnDesc
}

// paramDesc describes a parameter
type paramDesc struct {
	typ     byte
	signed  bool
}

// columnDesc describes a result column
type columnDesc struct {
	name     string
	typ      byte
	length   uint32
	charset  uint16
}

// Server represents a MySQL protocol server
type Server struct {
	engine     *executor.Engine
	listener   net.Listener
	clients    map[net.Conn]*clientConn
	mu         sync.RWMutex
	shutdown   bool
	serverID   uint32
	version    string
	charset    uint8
	capability uint32

	// Require authentication even when no password is set
	requireAuth bool

	// RSA key for caching_sha2_password full auth
	rsaKey    *rsa.PrivateKey
	rsaPubPEM []byte

	// Statement ID generator
	stmtIDCounter uint32
}

// Config holds server configuration
type Config struct {
	// Network address to listen on (default: ":3306")
	Addr string

	// Server version (default: "5.7.0")
	Version string

	// Server ID for replication
	ServerID uint32

	// Default charset (default: 33 = utf8)
	Charset uint8

	// Require authentication even when no password is set
	RequireAuth bool
}

// clientConn represents a client connection
type clientConn struct {
	conn       net.Conn
	salt       []byte
	capability uint32
	charset    uint8
	username   string
	database   string
	closed     bool

	// authSeq is the sequence number to use for the post-auth OK/error packet.
	// For direct auth: 2; for AuthSwitch path: 4.
	authSeq uint8

	// Prepared statements
	statements map[uint32]*preparedStatement
	stmtMu     sync.RWMutex
}

// NewServer creates a new MySQL server
func NewServer(engine *executor.Engine, config Config) (*Server, error) {
	if config.Addr == "" {
		config.Addr = ":3306"
	}
	if config.Version == "" {
		config.Version = "5.7.42"
	}
	if config.Charset == 0 {
		config.Charset = 33 // utf8
	}

	s := &Server{
		engine:      engine,
		clients:     make(map[net.Conn]*clientConn),
		serverID:    config.ServerID,
		version:     config.Version,
		charset:     config.Charset,
		capability:  DEFAULT_CAPABILITIES,
		requireAuth: config.RequireAuth,
	}

	// Generate random server ID if not specified
	if s.serverID == 0 {
		var b [4]byte
		rand.Read(b[:])
		s.serverID = uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	}

	// Generate RSA key pair for caching_sha2_password full auth
	if key, err := rsa.GenerateKey(rand.Reader, 2048); err == nil {
		s.rsaKey = key
		if pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey); err == nil {
			s.rsaPubPEM = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
		}
	}

	return s, nil
}

// Start starts the MySQL server
func (s *Server) Start(addr string) error {
	if addr == "" {
		addr = ":3306"
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	s.listener = listener
	fmt.Printf("MySQL server listening on %s\n", addr)

	go s.acceptLoop()

	return nil
}

// Stop stops the MySQL server
func (s *Server) Stop() error {
	s.mu.Lock()
	s.shutdown = true
	s.mu.Unlock()

	// Close all client connections
	s.mu.RLock()
	for conn := range s.clients {
		conn.Close()
	}
	s.mu.RUnlock()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	return nil
}

// acceptLoop accepts new connections
func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.RLock()
			shutdown := s.shutdown
			s.mu.RUnlock()

			if shutdown {
				return
			}
			continue
		}

		client := &clientConn{
			conn:       conn,
			statements: make(map[uint32]*preparedStatement),
		}

		s.mu.Lock()
		s.clients[conn] = client
		s.mu.Unlock()

		go s.handleConnection(client)
	}
}

// handleConnection handles a client connection
func (s *Server) handleConnection(client *clientConn) {
	remoteAddr := client.conn.RemoteAddr().String()
	fmt.Printf("\n[CONN] ===== New connection from %s =====\n", remoteAddr)

	defer func() {
		fmt.Printf("[CONN] Connection closed: %s\n", remoteAddr)
		client.conn.Close()
		s.mu.Lock()
		delete(s.clients, client.conn)
		s.mu.Unlock()
	}()

	// Generate salt for authentication
	// MySQL protocol: 20 bytes for mysql_native_password
	client.salt = make([]byte, 20)
	rand.Read(client.salt)
	client.authSeq = 2 // default for direct auth path (handshake=0, client=1, OK=2)
	fmt.Printf("[CONN] Generated salt: %x\n", client.salt)

	// Send handshake
	if err := s.sendHandshake(client); err != nil {
		fmt.Printf("[CONN] Error sending handshake: %v\n", err)
		return
	}

	// Read auth response
	pw := NewPacketReader(client.conn)
	authData, err := pw.ReadPacket()
	if err != nil {
		fmt.Printf("[CONN] Error reading auth packet: %v\n", err)
		return
	}
	fmt.Printf("[CONN] Auth packet received: len=%d\n", len(authData))

	// Parse auth response
	if err := s.handleAuth(client, authData); err != nil {
		fmt.Printf("[CONN] Authentication failed: %v\n", err)
		// Send error packet to client before disconnecting
		writer := NewPacketWriter(client.conn)
		writer.sequence = client.authSeq
		writer.WriteError(1045, "28000", err.Error())
		return
	}

	// Send OK packet
	fmt.Printf("[CONN] Auth OK, sending OK packet (seq=%d)\n", client.authSeq)
	writer := NewPacketWriter(client.conn)
	writer.sequence = client.authSeq
	if err := writer.WriteOK(0, 0, 0, 0, ""); err != nil {
		fmt.Printf("[CONN] Error sending OK: %v\n", err)
		return
	}
	fmt.Printf("[CONN] Auth OK sent, entering command loop\n")

	// Handle commands
	s.commandLoop(client, pw)
}

// sendHandshake sends the initial handshake packet
func (s *Server) sendHandshake(client *clientConn) error {
	writer := NewPacketWriter(client.conn)

	var data []byte

	// Protocol version
	data = append(data, ProtocolVersion)

	// Server version
	data = append(data, []byte(s.version)...)
	data = append(data, 0) // null terminator

	// Connection ID
	data = append(data, byte(s.serverID), byte(s.serverID>>8),
		byte(s.serverID>>16), byte(s.serverID>>24))

	// Auth plugin data part 1 (8 bytes of salt)
	data = append(data, client.salt[:8]...)

	// Filler (1 byte, always 0)
	data = append(data, 0)

	// Capability flags (lower 2 bytes)
	cap := uint16(s.capability)
	data = append(data, byte(cap), byte(cap>>8))

	// Character set
	data = append(data, s.charset)

	// Status flags
	data = append(data, 0x00, 0x00)

	// Capability flags (upper 2 bytes)
	capUpper := uint16(s.capability >> 16)
	data = append(data, byte(capUpper), byte(capUpper>>8))

	// Length of auth plugin data: report 21 so both MySQL and MariaDB clients
	// read max(13, 21-8)=13 bytes for part2, correctly stripping the trailing null.
	// MariaDB Connector reads auth_data_len-8=13 bytes (not max), also correct.
	data = append(data, 21)

	// Reserved (10 bytes)
	for i := 0; i < 10; i++ {
		data = append(data, 0)
	}

	// Auth plugin data part 2: 12 bytes of salt + 1 null terminator = 13 bytes
	// Both MySQL (max(13,21-8)=13) and MariaDB (21-8=13) read exactly 13 bytes,
	// strip the trailing null, and get the correct 12-byte second half of the salt.
	data = append(data, client.salt[8:]...)
	data = append(data, 0) // null terminator (NOT padding — it's the field terminator)

	// Auth plugin name (null-terminated) — advertise mysql_native_password (SHA1).
	// This avoids the RSA key exchange required by caching_sha2_password on non-SSL connections
	// and works with all clients: HeidiSQL, DBeaver (MariaDB and MySQL 8 drivers), etc.
	// Clients using caching_sha2_password by default will receive an AuthSwitch response.
	data = append(data, []byte(AUTH_PLUGIN_NAME)...)
	data = append(data, 0)

	fmt.Printf("Debug handshake: salt=%x, plugin=%s\n", client.salt, AUTH_PLUGIN_NAME)
	fmt.Printf("Debug handshake: packet len=%d, data=%x\n", len(data), data)

	return writer.WritePacket(data)
}

// handleAuth handles authentication
func (s *Server) handleAuth(client *clientConn, data []byte) error {
	fmt.Printf("[AUTH] --- handleAuth start, packet len=%d ---\n", len(data))
	fmt.Printf("[AUTH] Full packet hex: %x\n", data)

	if len(data) < 4 {
		return fmt.Errorf("auth packet too short: %d bytes", len(data))
	}

	// Capability flags
	client.capability = uint32(data[0]) | uint32(data[1])<<8 |
		uint32(data[2])<<16 | uint32(data[3])<<24
	fmt.Printf("[AUTH] Client capability flags: 0x%08x\n", client.capability)
	// Decode key flags
	flagNames := []struct{ bit uint32; name string }{
		{0x0001, "LONG_PASSWORD"}, {0x0004, "LONG_FLAG"}, {0x0008, "CONNECT_WITH_DB"},
		{0x0080, "LOCAL_FILES"}, {0x0200, "PROTOCOL_41"}, {0x0800, "SSL"},
		{0x2000, "TRANSACTIONS"}, {0x8000, "SECURE_CONN"}, {0x10000, "MULTI_STMT"},
		{0x20000, "MULTI_RESULTS"}, {0x80000, "PLUGIN_AUTH"}, {0x100000, "CONNECT_ATTRS"},
		{0x200000, "PLUGIN_AUTH_LENENC"}, {0x400000, "EXPIRE_PASSWORDS"},
		{0x800000, "SESSION_TRACK"}, {0x1000000, "DEPRECATE_EOF"},
	}
	var setFlags []string
	for _, f := range flagNames {
		if client.capability&f.bit != 0 {
			setFlags = append(setFlags, f.name)
		}
	}
	fmt.Printf("[AUTH] Set flags: %v\n", setFlags)

	offset := 4

	// Max packet size
	if len(data) < offset+4 {
		return fmt.Errorf("invalid auth packet: too short for max_packet_size")
	}
	maxPktSize := uint32(data[offset]) | uint32(data[offset+1])<<8 | uint32(data[offset+2])<<16 | uint32(data[offset+3])<<24
	fmt.Printf("[AUTH] Max packet size: %d\n", maxPktSize)
	offset += 4

	// Charset
	if len(data) < offset+1 {
		return fmt.Errorf("invalid auth packet: too short for charset")
	}
	client.charset = data[offset]
	fmt.Printf("[AUTH] Charset: %d\n", client.charset)
	offset++

	// Reserved (23 bytes)
	if len(data) < offset+23 {
		return fmt.Errorf("invalid auth packet: too short for reserved")
	}
	offset += 23

	// Username
	usernameEnd := offset
	for usernameEnd < len(data) && data[usernameEnd] != 0 {
		usernameEnd++
	}
	client.username = string(data[offset:usernameEnd])
	offset = usernameEnd + 1
	fmt.Printf("[AUTH] Username: %q\n", client.username)

	// Auth response
	var authResponse []byte
	if offset < len(data) {
		authLen := int(data[offset])
		offset++

		// Handle length-encoded auth data (0xfc, 0xfd, 0xfe prefixes)
		switch authLen {
		case 0xfc:
			if len(data) >= offset+2 {
				authLen = int(data[offset]) | int(data[offset+1])<<8
				offset += 2
			}
		case 0xfd:
			if len(data) >= offset+3 {
				authLen = int(data[offset]) | int(data[offset+1])<<8 | int(data[offset+2])<<16
				offset += 3
			}
		case 0xfe:
			if len(data) >= offset+8 {
				authLen = int(data[offset]) | int(data[offset+1])<<8 | int(data[offset+2])<<16 | int(data[offset+3])<<24
				offset += 8
			}
		}

		fmt.Printf("[AUTH] Auth response: len=%d\n", authLen)
		if authLen > 0 && len(data) >= offset+authLen {
			fmt.Printf("[AUTH] Auth response bytes: %x\n", data[offset:offset+authLen])
			authResponse = data[offset : offset+authLen]
			offset += authLen
		} else if authLen == 0 {
			fmt.Printf("[AUTH] Auth response: EMPTY (len=0)\n")
		} else {
			fmt.Printf("[AUTH] Auth response: TRUNCATED (need %d, have %d)\n", authLen, len(data)-offset)
		}
	}

	// Check for database name (CLIENT_CONNECT_WITH_DB)
	if client.capability&CLIENT_CONNECT_WITH_DB != 0 && offset < len(data) {
		dbEnd := offset
		for dbEnd < len(data) && data[dbEnd] != 0 {
			dbEnd++
		}
		client.database = string(data[offset:dbEnd])
		fmt.Printf("[AUTH] Database: %q\n", client.database)
		offset = dbEnd + 1
	}

	// Auth plugin name (CLIENT_PLUGIN_AUTH)
	var clientAuthPlugin string
	if client.capability&CLIENT_PLUGIN_AUTH != 0 && offset < len(data) {
		pluginEnd := offset
		for pluginEnd < len(data) && data[pluginEnd] != 0 {
			pluginEnd++
		}
		clientAuthPlugin = string(data[offset:pluginEnd])
		fmt.Printf("[AUTH] Client auth plugin: %q\n", clientAuthPlugin)
		offset = pluginEnd + 1
	}

	// Connection attributes (CLIENT_CONNECT_ATTRS) - just log size
	if client.capability&0x100000 != 0 && offset < len(data) {
		fmt.Printf("[AUTH] Connection attrs present, remaining bytes: %d\n", len(data)-offset)
	}

	fmt.Printf("[AUTH] Parsed: user=%q plugin=%q authLen=%d\n", client.username, clientAuthPlugin, len(authResponse))

	// Get auth from engine (unified authentication)
	auth := s.engine.GetAuth()

	// Check if authentication is required
	if s.requireAuth && !auth.IsEnabled() {
		return fmt.Errorf("access denied: authentication required but no credentials configured")
	}

	// If auth is not enabled, allow anonymous access with warning
	if !auth.IsEnabled() {
		fmt.Printf("[AUTH] Warning: No authentication configured. Anonymous access allowed.\n")
		return nil
	}

	// Check username first (applies to all auth paths)
	expectedUsername := auth.GetUsername()
	if expectedUsername != "" && client.username != expectedUsername {
		return fmt.Errorf("access denied for user '%s'", client.username)
	}

	// Route to the correct auth handler based on client's chosen plugin
	switch clientAuthPlugin {
	case AUTH_PLUGIN_CACHING_SHA2:
		// Client sent caching_sha2_password token, but we advertise mysql_native_password.
		// Send AuthSwitch to redirect client to mysql_native_password (SHA1, no RSA needed).
		fmt.Printf("[AUTH] client sent caching_sha2, switching to mysql_native_password\n")
		return s.sendAuthSwitchRequest(client)

	case AUTH_PLUGIN_NAME: // mysql_native_password
		// Client used mysql_native_password (e.g. MariaDB driver, HeidiSQL)
		fmt.Printf("[AUTH] mysql_native_password, response len=%d\n", len(authResponse))
		// Verify using MySQL SHA1 hash
		mysqlSha1 := auth.GetMySQLSha1()
		if mysqlSha1 == nil {
			// No password set, allow
			return nil
		}
		if verifyMysqlNativePasswordWithSha1(authResponse, client.salt, mysqlSha1) {
			fmt.Printf("[AUTH] mysql_native_password: OK\n")
			return nil
		}
		// Token mismatch — send AuthSwitch to let client retry with mysql_native_password
		fmt.Printf("[AUTH] mysql_native_password failed, sending AuthSwitch\n")
		return s.sendAuthSwitchRequest(client)

	default:
		// Unknown plugin — send AuthSwitchRequest asking for mysql_native_password
		fmt.Printf("[AUTH] unknown plugin %q, sending AuthSwitch\n", clientAuthPlugin)
		return s.sendAuthSwitchRequest(client)
	}
}

// commandLoop handles client commands
func (s *Server) commandLoop(client *clientConn, reader *PacketReader) {
	writer := NewPacketWriter(client.conn)

	for {
		// Reset sequence for reading new command (client sends at seq 0)
		// We respond starting at seq 1
		writer.sequence = 1

		data, err := reader.ReadPacket()
		if err != nil {
			return
		}

		if len(data) == 0 {
			continue
		}

		cmd := data[0]
		args := data[1:]

		switch cmd {
		case COM_QUIT:
			return

		case COM_PING:
			writer.WriteOK(0, 0, 0, 0, "")

		case COM_QUERY:
			fmt.Printf("Debug: received query: %q\n", string(args))
			s.handleQuery(client, writer, string(args))

		case COM_INIT_DB:
			client.database = string(args)
			writer.WriteOK(0, 0, 0, 0, "")

		case COM_FIELD_LIST:
			// Not implemented, send EOF
			writer.WriteEOF(0, 0)

		case COM_STATISTICS:
			s.handleStatistics(writer)

		case COM_STMT_PREPARE:
			s.handleStmtPrepare(client, writer, string(args))

		case COM_STMT_EXECUTE:
			s.handleStmtExecute(client, writer, args)

		case COM_STMT_CLOSE:
			s.handleStmtClose(client, args)

		default:
			writer.WriteError(1047, "42000", fmt.Sprintf("Unknown command: %d", cmd))
		}
	}
}

// handleQuery handles a query command
func (s *Server) handleQuery(client *clientConn, writer *PacketWriter, query string) {
	// Intercept information_schema queries that HeidiSQL/DBeaver send on connect
	if result := s.handleInfoSchemaQuery(query); result != nil {
		s.writeResult(client, writer, result)
		return
	}

	result, err := s.engine.Execute(query)
	if err != nil {
		writer.WriteError(1064, "42000", err.Error())
		return
	}

	if result.IsExecutionResult {
		writer.WriteOK(uint64(result.RowsAffected), uint64(result.LastInsertID), 0, 0, "")
		return
	}

	// Send result set
	if len(result.Columns) > 0 {
		// Column count
		writer.WriteResultHeader(uint64(len(result.Columns)))

		// Column definitions
		for _, col := range result.Columns {
			writer.WriteColumnDefinition(
				client.database, // schema
				"",              // table
				"",              // org table
				col,             // name
				col,             // org name
				255,             // length
				33,              // charset (utf8)
				MYSQL_TYPE_VAR_STRING, // type
				0,               // decimals
				0,               // flags
			)
		}

		// EOF packet after columns
		writer.WriteEOF(0, 0)

		// Row data
		for _, row := range result.Rows {
			values := make([]interface{}, len(row.Data))
			for i, v := range row.Data {
				values[i] = v.Data
			}
			writer.WriteRowData(values)
		}

		// EOF packet after rows
		writer.WriteEOF(0, 0)
	}
}

// infoSchemaResult is a lightweight result set for information_schema responses
type infoSchemaResult struct {
	columns []string
	rows    [][]string
}

// handleInfoSchemaQuery intercepts information_schema / system queries that GUI clients
// send automatically on connect and returns minimal compatible responses.
func (s *Server) handleInfoSchemaQuery(query string) *infoSchemaResult {
	q := strings.TrimSpace(query)
	upper := strings.ToUpper(q)

	// SHOW TABLES FROM `information_schema` or information_schema
	if strings.HasPrefix(upper, "SHOW TABLES FROM") &&
		strings.Contains(upper, "INFORMATION_SCHEMA") {
		return &infoSchemaResult{
			columns: []string{"Tables_in_information_schema"},
			rows: [][]string{
				{"SCHEMATA"}, {"TABLES"}, {"COLUMNS"}, {"STATISTICS"},
				{"KEY_COLUMN_USAGE"}, {"TABLE_CONSTRAINTS"}, {"EVENTS"},
				{"ROUTINES"}, {"TRIGGERS"}, {"VIEWS"},
			},
		}
	}

	// SELECT ... FROM information_schema.SCHEMATA
	if strings.Contains(upper, "INFORMATION_SCHEMA") && strings.Contains(upper, "SCHEMATA") {
		return &infoSchemaResult{
			columns: []string{"SCHEMA_NAME", "DEFAULT_CHARACTER_SET_NAME", "DEFAULT_COLLATION_NAME", "SQL_PATH"},
			rows:    [][]string{{"xxldb", "utf8mb4", "utf8mb4_general_ci", ""}},
		}
	}

	// SELECT ... FROM information_schema.EVENTS
	if strings.Contains(upper, "INFORMATION_SCHEMA") && strings.Contains(upper, "EVENTS") {
		return &infoSchemaResult{
			columns: []string{"EVENT_SCHEMA", "EVENT_NAME", "Db", "Name"},
			rows:    [][]string{},
		}
	}

	// SELECT ... FROM information_schema.COLUMNS
	if strings.Contains(upper, "INFORMATION_SCHEMA") && strings.Contains(upper, "COLUMNS") {
		cols := []string{"TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME", "ORDINAL_POSITION",
			"COLUMN_DEFAULT", "IS_NULLABLE", "DATA_TYPE", "CHARACTER_MAXIMUM_LENGTH", "CHARACTER_OCTET_LENGTH",
			"NUMERIC_PRECISION", "NUMERIC_SCALE", "DATETIME_PRECISION", "CHARACTER_SET_NAME", "COLLATION_NAME",
			"COLUMN_TYPE", "COLUMN_KEY", "EXTRA", "PRIVILEGES", "COLUMN_COMMENT", "GENERATION_EXPRESSION", "SRS_ID"}
		// Try to extract TABLE_NAME from WHERE TABLE_NAME='xxx'
		tableName := extractWHEREValue(upper, "TABLE_NAME")
		if tableName != "" {
			if result, err := s.engine.Execute("DESCRIBE " + tableName); err == nil {
				var rows [][]string
				for i, row := range result.Rows {
					colName := ""
					colType := "varchar"
					nullable := "YES"
					colKey := ""
					extra := ""
					if len(row.Data) > 0 {
						colName = fmt.Sprintf("%v", row.Data[0].Data)
					}
					if len(row.Data) > 1 {
						colType = strings.ToLower(fmt.Sprintf("%v", row.Data[1].Data))
					}
					if len(row.Data) > 2 {
						if fmt.Sprintf("%v", row.Data[2].Data) == "NO" {
							nullable = "NO"
						}
					}
					if len(row.Data) > 3 {
						colKey = fmt.Sprintf("%v", row.Data[3].Data)
					}
					if len(row.Data) > 5 {
						extra = fmt.Sprintf("%v", row.Data[5].Data)
					}
					charLen := ""
					if strings.Contains(colType, "char") || strings.Contains(colType, "text") {
						charLen = "65535"
					}
					rows = append(rows, []string{
						"def", "xxldb", tableName, colName, fmt.Sprintf("%d", i+1),
						"", nullable, colType, charLen, charLen,
						"", "", "", "utf8mb4", "utf8mb4_general_ci",
						colType, colKey, extra, "select,insert,update,references", "", "", "",
					})
				}
				return &infoSchemaResult{columns: cols, rows: rows}
			}
		}
		return &infoSchemaResult{columns: cols, rows: [][]string{}}
	}

	// SELECT ... FROM information_schema.REFERENTIAL_CONSTRAINTS
	if strings.Contains(upper, "REFERENTIAL_CONSTRAINTS") {
		return &infoSchemaResult{
			columns: []string{"CONSTRAINT_CATALOG", "CONSTRAINT_SCHEMA", "CONSTRAINT_NAME", "UNIQUE_CONSTRAINT_CATALOG", "UNIQUE_CONSTRAINT_SCHEMA", "UNIQUE_CONSTRAINT_NAME", "MATCH_OPTION", "UPDATE_RULE", "DELETE_RULE", "TABLE_NAME", "REFERENCED_TABLE_NAME"},
			rows:    [][]string{},
		}
	}

	// SELECT ... FROM information_schema.KEY_COLUMN_USAGE
	if strings.Contains(upper, "KEY_COLUMN_USAGE") {
		return &infoSchemaResult{
			columns: []string{"CONSTRAINT_CATALOG", "CONSTRAINT_SCHEMA", "CONSTRAINT_NAME", "TABLE_CATALOG", "TABLE_SCHEMA", "TABLE_NAME", "COLUMN_NAME", "ORDINAL_POSITION", "POSITION_IN_UNIQUE_CONSTRAINT", "REFERENCED_TABLE_SCHEMA", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"},
			rows:    [][]string{},
		}
	}

	// SELECT ... FROM information_schema (generic catch-all)
	if strings.Contains(upper, "INFORMATION_SCHEMA") {
		return &infoSchemaResult{columns: []string{"result"}, rows: [][]string{}}
	}

	return nil
}

// extractWHEREValue extracts the value of a condition like FIELD='value' from an uppercased query.
func extractWHEREValue(upper, field string) string {
	// Match FIELD='value' or FIELD = 'value'
	pat := field + `\s*=\s*'([^']+)'`
	re := regexp.MustCompile(pat)
	m := re.FindStringSubmatch(upper)
	if len(m) >= 2 {
		return strings.ToLower(m[1])
	}
	return ""
}

// writeResult writes an infoSchemaResult to the client
func (s *Server) writeResult(client *clientConn, writer *PacketWriter, res *infoSchemaResult) {
	writer.WriteResultHeader(uint64(len(res.columns)))
	for _, col := range res.columns {
		writer.WriteColumnDefinition("information_schema", "", "", col, col, 255, 33, MYSQL_TYPE_VAR_STRING, 0, 0)
	}
	writer.WriteEOF(0, 0)
	for _, row := range res.rows {
		vals := make([]interface{}, len(row))
		for i, v := range row {
			vals[i] = v
		}
		writer.WriteRowData(vals)
	}
	writer.WriteEOF(0, 0)
}

// handleStatistics handles statistics command
func (s *Server) handleStatistics(writer *PacketWriter) {
	stats := fmt.Sprintf("Uptime: %d\n", time.Now().Unix())
	writer.WritePacket([]byte(stats))
}

// handleStmtPrepare handles COM_STMT_PREPARE
func (s *Server) handleStmtPrepare(client *clientConn, writer *PacketWriter, query string) {
	// Count parameters (? placeholders)
	numParams := strings.Count(query, "?")

	// Generate statement ID
	stmtID := atomic.AddUint32(&s.stmtIDCounter, 1)

	// Create prepared statement
	stmt := &preparedStatement{
		id:        stmtID,
		query:     query,
		numParams: numParams,
		params:    make([]paramDesc, numParams),
	}

	// Store statement
	client.stmtMu.Lock()
	client.statements[stmtID] = stmt
	client.stmtMu.Unlock()

	// Send prepare response
	// Format: status (1) + statement_id (4) + num_columns (2) + num_params (2) + reserved (1) + warning_count (2)
	var resp []byte
	resp = append(resp, 0x00) // OK header
	resp = append(resp, byte(stmtID), byte(stmtID>>8), byte(stmtID>>16), byte(stmtID>>24))
	resp = append(resp, byte(0), byte(0)) // num_columns (0 for non-SELECT)
	resp = append(resp, byte(numParams), byte(numParams>>8))
	resp = append(resp, 0x00) // reserved
	resp = append(resp, 0x00, 0x00) // warning_count

	writer.WritePacket(resp)

	// Send parameter descriptions if any
	if numParams > 0 {
		for i := 0; i < numParams; i++ {
			// Send parameter packet
			// Type: MYSQL_TYPE_VAR_STRING (0xfd)
			writer.WriteColumnDefinition("", "", "", fmt.Sprintf("param%d", i), "", 255, 33, MYSQL_TYPE_VAR_STRING, 0, 0)
		}
		// EOF after parameters
		writer.WriteEOF(0, 0)
	}
}

// handleStmtExecute handles COM_STMT_EXECUTE
func (s *Server) handleStmtExecute(client *clientConn, writer *PacketWriter, data []byte) {
	if len(data) < 9 {
		writer.WriteError(1210, "HY000", "Invalid statement execute packet")
		return
	}

	// Parse statement ID
	stmtID := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24

	// Get statement
	client.stmtMu.RLock()
	stmt, ok := client.statements[stmtID]
	client.stmtMu.RUnlock()

	if !ok {
		writer.WriteError(1243, "42000", fmt.Sprintf("Unknown prepared statement: %d", stmtID))
		return
	}

	// Skip flags (1 byte) and iteration count (4 bytes)
	// Then parse null bitmap and parameters
	// For simplicity, we'll replace ? with actual values

	// Parse parameters from the packet
	// Format after header: null-bitmap, new-params-bound-flag, then type/value pairs
	offset := 9 // skip stmt_id(4) + flags(1) + iteration_count(4)

	// Null bitmap: (num_params+7)/8 bytes
	nullBitmapSize := (stmt.numParams + 7) / 8
	if len(data) < offset+nullBitmapSize {
		writer.WriteError(1210, "HY000", "Invalid parameter data")
		return
	}
	nullBitmap := data[offset : offset+nullBitmapSize]
	offset += nullBitmapSize

	// Check new params bound flag
	if len(data) > offset && data[offset] == 1 {
		offset++ // skip the flag
		// Read parameter types
		for i := 0; i < stmt.numParams; i++ {
			if len(data) < offset+2 {
				break
			}
			stmt.params[i].typ = data[offset]
			stmt.params[i].signed = data[offset+1]&0x80 != 0
			offset += 2
		}
	} else if stmt.numParams > 0 {
		// Use default type
		for i := range stmt.params {
			stmt.params[i].typ = MYSQL_TYPE_VAR_STRING
			stmt.params[i].signed = false
		}
	}

	// Read parameter values
	values := make([]interface{}, stmt.numParams)
	for i := 0; i < stmt.numParams; i++ {
		// Check if NULL
		if nullBitmap[i/8]&(1<<(uint(i)%8)) != 0 {
			values[i] = nil
			continue
		}

		// Read value based on type
		val, n := readParamValue(data[offset:], stmt.params[i].typ)
		values[i] = val
		offset += n
	}

	// Replace ? with values in query
	finalQuery := replacePlaceholders(stmt.query, values)

	fmt.Printf("Debug: execute prepared stmt %d: %s\n", stmtID, finalQuery)

	// Execute query
	result, err := s.engine.Execute(finalQuery)
	if err != nil {
		writer.WriteError(1064, "42000", err.Error())
		return
	}

	if result.IsExecutionResult {
		// For INSERT/UPDATE/DELETE, send OK packet
		writer.WriteOK(uint64(result.RowsAffected), uint64(result.LastInsertID), 0, 0, "")
		return
	}

	// Send result set in binary format
	if len(result.Columns) > 0 {
		// Column count
		writer.WriteResultHeader(uint64(len(result.Columns)))

		// Column definitions
		for _, col := range result.Columns {
			writer.WriteColumnDefinition(
				client.database,
				"",
				"",
				col,
				col,
				255,
				33,
				MYSQL_TYPE_VAR_STRING,
				0,
				0,
			)
		}

		// EOF after columns
		writer.WriteEOF(0, 0)

		// Binary row data
		for _, row := range result.Rows {
			values := make([]interface{}, len(row.Data))
			for i, v := range row.Data {
				values[i] = v.Data
			}
			writer.WriteBinaryRowData(values)
		}

		// EOF after rows
		writer.WriteEOF(0, 0)
	} else {
		// Empty result set
		writer.WriteResultHeader(0)
		writer.WriteEOF(0, 0)
	}
}

// handleStmtClose handles COM_STMT_CLOSE
func (s *Server) handleStmtClose(client *clientConn, data []byte) {
	if len(data) < 4 {
		return
	}

	stmtID := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24

	client.stmtMu.Lock()
	delete(client.statements, stmtID)
	client.stmtMu.Unlock()
}

// readParamValue reads a parameter value from the packet
func readParamValue(data []byte, typ byte) (interface{}, int) {
	if len(data) == 0 {
		return nil, 0
	}

	switch typ {
	case MYSQL_TYPE_TINY:
		if len(data) >= 1 {
			return int(data[0]), 1
		}
	case MYSQL_TYPE_SHORT, MYSQL_TYPE_YEAR:
		if len(data) >= 2 {
			return int(data[0]) | int(data[1])<<8, 2
		}
	case MYSQL_TYPE_LONG, MYSQL_TYPE_INT24:
		if len(data) >= 4 {
			return int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24, 4
		}
	case MYSQL_TYPE_LONGLONG:
		if len(data) >= 8 {
			val := uint64(data[0]) | uint64(data[1])<<8 | uint64(data[2])<<16 | uint64(data[3])<<24 |
				uint64(data[4])<<32 | uint64(data[5])<<40 | uint64(data[6])<<48 | uint64(data[7])<<56
			return val, 8
		}
	case MYSQL_TYPE_FLOAT:
		if len(data) >= 4 {
			return float64(float32(uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24)), 4
		}
	case MYSQL_TYPE_DOUBLE:
		if len(data) >= 8 {
			bits := uint64(data[0]) | uint64(data[1])<<8 | uint64(data[2])<<16 | uint64(data[3])<<24 |
				uint64(data[4])<<32 | uint64(data[5])<<40 | uint64(data[6])<<48 | uint64(data[7])<<56
			return float64(bits), 8
		}
	default:
		// Length-encoded string
		if len(data) >= 1 {
			length, n, _ := ReadLengthEncodedInt(data)
			if len(data) >= n+int(length) {
				return string(data[n : n+int(length)]), n + int(length)
			}
		}
	}
	return nil, 0
}

// replacePlaceholders replaces ? placeholders with actual values
func replacePlaceholders(query string, values []interface{}) string {
	result := query
	for _, v := range values {
		var replacement string
		if v == nil {
			replacement = "NULL"
		} else {
			switch val := v.(type) {
			case string:
				replacement = fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				replacement = fmt.Sprintf("%d", val)
			case float32, float64:
				replacement = fmt.Sprintf("%v", val)
			default:
				replacement = fmt.Sprintf("'%v'", val)
			}
		}
		result = strings.Replace(result, "?", replacement, 1)
	}
	return result
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
// sendAuthSwitchRequest sends an AuthSwitchRequest packet for MySQL 8.x clients
func (s *Server) sendAuthSwitchRequest(client *clientConn) error {
	writer := NewPacketWriter(client.conn)
	writer.sequence = 2 // After handshake(0) and auth response(1)

	// Build AuthSwitchRequest packet
	// Format: 0xfe + plugin_name + 0x00 + scramble (20 bytes) + 0x00
	var switchData []byte
	switchData = append(switchData, AUTH_SWITCH_REQUEST) // 0xfe
	switchData = append(switchData, []byte(AUTH_PLUGIN_NAME)...)
	switchData = append(switchData, 0)              // null terminator for plugin name
	switchData = append(switchData, client.salt...) // 20-byte scramble
	switchData = append(switchData, 0)              // trailing null required by mysql_native_password

	fmt.Printf("Debug: sending auth switch request, len=%d\n", len(switchData))

	if err := writer.WritePacket(switchData); err != nil {
		return fmt.Errorf("failed to send auth switch request: %w", err)
	}

	// Read the new auth response (client sends at seq=3)
	reader := NewPacketReader(client.conn)
	newAuthData, err := reader.ReadPacket()
	if err != nil {
		return fmt.Errorf("failed to read auth switch response: %w", err)
	}

	fmt.Printf("Debug: auth switch response len=%d\n", len(newAuthData))

	// After AuthSwitch: handshake=0, client=1, AuthSwitch=2, client=3, OK/err=4
	client.authSeq = 4

	// Verify authentication with new auth data
	auth := s.engine.GetAuth()
	if auth.IsEnabled() {
		expectedUsername := auth.GetUsername()
		if expectedUsername != "" && client.username != expectedUsername {
			return fmt.Errorf("access denied for user '%s'", client.username)
		}
		mysqlSha1 := auth.GetMySQLSha1()
		if mysqlSha1 != nil {
			fmt.Printf("[AUTH-SWITCH] Verifying password for user '%s'\n", client.username)
			fmt.Printf("[AUTH-SWITCH]   newAuthData len=%d, data=%x\n", len(newAuthData), newAuthData)
			fmt.Printf("[AUTH-SWITCH]   salt len=%d, data=%x\n", len(client.salt), client.salt)
			result := verifyMysqlNativePasswordWithSha1(newAuthData, client.salt, mysqlSha1)
			fmt.Printf("[AUTH-SWITCH]   verification result: %v\n", result)
			if !result {
				return fmt.Errorf("access denied for user '%s' (invalid password)", client.username)
			}
		}
	}

	return nil
}

// verifyMysqlNativePasswordWithSha1 verifies MySQL native password using SHA1 hash
// authResponse: client's response (SHA1(password) XOR SHA1(salt + SHA1(SHA1(password))))
// salt: 20-byte random salt sent by server
// sha1Password: SHA1(password)
func verifyMysqlNativePasswordWithSha1(authResponse []byte, salt []byte, sha1Password []byte) bool {
	if len(authResponse) != 20 || len(salt) != 20 || len(sha1Password) != 20 {
		return false
	}

	// Compute SHA1(SHA1(password))
	sha1Sha1Password := sha1.Sum(sha1Password)

	// Compute SHA1(salt + SHA1(SHA1(password)))
	hasher := sha1.New()
	hasher.Write(salt)
	hasher.Write(sha1Sha1Password[:])
	sha1SaltSha1 := hasher.Sum(nil)

	// XOR to get expected SHA1(password)
	expected := make([]byte, 20)
	for i := 0; i < 20; i++ {
		expected[i] = authResponse[i] ^ sha1SaltSha1[i]
	}

	// Compare with SHA1(password)
	for i := 0; i < 20; i++ {
		if expected[i] != sha1Password[i] {
			return false
		}
	}

	return true
}

// verifyMysqlNativePassword verifies the MySQL native password authentication
// authResponse: client's response (SHA1(password) XOR SHA1(salt + SHA1(SHA1(password))))
// salt: 20-byte random salt sent by server
// password: plaintext password
func verifyMysqlNativePassword(authResponse []byte, salt []byte, password string) bool {
	if len(authResponse) != 20 || len(salt) != 20 {
		return false
	}

	if password == "" {
		// Empty password - auth response should be empty
		return len(authResponse) == 0
	}

	// Compute SHA1(password)
	sha1Password := sha1.Sum([]byte(password))

	// Compute SHA1(SHA1(password))
	sha1Sha1Password := sha1.Sum(sha1Password[:])

	// Compute SHA1(salt + SHA1(SHA1(password)))
	hasher := sha1.New()
	hasher.Write(salt)
	hasher.Write(sha1Sha1Password[:])
	sha1SaltSha1 := hasher.Sum(nil)

	// XOR to get expected SHA1(password)
	expected := make([]byte, 20)
	for i := 0; i < 20; i++ {
		expected[i] = authResponse[i] ^ sha1SaltSha1[i]
	}

	// Compare with SHA1(password)
	for i := 0; i < 20; i++ {
		if expected[i] != sha1Password[i] {
			return false
		}
	}

	return true
}

// verifyCachingSha2Password verifies caching_sha2_password fast-auth token.
// authResponse: 32-byte token = SHA256(password) XOR SHA256(SHA256(SHA256(password)) + salt)
// The server can verify without storing plaintext by using the same formula.
func verifyCachingSha2Password(authResponse []byte, salt []byte, password string) bool {
	if len(authResponse) != 32 {
		return false
	}
	if password == "" {
		return len(authResponse) == 0
	}

	// h1 = SHA256(password)
	h1 := sha256.Sum256([]byte(password))
	// h2 = SHA256(SHA256(password))
	h2 := sha256.Sum256(h1[:])

	// h3 = SHA256(h2 + salt)
	h := sha256.New()
	h.Write(h2[:])
	h.Write(salt)
	h3 := h.Sum(nil)

	// recovered = authResponse XOR h3  →  should equal h1
	recovered := make([]byte, 32)
	for i := range recovered {
		recovered[i] = authResponse[i] ^ h3[i]
	}

	// Verify: SHA256(recovered) == h2
	check := sha256.Sum256(recovered)
	return bytes.Equal(check[:], h2[:])
}

// computeCachingSha2Token computes the expected 32-byte caching_sha2_password token.
// Used for debug logging only.
func computeCachingSha2Token(password, salt []byte) []byte {
	h1 := sha256.Sum256(password)
	h2 := sha256.Sum256(h1[:])
	h := sha256.New()
	h.Write(h2[:])
	h.Write(salt)
	h3 := h.Sum(nil)
	token := make([]byte, 32)
	for i := range token {
		token[i] = h1[i] ^ h3[i]
	}
	return token
}

// handleCachingSha2FullAuth handles the full-authentication path for caching_sha2_password.
// This is used when fast-auth (SHA256 token) fails or the client/server chose the RSA path.
// Flow (no SSL):
//   Server → AuthMoreData(0x04)  "full auth needed"
//   Client → 0x02                "send me your public key"
//   Server → AuthMoreData(PEM)   RSA public key
//   Client → RSA-encrypted(XOR(password+\0, cycled-salt))
//   Server → decrypts, verifies plaintext password
func (s *Server) handleCachingSha2FullAuth(client *clientConn) error {
	writer := NewPacketWriter(client.conn)
	writer.sequence = client.authSeq

	// Generate a fresh RSA key pair per connection so that clients cannot use a
	// cached public key from a previous session (DBeaver caches the pubkey).
	connKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("caching_sha2 full auth: generate RSA key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&connKey.PublicKey)
	if err != nil {
		return fmt.Errorf("caching_sha2 full auth: marshal pubkey: %w", err)
	}
	connPubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	// Tell client full auth is needed, then immediately send our RSA public key.
	// We do NOT wait for client to request it (0x02). Sending the pubkey proactively
	// ensures the client uses THIS connection's fresh key rather than any cached key.
	if err := writer.WriteAuthMoreData([]byte{0x04}); err != nil {
		return fmt.Errorf("caching_sha2 full auth: send 0x04: %w", err)
	}

	// Send pubkey immediately (proactive — no need to wait for 0x02 request)
	fmt.Printf("[AUTH-SHA2] sending RSA pubkey proactively (len=%d) seq=%d\n", len(connPubPEM), writer.sequence)
	pemWithNull := append(connPubPEM, 0x00)
	if err := writer.WriteAuthMoreData(pemWithNull); err != nil {
		return fmt.Errorf("caching_sha2 full auth: send RSA pubkey: %w", err)
	}

	reader := NewPacketReader(client.conn)
	pkt, err := reader.ReadPacket()
	if err != nil {
		return fmt.Errorf("caching_sha2 full auth: read client response: %w", err)
	}
	fmt.Printf("[AUTH-SHA2] received packet after pubkey: len=%d pkt[0]=%02x\n", len(pkt), pkt[0])

	// If client sends 0x02, it's explicitly requesting the pubkey (didn't get our proactive send).
	// In that case send again and read the actual encrypted blob.
	if len(pkt) == 1 && pkt[0] == 0x02 {
		fmt.Printf("[AUTH-SHA2] client still sent 0x02 (did not use proactive pubkey); sending again\n")
		pemWithNull2 := append(connPubPEM, 0x00)
		if err := writer.WriteAuthMoreData(pemWithNull2); err != nil {
			return fmt.Errorf("caching_sha2 full auth: send RSA pubkey (2nd): %w", err)
		}
		pkt, err = reader.ReadPacket()
		if err != nil {
			return fmt.Errorf("caching_sha2 full auth: read encrypted password: %w", err)
		}
		fmt.Printf("[AUTH-SHA2] encrypted password len=%d\n", len(pkt))
	}

	fmt.Printf("[AUTH-SHA2] encrypted password len=%d hex(32)=%x\n", len(pkt), pkt[:min(32, len(pkt))])

	// Decrypt RSA-OAEP encrypted payload.
	// Try SHA-1 first (older Connector/J), then SHA-256 (newer Connector/J 8.x)
	decrypted, decErr := rsa.DecryptOAEP(sha1.New(), rand.Reader, connKey, pkt, nil)
	if decErr != nil {
		fmt.Printf("[AUTH-SHA2] SHA-1 OAEP failed (%v), trying SHA-256\n", decErr)
		decrypted, decErr = rsa.DecryptOAEP(sha256.New(), rand.Reader, connKey, pkt, nil)
		if decErr != nil {
			return fmt.Errorf("caching_sha2 full auth: RSA decrypt failed (tried SHA-1 and SHA-256): %w", decErr)
		}
		fmt.Printf("[AUTH-SHA2] RSA decrypt OK with SHA-256\n")
	} else {
		fmt.Printf("[AUTH-SHA2] RSA decrypt OK with SHA-1\n")
	}

	// Client XOR'd password with cycled scramble before encrypting; undo that
	for i := range decrypted {
		decrypted[i] ^= client.salt[i%len(client.salt)]
	}

	// Strip null terminator
	if idx := bytes.IndexByte(decrypted, 0); idx >= 0 {
		decrypted = decrypted[:idx]
	}

	plainPwd := string(decrypted)
	// Verify using unified auth
	auth := s.engine.GetAuth()
	if auth.IsEnabled() {
		// For caching_sha2_password, we need to verify the plaintext password
		// We'll use the Authenticate method which works with the stored hash
		if !auth.Authenticate(auth.GetUsername(), plainPwd) {
			return fmt.Errorf("access denied for user '%s' (invalid password)", client.username)
		}
	}
	fmt.Printf("[AUTH-SHA2] full auth password verified\n")

	// Update authSeq so the caller sends OK with the right sequence number
	client.authSeq = writer.sequence + 1
	return nil
}
