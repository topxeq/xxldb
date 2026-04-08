// Package main provides the xxldb command-line client
package main

import (
	"bufio"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	_ "github.com/topxeq/xxldb/driver"
	importpkg "github.com/topxeq/xxldb/import"
	"github.com/topxeq/xxldb/executor"
	"github.com/topxeq/xxldb/mysql"
	"github.com/peterh/liner"
)

var (
	version   = "2.0.0"
	buildDate = "2026-04-04"
)

func main() {
	// Parse command line flags
	dbPath := flag.String("db", "", "Database file path (in-memory if not specified)")
	inMemory := flag.Bool("memory", false, "Use in-memory database")
	username := flag.String("user", "", "Username for authentication")
	password := flag.String("password", "", "Password for authentication")
	encryptPassword := flag.String("encrypt", "", "Password for database encryption (or to open encrypted database)")
	logLevel := flag.String("log", "INFO", "Log level: DEBUG, INFO, WARN, ERROR")
	execStmt := flag.String("e", "", "Execute SQL statement and exit")
	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help")

	// MySQL server flags
	mysqlAddr := flag.String("mysql", "", "Start MySQL protocol server on address (e.g., :3306)")
	requireAuth := flag.Bool("require-auth", false, "Require authentication for MySQL server (reject anonymous connections if no password set)")

	// SSH remote database flags
	sshHost := flag.String("ssh-host", "", "SSH host for remote database")
	sshPort := flag.Int("ssh-port", 22, "SSH port (default: 22)")
	sshUser := flag.String("ssh-user", "", "SSH username")
	sshPassword := flag.String("ssh-password", "", "SSH password")
	sshKey := flag.String("ssh-key", "", "SSH private key path (e.g., ~/.ssh/id_rsa)")

	// SMB/CIFS remote database flags
	smbHost := flag.String("smb-host", "", "SMB/CIFS server host")
	smbPort := flag.Int("smb-port", 445, "SMB port (default: 445)")
	smbShare := flag.String("smb-share", "", "SMB share name")
	smbUser := flag.String("smb-user", "", "SMB username")
	smbPassword := flag.String("smb-password", "", "SMB password")
	smbDomain := flag.String("smb-domain", "", "SMB domain (optional)")

	// WebDAV remote database flags
	webdavURL := flag.String("webdav-url", "", "WebDAV server URL (e.g., https://cloud.example.com/dav)")
	webdavUser := flag.String("webdav-user", "", "WebDAV username")
	webdavPassword := flag.String("webdav-password", "", "WebDAV password")

	// Import flags
	importSource := flag.String("import", "", "Import from source database (format: type://connection)")
	importTable := flag.String("table", "", "Source table to import")
	importTo := flag.String("to", "", "Target table name (default: same as source)")
	importAll := flag.Bool("import-all", false, "Import all tables from source database")
	batchSize := flag.Int("batch", 1000, "Batch size for import")
	overwrite := flag.Bool("overwrite", false, "Overwrite existing tables")

	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	if *showVersion {
		fmt.Printf("xxldb version %s (built %s)\n", version, buildDate)
		return
	}

	// Determine database path and connection
	path := *dbPath
	var dsn string

	// Build DSN based on connection type
	if *sshHost != "" {
		// SSH remote database
		if path == "" {
			fmt.Fprintf(os.Stderr, "Error: database path (-db) is required for SSH connection\n")
			os.Exit(1)
		}
		if *sshUser == "" {
			fmt.Fprintf(os.Stderr, "Error: SSH username (-ssh-user) is required\n")
			os.Exit(1)
		}
		if *sshPassword == "" && *sshKey == "" {
			fmt.Fprintf(os.Stderr, "Error: SSH requires either -ssh-password or -ssh-key\n")
			os.Exit(1)
		}

		// Build SSH DSN
		dsn = buildSSHDSN(*sshHost, *sshPort, *sshUser, *sshPassword, *sshKey, path)
		fmt.Printf("Connecting to remote database via SSH: %s@%s:%d%s\n", *sshUser, *sshHost, *sshPort, path)
	} else if *smbHost != "" {
		// SMB/CIFS remote database
		if path == "" {
			fmt.Fprintf(os.Stderr, "Error: database path (-db) is required for SMB connection\n")
			os.Exit(1)
		}
		if *smbShare == "" {
			fmt.Fprintf(os.Stderr, "Error: SMB share name (-smb-share) is required\n")
			os.Exit(1)
		}
		if *smbUser == "" {
			fmt.Fprintf(os.Stderr, "Error: SMB username (-smb-user) is required\n")
			os.Exit(1)
		}
		if *smbPassword == "" {
			fmt.Fprintf(os.Stderr, "Error: SMB password (-smb-password) is required\n")
			os.Exit(1)
		}

		// Build SMB DSN
		dsn = buildSMBDSN(*smbHost, *smbPort, *smbShare, *smbUser, *smbPassword, *smbDomain, path)
		fmt.Printf("Connecting to remote database via SMB: %s@%s/%s%s\n", *smbUser, *smbHost, *smbShare, path)
	} else if *webdavURL != "" {
		// WebDAV remote database
		if path == "" {
			fmt.Fprintf(os.Stderr, "Error: database path (-db) is required for WebDAV connection\n")
			os.Exit(1)
		}
		if *webdavUser == "" {
			fmt.Fprintf(os.Stderr, "Error: WebDAV username (-webdav-user) is required\n")
			os.Exit(1)
		}
		if *webdavPassword == "" {
			fmt.Fprintf(os.Stderr, "Error: WebDAV password (-webdav-password) is required\n")
			os.Exit(1)
		}

		// Build WebDAV DSN
		dsn = buildWebDAVDSN(*webdavURL, *webdavUser, *webdavPassword, path)
		fmt.Printf("Connecting to remote database via WebDAV: %s@%s%s\n", *webdavUser, *webdavURL, path)
	} else if *inMemory || path == "" {
		dsn = ":memory:"
		*inMemory = true
	} else {
		dsn = path
		// Add encryption password to DSN if provided
		if *encryptPassword != "" {
			dsn = fmt.Sprintf("%s?encrypt=%s", path, *encryptPassword)
		}
	}

	// Handle import - use dedicated engine to avoid conflicts with driver connection
	if *importSource != "" {
		err := handleImportDirect(path, *importSource, *importTable, *importTo, *importAll, *batchSize, *overwrite, *encryptPassword, *username, *password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Import error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Open database (only if not importing)
	var db *sql.DB
	var err error

	if dsn == ":memory:" {
		db, err = sql.Open("xxldb", dsn)
		fmt.Println("Opened in-memory database")
	} else if *sshHost != "" || *smbHost != "" || *webdavURL != "" {
		// Remote connection - don't create local directory
		db, err = sql.Open("xxldb", dsn)
	} else {
		// Local file - ensure directory exists
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			os.MkdirAll(dir, 0755)
		}
		db, err = sql.Open("xxldb", dsn)
		fmt.Printf("Opened database: %s\n", path)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Verify connection (this triggers actual connection and validates encryption password)
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}

	// Set log level via SET statement
	if _, err := db.Exec(fmt.Sprintf("SET LOG_LEVEL %s", *logLevel)); err != nil {
		// Non-fatal
	}

	// Handle authentication
	if *username != "" && *password != "" {
		if _, err := db.Exec(fmt.Sprintf("SET USER = '%s'", *username)); err != nil {
			// Non-fatal
		}
		if _, err := db.Exec(fmt.Sprintf("SET PASSWORD = '%s'", *password)); err != nil {
			// Non-fatal
		}
	}

	// Start MySQL server if requested
	if *mysqlAddr != "" {
		// Create engine for MySQL server
		enginePath := path
		if *inMemory {
			enginePath = ""
		}
		engine, err := executor.NewEngineWithConfig(executor.Config{
			Path:           enginePath,
			InMemory:       *inMemory,
			EncryptPassword: *encryptPassword,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating engine for MySQL server: %v\n", err)
			os.Exit(1)
		}
		defer engine.Close()

		// Check auth status and show warning if needed
		auth := engine.GetAuth()
		if !auth.IsEnabled() {
			if *requireAuth {
				fmt.Fprintf(os.Stderr, "Error: --require-auth specified but no credentials configured. Use -user and -password or SET USER/PASSWORD.\n")
				os.Exit(1)
			}
			fmt.Println("Warning: No authentication configured. Anonymous access allowed.")
			fmt.Println("         Use -user and -password to set credentials, or SET USER/PASSWORD in REPL.")
		}

		// Create and start MySQL server (uses unified auth from engine)
		server, err := mysql.NewServer(engine, mysql.Config{
			Addr:        *mysqlAddr,
			RequireAuth: *requireAuth,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating MySQL server: %v\n", err)
			os.Exit(1)
		}

		if err := server.Start(*mysqlAddr); err != nil {
			fmt.Fprintf(os.Stderr, "Error starting MySQL server: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("MySQL server started on %s\n", *mysqlAddr)
		defer server.Stop()

		// If only MySQL server is needed, wait for interrupt
		fmt.Println("Press Ctrl+C to stop the server...")
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		fmt.Println("\nShutting down...")
		return
	}

	// Execute single statement if provided
	if *execStmt != "" {
		// Check if it's a dot command
		if strings.HasPrefix(*execStmt, ".") {
			handleSpecialCommand(db, *execStmt)
		} else {
			executeStatement(db, *execStmt)
		}
		return
	}

	// Start REPL
	startREPL(db, path, *inMemory, *encryptPassword)
}

func printHelp() {
	fmt.Println("xxldb - A lightweight SQL database")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  xxldb [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("SQL Commands:")
	fmt.Println("  CREATE TABLE name (col1 type, col2 type, ...)")
	fmt.Println("  DROP TABLE name")
	fmt.Println("  INSERT INTO table (col1, col2) VALUES (val1, val2)")
	fmt.Println("  SELECT * FROM table WHERE condition")
	fmt.Println("  UPDATE table SET col=val WHERE condition")
	fmt.Println("  DELETE FROM table WHERE condition")
	fmt.Println()
	fmt.Println("Special Commands:")
	fmt.Println("  .help       Show this help")
	fmt.Println("  .tables     List all tables")
	fmt.Println("  .schema     Show table schema")
	fmt.Println("  .backup     Backup database")
	fmt.Println("  .restore    Restore database")
	fmt.Println("  .user       Set username")
	fmt.Println("  .password   Set password")
	fmt.Println("  .log        Set log level")
	fmt.Println("  .version    Show version")
	fmt.Println("  .quit       Exit the program")
	fmt.Println()
	fmt.Println("Import Commands:")
	fmt.Println("  .import <dsn> <table> [target]  Import single table")
	fmt.Println("  .import-all <dsn>               Import all tables")
	fmt.Println("  .import-sql <file>              Import SQL backup file (MySQL dump)")
	fmt.Println()
	fmt.Println("Data Types:")
	fmt.Println("  SEQ         Auto-increment sequence (int64)")
	fmt.Println("  INT         Integer (int64)")
	fmt.Println("  FLOAT       Floating point (float64)")
	fmt.Println("  CHAR(n)     Fixed-length string")
	fmt.Println("  VARCHAR(n)  Variable-length string")
	fmt.Println("  TEXT        Large text")
	fmt.Println("  DATE        Date")
	fmt.Println("  TIME        Time")
	fmt.Println("  DATETIME    Date and time")
	fmt.Println("  BLOB        Binary large object")
	fmt.Println("  FILE        File reference")
	fmt.Println()
	fmt.Println("Special Functions:")
	fmt.Println("  LOAD_FILE(path)        Load file content as BLOB")
	fmt.Println("  FILE(path)             Create file reference")
	fmt.Println("  FOLDER(path)           Create folder reference")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  xxldb -db /path/to/db")
	fmt.Println("  xxldb -memory")
	fmt.Println("  xxldb -e \"SELECT * FROM users\"")
	fmt.Println()
	fmt.Println("SSH Remote Database:")
	fmt.Println("  # Connect with password")
	fmt.Println("  xxldb -ssh-host server.com -ssh-user admin -ssh-password secret -db /data/mydb")
	fmt.Println()
	fmt.Println("  # Connect with SSH key")
	fmt.Println("  xxldb -ssh-host server.com -ssh-user admin -ssh-key ~/.ssh/id_rsa -db /data/mydb")
	fmt.Println()
	fmt.Println("  # Using DSN format")
	fmt.Println("  xxldb -db \"ssh://admin:secret@server.com:22/data/mydb\"")
	fmt.Println()
	fmt.Println("SMB/CIFS Remote Database:")
	fmt.Println("  # Connect to SMB share")
	fmt.Println("  xxldb -smb-host fileserver.local -smb-share data -smb-user user -smb-password pass -db mydb")
	fmt.Println()
	fmt.Println("  # With domain")
	fmt.Println("  xxldb -smb-host fileserver.local -smb-share data -smb-domain MYDOMAIN -smb-user user -smb-password pass -db mydb")
	fmt.Println()
	fmt.Println("  # Using DSN format")
	fmt.Println("  xxldb -db \"smb://user:pass@fileserver.local/data/mydb\"")
	fmt.Println()
	fmt.Println("WebDAV Remote Database:")
	fmt.Println("  # Connect to WebDAV server")
	fmt.Println("  xxldb -webdav-url https://cloud.example.com/dav -webdav-user user -webdav-password pass -db mydb")
	fmt.Println()
	fmt.Println("  # Using DSN format")
	fmt.Println("  xxldb -db \"webdavs://user:pass@cloud.example.com/dav/mydb\"")
	fmt.Println()
	fmt.Println("MySQL Protocol Server:")
	fmt.Println("  xxldb -db /path/to/db -mysql :3306 -user admin -password secret")
	fmt.Println()
	fmt.Println("  # Require authentication (reject anonymous connections)")
	fmt.Println("  xxldb -db /path/to/db -mysql :3306 -user admin -password secret -require-auth")
	fmt.Println()
	fmt.Println("Import Examples:")
	fmt.Println("  # Import single table from MySQL")
	fmt.Println("  xxldb -db my.db -import \"mysql://user:pass@localhost/db\" -table users")
	fmt.Println()
	fmt.Println("  # Import all tables from PostgreSQL")
	fmt.Println("  xxldb -db my.db -import \"postgresql://user:pass@localhost/db\" -import-all")
	fmt.Println()
	fmt.Println("  # Import from SQLite")
	fmt.Println("  xxldb -db my.db -import \"sqlite:///path/to/source.db\" -import-all")
}

func startREPL(db *sql.DB, dbPath string, inMemory bool, encryptPassword string) {
	// Create liner instance (better Unicode/Chinese support)
	line := liner.NewLiner()
	defer line.Close()

	// Set up history file
	line.SetCtrlCAborts(true)

	// Handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nGoodbye!")
		os.Exit(0)
	}()

	// Check if authentication is enabled by querying the database
	var authEnabled bool
	if !inMemory && dbPath != "" {
		// Check auth status via SQL query
		rows, err := db.Query("SHOW AUTH")
		if err == nil {
			rows.Close()
			// If we can query auth, it's enabled
			authEnabled = true
		}
	}

	// If auth is enabled, require login
	if authEnabled {
		fmt.Println("xxldb - A lightweight SQL database")
		fmt.Println("Authentication required.")
		fmt.Println()
		username, err := line.Prompt("Username: ")
		if err != nil {
			return
		}
		username = strings.TrimSpace(username)
		password, err := line.PasswordPrompt("Password: ")
		if err != nil {
			return
		}
		password = strings.TrimSpace(password)

		// Verify credentials via SQL
		// For now, just proceed - actual auth check would need to be implemented
		fmt.Println()
		fmt.Printf("Welcome, %s!\n", username)
		fmt.Println("Type .help for help, .quit to exit")
	} else {
		fmt.Println("xxldb - A lightweight SQL database")
		fmt.Println("Type .help for help, .quit to exit")
	}
	fmt.Println()

	for {
		input, err := line.Prompt("xxldb> ")
		if err != nil {
			// io.EOF or liner.ErrInterrupt
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Add to history
		line.AppendHistory(input)

		// Handle special commands
		if strings.HasPrefix(input, ".") {
			handleSpecialCommand(db, input)
			continue
		}

		// Handle SQL statements
		executeStatement(db, input)
	}
}

func handleSpecialCommand(db *sql.DB, cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch strings.ToLower(parts[0]) {
	case ".help":
		printHelp()

	case ".quit", ".exit", ".q":
		fmt.Println("Goodbye!")
		db.Close() // Properly close database before exit
		os.Exit(0)

	case ".tables":
		rows, err := db.Query("SHOW TABLES")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer rows.Close()
		printRows(rows)

	case ".schema":
		if len(parts) < 2 {
			fmt.Println("Usage: .schema <table_name>")
			return
		}
		tableName := parts[1]
		rows, err := db.Query(fmt.Sprintf("SHOW COLUMNS FROM %s", tableName))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer rows.Close()
		printRows(rows)

	case ".backup":
		if len(parts) < 2 {
			fmt.Println("Usage: .backup <path>")
			return
		}
		backupPath := parts[1]
		if _, err := db.Exec(fmt.Sprintf("BACKUP TO '%s'", backupPath)); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Database backed up to: %s\n", backupPath)

	case ".restore":
		if len(parts) < 2 {
			fmt.Println("Usage: .restore <path>")
			return
		}
		restorePath := parts[1]
		if _, err := db.Exec(fmt.Sprintf("RESTORE FROM '%s'", restorePath)); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Database restored from: %s\n", restorePath)

	case ".user":
		if len(parts) < 2 {
			fmt.Println("Usage: .user <username>")
			return
		}
		if _, err := db.Exec(fmt.Sprintf("SET USER = '%s'", parts[1])); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("Username set. Use .password to set the password.")

	case ".password":
		if len(parts) < 2 {
			fmt.Println("Usage: .password <password>")
			return
		}
		if _, err := db.Exec(fmt.Sprintf("SET PASSWORD = '%s'", parts[1])); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Println("Credentials updated.")

	case ".log":
		if len(parts) < 2 {
			fmt.Println("Usage: .log <level>")
			fmt.Println("Levels: DEBUG, INFO, WARN, ERROR")
			return
		}
		if _, err := db.Exec(fmt.Sprintf("SET LOG_LEVEL %s", parts[1])); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		fmt.Printf("Log level set to: %s\n", parts[1])

	case ".version":
		fmt.Printf("xxldb version %s (built %s)\n", version, buildDate)

	case ".import":
		if len(parts) < 3 {
			fmt.Println("Usage: .import <dsn> <table> [target_table]")
			fmt.Println("  DSN format: <type>://<connection>")
			fmt.Println("  Types: mysql, postgresql, sqlite, oracle, mssql")
			fmt.Println("Examples:")
			fmt.Println("  .import mysql://user:pass@localhost/db users")
			fmt.Println("  .import sqlite:///path/to/source.db mytable newtable")
			return
		}
		dsn := parts[1]
		sourceTable := parts[2]
		targetTable := sourceTable
		if len(parts) > 3 {
			targetTable = parts[3]
		}
		handleREPLImport(db, dsn, sourceTable, targetTable, false)

	case ".import-all":
		if len(parts) < 2 {
			fmt.Println("Usage: .import-all <dsn>")
			fmt.Println("  DSN format: <type>://<connection>")
			fmt.Println("  Types: mysql, postgresql, sqlite, oracle, mssql")
			return
		}
		handleREPLImport(db, parts[1], "", "", true)

	case ".import-sql":
		if len(parts) < 2 {
			fmt.Println("Usage: .import-sql <sql_file>")
			fmt.Println("  Import SQL backup file (MySQL dump format)")
			fmt.Println("Example:")
			fmt.Println("  .import-sql /path/to/backup.sql")
			return
		}
		handleImportSQL(db, parts[1])

	case ".export":
		if len(parts) < 3 {
			fmt.Println("Usage: .export <table_name> <file_path>")
			return
		}
		fmt.Printf("Export not yet implemented\n")

	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
		fmt.Println("Type .help for help")
	}
}

func executeStatement(db *sql.DB, sql string) {
	// Handle multiple statements
	statements := splitStatements(sql)

	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}

		// Determine if it's a query or exec
		upperStmt := strings.ToUpper(stmt)
		isQuery := strings.HasPrefix(upperStmt, "SELECT") ||
			strings.HasPrefix(upperStmt, "SHOW") ||
			strings.HasPrefix(upperStmt, "DESCRIBE") ||
			strings.HasPrefix(upperStmt, "EXPLAIN")

		if isQuery {
			rows, err := db.Query(stmt)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			printRows(rows)
			rows.Close()
		} else {
			result, err := db.Exec(stmt)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			rowsAffected, _ := result.RowsAffected()
			lastInsertID, _ := result.LastInsertId()

			fmt.Printf("Rows affected: %d\n", rowsAffected)
			if lastInsertID > 0 {
				fmt.Printf("Last insert ID: %d\n", lastInsertID)
			}
		}
	}
}

func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := byte(0)

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		// Handle string literals
		if c == '\'' || c == '"' {
			if !inString {
				inString = true
				stringChar = c
				current.WriteByte(c)
				continue
			}
			// We're in a string and found a quote
			if c == stringChar {
				// Check for escaped quote (doubled quote)
				if i+1 < len(sql) && sql[i+1] == c {
					// Write both quotes (preserving escape sequence)
					current.WriteByte(c)
					current.WriteByte(c)
					i++
					continue
				}
				// End of string
				inString = false
				current.WriteByte(c)
				continue
			}
			// Quote of different type inside string
			current.WriteByte(c)
			continue
		}

		if inString {
			current.WriteByte(c)
			continue
		}

		// Handle statement terminator
		if c == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteByte(c)
	}

	// Add remaining statement
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

func printRows(rows *sql.Rows) {
	columns, err := rows.Columns()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Calculate column widths
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}

	// Read all rows
	var allRows [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		allRows = append(allRows, values)

		// Update widths
		for i, val := range values {
			var l int
			if val == nil {
				l = 4 // "NULL"
			} else {
				l = len(fmt.Sprintf("%v", val))
			}
			if l > widths[i] {
				widths[i] = l
			}
		}
	}

	// Print header
	fmt.Print("+")
	for _, w := range widths {
		fmt.Print(strings.Repeat("-", w+2))
		fmt.Print("+")
	}
	fmt.Println()

	// Print column names
	fmt.Print("|")
	for i, col := range columns {
		fmt.Print(" ")
		fmt.Print(col)
		fmt.Print(strings.Repeat(" ", widths[i]-len(col)))
		fmt.Print(" |")
	}
	fmt.Println()

	// Print separator
	fmt.Print("+")
	for _, w := range widths {
		fmt.Print(strings.Repeat("-", w+2))
		fmt.Print("+")
	}
	fmt.Println()

	// Print rows
	for _, row := range allRows {
		fmt.Print("|")
		for i, val := range row {
			fmt.Print(" ")
			var s string
			if val == nil {
				s = "NULL"
			} else {
				s = fmt.Sprintf("%v", val)
			}
			fmt.Print(s)
			fmt.Print(strings.Repeat(" ", widths[i]-len(s)))
			fmt.Print(" |")
		}
		fmt.Println()
	}

	// Print footer
	fmt.Print("+")
	for _, w := range widths {
		fmt.Print(strings.Repeat("-", w+2))
		fmt.Print("+")
	}
	fmt.Println()

	fmt.Printf("%d rows\n", len(allRows))
}

// handleImportDirect handles database import operations with dedicated engine
// This avoids conflicts with driver connection that could overwrite saved data
func handleImportDirect(dbPath, source, table, targetTable string, importAll bool, batchSize int, overwrite bool, encryptPassword string, username, password string) error {
	// Parse source DSN
	dbType, connStr, sourceDB, err := importpkg.ParseDSN(source)
	if err != nil {
		return fmt.Errorf("failed to parse source DSN: %w", err)
	}

	fmt.Printf("Importing from %s (database: %s)...\n", dbType, sourceDB)

	// Ensure directory exists
	if dbPath != "" {
		dir := filepath.Dir(dbPath)
		if dir != "" && dir != "." {
			os.MkdirAll(dir, 0755)
		}
	}

	// Create engine with the target database path for persistence
	var engine *executor.Engine
	if dbPath != "" {
		engine, err = executor.NewEngineWithConfig(executor.Config{
			Path:            dbPath,
			InMemory:        false,
			EncryptPassword: encryptPassword,
		})
		if err != nil {
			return fmt.Errorf("failed to create engine: %w", err)
		}
	} else {
		// Use in-memory if no path specified
		engine, err = executor.NewEngine("", true)
		if err != nil {
			return fmt.Errorf("failed to create engine: %w", err)
		}
	}
	defer engine.Close()

	// Set authentication if provided
	if username != "" && password != "" {
		if _, err := engine.Execute(fmt.Sprintf("SET USER = '%s'", username)); err != nil {
			// Non-fatal, log warning
		}
		if _, err := engine.Execute(fmt.Sprintf("SET PASSWORD = '%s'", password)); err != nil {
			// Non-fatal, log warning
		}
	}

	// Create import manager
	manager := importpkg.NewImportManager(engine)

	// Register appropriate importer
	var importer importpkg.Importer
	switch dbType {
	case importpkg.DatabaseTypeMySQL:
		importer = importpkg.NewMySQLImporter()
	case importpkg.DatabaseTypePostgreSQL:
		importer = importpkg.NewPostgreSQLImporter()
	case importpkg.DatabaseTypeSQLite:
		importer = importpkg.NewSQLiteImporter()
	case importpkg.DatabaseTypeOracle:
		importer = importpkg.NewOracleImporter()
	case importpkg.DatabaseTypeMSSQL:
		importer = importpkg.NewMSSQLImporter()
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	manager.RegisterImporter(dbType, importer)

	if importAll {
		// Import all tables
		config := &importpkg.ImportConfig{
			SourceType:  dbType,
			SourceDSN:   connStr,
			SourceDB:    sourceDB,
			BatchSize:   batchSize,
			Overwrite:   overwrite,
			CreateTable: true,
		}

		result, err := manager.ImportAll(config)
		if err != nil {
			return err
		}

		fmt.Printf("Import completed!\n")
		fmt.Printf("  Tables imported: %d\n", result.TablesImported)
		fmt.Printf("  Rows imported: %d\n", result.RowsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, e := range result.Errors {
				fmt.Printf("    - %v\n", e)
			}
		}
	} else {
		// Import single table
		if table == "" {
			return fmt.Errorf("table name is required for single table import")
		}

		config := &importpkg.ImportConfig{
			SourceType:  dbType,
			SourceDSN:   connStr,
			SourceDB:    sourceDB,
			SourceTable: table,
			TargetTable: targetTable,
			BatchSize:   batchSize,
			Overwrite:   overwrite,
			CreateTable: true,
		}

		result, err := manager.ImportTable(config)
		if err != nil {
			return err
		}

		fmt.Printf("Import completed!\n")
		fmt.Printf("  Rows imported: %d\n", result.RowsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, e := range result.Errors {
				fmt.Printf("    - %v\n", e)
			}
		}
	}

	fmt.Println("Import complete.")
	return nil
}

// handleImport handles database import operations
func handleImport(db *sql.DB, dbPath, source, table, targetTable string, importAll bool, batchSize int, overwrite bool, encryptPassword string) error {
	// Parse source DSN
	dbType, connStr, sourceDB, err := importpkg.ParseDSN(source)
	if err != nil {
		return fmt.Errorf("failed to parse source DSN: %w", err)
	}

	fmt.Printf("Importing from %s (database: %s)...\n", dbType, sourceDB)

	// Create engine with the target database path for persistence
	var engine *executor.Engine
	if dbPath != "" {
		engine, err = executor.NewEngineWithConfig(executor.Config{
			Path:            dbPath,
			InMemory:        false,
			EncryptPassword: encryptPassword,
		})
		if err != nil {
			return fmt.Errorf("failed to create engine: %w", err)
		}
	} else {
		// Use in-memory if no path specified
		engine, err = executor.NewEngine("", true)
		if err != nil {
			return fmt.Errorf("failed to create engine: %w", err)
		}
	}
	defer engine.Close()

	// Create import manager with a wrapper
	manager := importpkg.NewImportManager(engine)

	// Register appropriate importer
	var importer importpkg.Importer
	switch dbType {
	case importpkg.DatabaseTypeMySQL:
		importer = importpkg.NewMySQLImporter()
	case importpkg.DatabaseTypePostgreSQL:
		importer = importpkg.NewPostgreSQLImporter()
	case importpkg.DatabaseTypeSQLite:
		importer = importpkg.NewSQLiteImporter()
	case importpkg.DatabaseTypeOracle:
		importer = importpkg.NewOracleImporter()
	case importpkg.DatabaseTypeMSSQL:
		importer = importpkg.NewMSSQLImporter()
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	manager.RegisterImporter(dbType, importer)

	if importAll {
		// Import all tables
		config := &importpkg.ImportConfig{
			SourceType:  dbType,
			SourceDSN:   connStr,
			SourceDB:    sourceDB,
			BatchSize:   batchSize,
			Overwrite:   overwrite,
			CreateTable: true,
		}

		result, err := manager.ImportAll(config)
		if err != nil {
			return err
		}

		fmt.Printf("Import completed!\n")
		fmt.Printf("  Tables imported: %d\n", result.TablesImported)
		fmt.Printf("  Rows imported: %d\n", result.RowsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, e := range result.Errors {
				fmt.Printf("    - %v\n", e)
			}
		}
	} else {
		// Import single table
		if table == "" {
			return fmt.Errorf("table name is required for single table import")
		}

		config := &importpkg.ImportConfig{
			SourceType:  dbType,
			SourceDSN:   connStr,
			SourceDB:    sourceDB,
			SourceTable: table,
			TargetTable: targetTable,
			BatchSize:   batchSize,
			Overwrite:   overwrite,
			CreateTable: true,
		}

		result, err := manager.ImportTable(config)
		if err != nil {
			return err
		}

		fmt.Printf("Import completed!\n")
		fmt.Printf("  Rows imported: %d\n", result.RowsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, e := range result.Errors {
				fmt.Printf("    - %v\n", e)
			}
		}
	}

		fmt.Println("Import complete.")
	return nil
}

// handleREPLImport handles import commands from the REPL
func handleREPLImport(db *sql.DB, dsn, sourceTable, targetTable string, importAll bool) {
	// Parse DSN
	dbType, connStr, sourceDB, err := importpkg.ParseDSN(dsn)
	if err != nil {
		fmt.Printf("Error parsing DSN: %v\n", err)
		return
	}

	fmt.Printf("Connecting to %s database (database: %s)...\n", dbType, sourceDB)

	// Create engine for import operations
	engine, err := executor.NewEngine("", true)
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		return
	}
	defer engine.Close()

	// Create import manager
	manager := importpkg.NewImportManager(engine)

	// Register appropriate importer
	var importer importpkg.Importer
	switch dbType {
	case importpkg.DatabaseTypeMySQL:
		importer = importpkg.NewMySQLImporter()
	case importpkg.DatabaseTypePostgreSQL:
		importer = importpkg.NewPostgreSQLImporter()
	case importpkg.DatabaseTypeSQLite:
		importer = importpkg.NewSQLiteImporter()
	case importpkg.DatabaseTypeOracle:
		importer = importpkg.NewOracleImporter()
	case importpkg.DatabaseTypeMSSQL:
		importer = importpkg.NewMSSQLImporter()
	default:
		fmt.Printf("Unsupported database type: %s\n", dbType)
		return
	}

	manager.RegisterImporter(dbType, importer)

	if importAll {
		// Import all tables
		fmt.Println("Importing all tables...")
		config := &importpkg.ImportConfig{
			SourceType:  dbType,
			SourceDSN:   connStr,
			SourceDB:    sourceDB,
			BatchSize:   1000,
			Overwrite:   false,
			CreateTable: true,
		}

		result, err := manager.ImportAll(config)
		if err != nil {
			fmt.Printf("Import error: %v\n", err)
			return
		}

		fmt.Printf("Import completed!\n")
		fmt.Printf("  Tables imported: %d\n", result.TablesImported)
		fmt.Printf("  Total rows imported: %d\n", result.RowsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, e := range result.Errors {
				fmt.Printf("    - %v\n", e)
			}
		}
	} else {
		// Import single table
		if sourceTable == "" {
			fmt.Println("Error: source table name is required")
			return
		}

		if targetTable == "" {
			targetTable = sourceTable
		}

		fmt.Printf("Importing table %s -> %s...\n", sourceTable, targetTable)

		config := &importpkg.ImportConfig{
			SourceType:  dbType,
			SourceDSN:   connStr,
			SourceDB:    sourceDB,
			SourceTable: sourceTable,
			TargetTable: targetTable,
			BatchSize:   1000,
			Overwrite:   false,
			CreateTable: true,
		}

		result, err := manager.ImportTable(config)
		if err != nil {
			fmt.Printf("Import error: %v\n", err)
			return
		}

		fmt.Printf("Import completed!\n")
		fmt.Printf("  Rows imported: %d\n", result.RowsImported)
		if len(result.Errors) > 0 {
			fmt.Printf("  Errors: %d\n", len(result.Errors))
			for _, e := range result.Errors {
				fmt.Printf("    - %v\n", e)
			}
		}
	}

	// Copy imported data from in-memory engine to the opened database
	fmt.Println("Synchronizing with current database...")
	syncData(engine, db)
}

// syncData synchronizes data from the engine to the database connection
func syncData(engine *executor.Engine, db *sql.DB) {
	// Get list of tables from engine
	result, err := engine.Execute("SHOW TABLES")
	if err != nil {
		fmt.Printf("Warning: could not list tables for sync: %v\n", err)
		return
	}

	var tables []string
	for _, row := range result.Rows {
		if len(row.Data) > 0 {
			tables = append(tables, row.Data[0].ToString())
		}
	}

	for _, table := range tables {
		// For each table, get the data and insert into db
		selectResult, err := engine.Execute(fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			continue
		}

		if len(selectResult.Rows) == 0 {
			continue
		}

		// Get schema info
		schemaResult, _ := engine.Execute(fmt.Sprintf("SHOW COLUMNS FROM %s", table))
		
		// Check if table exists in target db
		var tableExists bool
		checkSQL := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1", table)
		_, err = db.Exec(checkSQL)
		tableExists = err == nil

		if !tableExists && schemaResult != nil {
			// Create table in target db
			var colDefs []string
			for _, row := range schemaResult.Rows {
				if len(row.Data) >= 5 {
					colName := row.Data[0].ToString()
					colType := row.Data[1].ToString()
					nullable := row.Data[3].ToString()
					primaryKey := row.Data[4].ToString()
					
					def := fmt.Sprintf("%s %s", colName, colType)
					if nullable == "false" {
						def += " NOT NULL"
					}
					if primaryKey == "true" {
						def += " PRIMARY KEY"
					}
					colDefs = append(colDefs, def)
				}
			}
			
			if len(colDefs) > 0 {
				createSQL := fmt.Sprintf("CREATE TABLE %s (%s)", table, strings.Join(colDefs, ", "))
				_, err = db.Exec(createSQL)
				if err != nil {
					fmt.Printf("Warning: could not create table %s: %v\n", table, err)
					continue
				}
			}
		}

		// Insert rows
		for _, row := range selectResult.Rows {
			if len(row.Data) == 0 {
				continue
			}

			// Build VALUES clause with actual values
			var valueStrs []string
			for _, val := range row.Data {
				if val.Data == nil {
					valueStrs = append(valueStrs, "NULL")
				} else {
					// Escape and quote string values
					switch v := val.Data.(type) {
					case string:
						escaped := strings.ReplaceAll(v, "'", "''")
						valueStrs = append(valueStrs, fmt.Sprintf("'%s'", escaped))
					case int, int32, int64, float32, float64:
						valueStrs = append(valueStrs, fmt.Sprintf("%v", v))
					case []byte:
						escaped := strings.ReplaceAll(string(v), "'", "''")
						valueStrs = append(valueStrs, fmt.Sprintf("'%s'", escaped))
					default:
						escaped := strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "''")
						valueStrs = append(valueStrs, fmt.Sprintf("'%s'", escaped))
					}
				}
				}

				insertSQL := fmt.Sprintf("INSERT INTO %s VALUES (%s)", table, strings.Join(valueStrs, ", "))
				_, err = db.Exec(insertSQL)
				if err != nil {
					fmt.Printf("Warning: insert error for table %s: %v\n", table, err)
				}
			}
		}

		fmt.Println("Synchronization complete.")
	}

// buildSSHDSN builds an SSH DSN string
func buildSSHDSN(host string, port int, user, password, key, dbPath string) string {
	dsn := fmt.Sprintf("ssh://%s@%s:%d/%s", user, host, port, dbPath)

	// Add authentication
	if password != "" {
		// Encode password in URL
		dsn = fmt.Sprintf("ssh://%s:%s@%s:%d/%s", user, password, host, port, dbPath)
	}

	// Add key as query parameter
	if key != "" {
		dsn += fmt.Sprintf("?private_key=%s", key)
	}

	return dsn
}

// buildSMBDSN builds an SMB DSN string
func buildSMBDSN(host string, port int, share, user, password, domain, dbPath string) string {
	// Build basic SMB DSN
	dsn := fmt.Sprintf("smb://%s:%s@%s:%d/%s/%s", user, password, host, port, share, dbPath)

	// Add domain as query parameter if specified
	if domain != "" {
		dsn += fmt.Sprintf("?domain=%s", domain)
	}

	return dsn
}

// buildWebDAVDSN builds a WebDAV DSN string
func buildWebDAVDSN(url, user, password, dbPath string) string {
	// Determine scheme
	scheme := "webdav"
	if strings.HasPrefix(url, "https://") {
		scheme = "webdavs"
		// Remove https:// prefix for DSN
		url = strings.TrimPrefix(url, "https://")
	} else {
		// Remove http:// prefix for DSN
		url = strings.TrimPrefix(url, "http://")
	}

	return fmt.Sprintf("%s://%s:%s@%s/%s", scheme, user, password, url, dbPath)
}

// handleImportSQL imports a SQL backup file (MySQL dump format)
func handleImportSQL(db *sql.DB, filePath string) {
	// Open the SQL file
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		return
	}
	defer file.Close()

	fmt.Printf("Importing SQL file: %s\n", filePath)

	// Read and parse the SQL file
	scanner := bufio.NewScanner(file)
	// Increase buffer size for large lines
	buf := make([]byte, 0, 64*1024*1024) // 64MB buffer
	scanner.Buffer(buf, 64*1024*1024)

	var currentStmt strings.Builder
	var inCreateTable bool
	var createTableName string
	var tablesCreated int
	var rowsInserted int64
	totalLines := 0

	for scanner.Scan() {
		line := scanner.Text()
		totalLines++

		// Skip comments and MySQL-specific directives
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*/") {
			continue
		}

		// Skip MySQL-specific SET statements
		if strings.HasPrefix(trimmed, "SET ") || strings.HasPrefix(trimmed, "/*!") {
			continue
		}

		// Skip LOCK TABLES and UNLOCK TABLES
		if strings.HasPrefix(trimmed, "LOCK TABLES") || strings.HasPrefix(trimmed, "UNLOCK TABLES") {
			continue
		}

		// Skip ALTER TABLE ... DISABLE/ENABLE KEYS
		if strings.HasPrefix(trimmed, "ALTER TABLE") && strings.Contains(trimmed, "KEYS") {
			continue
		}

		// Handle CREATE TABLE
		if strings.HasPrefix(trimmed, "CREATE TABLE") || strings.HasPrefix(trimmed, "DROP TABLE") {
			// Execute any pending statement
			if currentStmt.Len() > 0 {
				stmtStr := currentStmt.String()
				if err := executeImportStatement(db, stmtStr); err != nil {
					// Log but continue
					fmt.Printf("Warning: %v\n", err)
				}
				currentStmt.Reset()
			}

			// Handle DROP TABLE
			if strings.HasPrefix(trimmed, "DROP TABLE") {
				// Extract table name
				re := regexp.MustCompile("DROP TABLE IF EXISTS `?([^`\\s;]+)`?")
				matches := re.FindStringSubmatch(trimmed)
				if len(matches) > 1 {
					dropStmt := fmt.Sprintf("DROP TABLE IF EXISTS %s", matches[1])
					if _, err := db.Exec(dropStmt); err != nil {
						fmt.Printf("Warning dropping table: %v\n", err)
					}
				}
				continue
			}

			// Start capturing CREATE TABLE
			inCreateTable = true
			// Extract table name
			re := regexp.MustCompile("CREATE TABLE `?([^`\\s(]+)`?")
			matches := re.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				createTableName = matches[1]
			}
			currentStmt.WriteString(convertCreateTable(trimmed))
			// Check if statement is complete (ends with ;)
			if strings.HasSuffix(trimmed, ";") {
				inCreateTable = false
				stmtStr := currentStmt.String()
				if err := executeImportStatement(db, stmtStr); err != nil {
					fmt.Printf("Error creating table %s: %v\n", createTableName, err)
				} else {
					tablesCreated++
					fmt.Printf("Created table: %s\n", createTableName)
				}
				currentStmt.Reset()
			} else {
				currentStmt.WriteString("\n")
			}
			continue
		}

		// Continue CREATE TABLE
		if inCreateTable {
			currentStmt.WriteString(convertCreateTableLine(trimmed))
			if strings.HasSuffix(trimmed, ";") {
				inCreateTable = false
				stmtStr := currentStmt.String()
				if err := executeImportStatement(db, stmtStr); err != nil {
					fmt.Printf("Error creating table %s: %v\n", createTableName, err)
				} else {
					tablesCreated++
					fmt.Printf("Created table: %s\n", createTableName)
				}
				currentStmt.Reset()
			} else {
				currentStmt.WriteString("\n")
			}
			continue
		}

		// Handle INSERT statements
		if strings.HasPrefix(trimmed, "INSERT INTO") {
			// Convert and execute INSERT
			converted := convertInsertStatement(trimmed)
			result, err := db.Exec(converted)
			if err != nil {
				// Try to continue on error
				if rowsInserted > 0 {
					// Only log every 1000th error to avoid flooding
					if rowsInserted%1000 == 0 {
						fmt.Printf("Warning at row %d: %v\n", rowsInserted, err)
					}
				}
			} else {
				if affected, e := result.RowsAffected(); e == nil {
					rowsInserted += affected
					if rowsInserted%10000 == 0 {
						fmt.Printf("Inserted %d rows...\n", rowsInserted)
					}
				}
			}
			continue
		}

		// Accumulate other statements
		currentStmt.WriteString(line)
		if strings.HasSuffix(trimmed, ";") {
			stmtStr := currentStmt.String()
			if err := executeImportStatement(db, stmtStr); err != nil {
				// Log but continue
			}
			currentStmt.Reset()
		} else {
			currentStmt.WriteString(" ")
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}

	fmt.Printf("\nImport complete!\n")
	fmt.Printf("  Tables created: %d\n", tablesCreated)
	fmt.Printf("  Rows inserted: %d\n", rowsInserted)
	fmt.Printf("  Lines processed: %d\n", totalLines)
}

// executeImportStatement executes a single SQL statement
func executeImportStatement(db *sql.DB, stmt string) error {
	stmt = strings.TrimSpace(stmt)
	if stmt == "" || stmt == ";" {
		return nil
	}
	_, err := db.Exec(stmt)
	return err
}

// convertCreateTable converts MySQL CREATE TABLE to XxLdb format
func convertCreateTable(line string) string {
	// Replace backticks with nothing or double quotes
	result := strings.ReplaceAll(line, "`", "")
	
	// Replace AUTO_INCREMENT with AUTO_INCREMENT (XxLdb supports this)
	// But we also need to remove MySQL-specific clauses
	
	// Remove ENGINE=...
	result = regexp.MustCompile("(?i)ENGINE=\\w+").ReplaceAllString(result, "")
	
	// Remove DEFAULT CHARSET=...
	result = regexp.MustCompile("(?i)DEFAULT CHARSET=\\w+").ReplaceAllString(result, "")
	
	// Remove COLLATE=...
	result = regexp.MustCompile("(?i)COLLATE=\\w+").ReplaceAllString(result, "")
	
	// Remove CHARACTER SET
	result = regexp.MustCompile("(?i)CHARACTER SET \\w+").ReplaceAllString(result, "")
	
	// Remove COLLATE
	result = regexp.MustCompile("(?i)COLLATE \\w+").ReplaceAllString(result, "")
	
	// Remove AUTO_INCREMENT=...
	result = regexp.MustCompile("(?i)AUTO_INCREMENT=\\d+").ReplaceAllString(result, "")
	
	// Remove COMMENT='...' for table
	result = regexp.MustCompile("(?i)COMMENT='[^']*'").ReplaceAllString(result, "")
	
	// Remove ROW_FORMAT=...
	result = regexp.MustCompile("(?i)ROW_FORMAT=\\w+").ReplaceAllString(result, "")
	
	// Remove AVG_ROW_LENGTH=...
	result = regexp.MustCompile("(?i)AVG_ROW_LENGTH=\\d+").ReplaceAllString(result, "")
	
	// Remove MAX_ROWS=...
	result = regexp.MustCompile("(?i)MAX_ROWS=\\d+").ReplaceAllString(result, "")
	
	// Remove MIN_ROWS=...
	result = regexp.MustCompile("(?i)MIN_ROWS=\\d+").ReplaceAllString(result, "")
	
	// Remove DATA DIRECTORY=...
	result = regexp.MustCompile("(?i)DATA DIRECTORY='[^']*'").ReplaceAllString(result, "")
	
	// Remove INDEX DIRECTORY=...
	result = regexp.MustCompile("(?i)INDEX DIRECTORY='[^']*'").ReplaceAllString(result, "")
	
	// Remove PACK_KEYS=...
	result = regexp.MustCompile("(?i)PACK_KEYS=\\d").ReplaceAllString(result, "")
	
	// Remove CHECKSUM=...
	result = regexp.MustCompile("(?i)CHECKSUM=\\d").ReplaceAllString(result, "")
	
	// Remove DELAY_KEY_WRITE=...
	result = regexp.MustCompile("(?i)DELAY_KEY_WRITE=\\d").ReplaceAllString(result, "")
	
	// Remove STATS_PERSISTENT=...
	result = regexp.MustCompile("(?i)STATS_PERSISTENT=\\w+").ReplaceAllString(result, "")
	
	// Remove STATS_AUTO_RECALC=...
	result = regexp.MustCompile("(?i)STATS_AUTO_RECALC=\\w+").ReplaceAllString(result, "")
	
	// Remove STATS_SAMPLE_PAGES=...
	result = regexp.MustCompile("(?i)STATS_SAMPLE_PAGES=\\d+").ReplaceAllString(result, "")
	
	// Remove ROW_FORMAT
	result = regexp.MustCompile("(?i)ROW_FORMAT \\w+").ReplaceAllString(result, "")
	
	// Remove KEY_BLOCK_SIZE=...
	result = regexp.MustCompile("(?i)KEY_BLOCK_SIZE=\\d+").ReplaceAllString(result, "")

	// Convert UNIQUE KEY name (columns) to UNIQUE (columns) for XxLdb compatibility
	result = regexp.MustCompile("(?i)UNIQUE KEY \\w+ \\(").ReplaceAllString(result, "UNIQUE (")

	// Remove KEY name (columns) - regular non-unique indexes (XxLdb doesn't support in CREATE TABLE)
	result = regexp.MustCompile("(?i),?\\s*KEY \\w+ \\([^)]+\\)").ReplaceAllString(result, "")

	// Clean up multiple spaces
	result = regexp.MustCompile(" +").ReplaceAllString(result, " ")
	result = strings.ReplaceAll(result, " ,", ",")
	result = strings.ReplaceAll(result, "( ", "(")
	result = strings.ReplaceAll(result, " )", ")")
	
	return result
}

// convertCreateTableLine converts a line within CREATE TABLE
func convertCreateTableLine(line string) string {
	result := strings.ReplaceAll(line, "`", "")

	// Remove COLLATE for column definitions
	result = regexp.MustCompile("(?i) COLLATE \\w+").ReplaceAllString(result, "")

	// Remove CHARACTER SET for column definitions
	result = regexp.MustCompile("(?i) CHARACTER SET \\w+").ReplaceAllString(result, "")

	// Convert UNIQUE KEY name (columns) to UNIQUE (columns) for XxLdb compatibility
	result = regexp.MustCompile("(?i)UNIQUE KEY \\w+ \\(").ReplaceAllString(result, "UNIQUE (")

	// Remove KEY name (columns) - regular non-unique indexes (XxLdb doesn't support in CREATE TABLE)
	result = regexp.MustCompile("(?i)KEY \\w+ \\([^)]+\\)").ReplaceAllString(result, "")

	// Remove table-level MySQL options for closing line
	result = regexp.MustCompile("(?i)ENGINE=\\w+").ReplaceAllString(result, "")
	result = regexp.MustCompile("(?i)DEFAULT CHARSET=\\w+").ReplaceAllString(result, "")
	result = regexp.MustCompile("(?i)COLLATE=\\w+").ReplaceAllString(result, "")
	result = regexp.MustCompile("(?i)AUTO_INCREMENT=\\d+").ReplaceAllString(result, "")
	result = regexp.MustCompile("(?i)ROW_FORMAT=\\w+").ReplaceAllString(result, "")
	result = regexp.MustCompile("(?i)COMMENT='[^']*'").ReplaceAllString(result, "")

	// Convert tinyint to INT
	result = regexp.MustCompile("(?i)tinyint(\\(\\d+\\))?").ReplaceAllString(result, "INT")

	// Convert smallint to INT
	result = regexp.MustCompile("(?i)smallint(\\(\\d+\\))?").ReplaceAllString(result, "INT")

	// Convert mediumint to INT
	result = regexp.MustCompile("(?i)mediumint(\\(\\d+\\))?").ReplaceAllString(result, "INT")

	// Convert bigint to INT (or keep as BIGINT if XxLdb supports it)
	result = regexp.MustCompile("(?i)bigint(\\(\\d+\\))?").ReplaceAllString(result, "INT")

	// Convert double to FLOAT
	result = regexp.MustCompile("(?i)double(\\(\\d+,\\d+\\))?").ReplaceAllString(result, "FLOAT")

	// Convert float to FLOAT (keep)
	result = regexp.MustCompile("(?i)float(\\(\\d+,\\d+\\))?").ReplaceAllString(result, "FLOAT")

	// Convert decimal to FLOAT
	result = regexp.MustCompile("(?i)decimal(\\(\\d+,\\d+\\))?").ReplaceAllString(result, "FLOAT")

	// Convert numeric to FLOAT
	result = regexp.MustCompile("(?i)numeric(\\(\\d+,\\d+\\))?").ReplaceAllString(result, "FLOAT")

	// Remove unsigned
	result = regexp.MustCompile("(?i) unsigned").ReplaceAllString(result, "")

	// Remove zerofill
	result = regexp.MustCompile("(?i) zerofill").ReplaceAllString(result, "")

	// Remove DEFAULT CURRENT_TIMESTAMP for datetime
	result = regexp.MustCompile("(?i) DEFAULT CURRENT_TIMESTAMP(\\(\\))?").ReplaceAllString(result, "")

	// Remove ON UPDATE CURRENT_TIMESTAMP
	result = regexp.MustCompile("(?i) ON UPDATE CURRENT_TIMESTAMP(\\(\\))?").ReplaceAllString(result, "")

	// Remove COMMENT '...' for columns
	result = regexp.MustCompile("(?i) COMMENT '[^']*'").ReplaceAllString(result, "")

	// Clean up multiple spaces
	result = regexp.MustCompile(" +").ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}

// convertInsertStatement converts MySQL INSERT to XxLdb format
func convertInsertStatement(line string) string {
	// Replace backticks with nothing
	result := strings.ReplaceAll(line, "`", "")
	
	// Handle escaped quotes
	// MySQL uses \' and \" for escaped quotes, XxLdb uses ''
	result = strings.ReplaceAll(result, "\\'", "''")
	result = strings.ReplaceAll(result, "\\\"", "\"\"")
	
	// Handle escaped backslashes - just remove the escape
	result = strings.ReplaceAll(result, "\\\\", "\\")
	
	// Handle \n, \r, \t
	result = strings.ReplaceAll(result, "\\n", "\n")
	result = strings.ReplaceAll(result, "\\r", "\r")
	result = strings.ReplaceAll(result, "\\t", "\t")
	
	return result
}
