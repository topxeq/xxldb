// Package driver provides a Go SQL driver for xxldb
package driver

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// DSN represents a parsed data source name
type DSN struct {
	// Database path
	Path string

	// In-memory mode
	InMemory bool

	// SSH configuration (if remote)
	SSH *SSHConfig

	// SMB configuration (if remote)
	SMB *SMBConfig

	// WebDAV configuration (if remote)
	WebDAV *WebDAVConfig
}

// SSHConfig holds SSH connection parameters
type SSHConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	PrivateKey string
	Passphrase string
	Timeout    time.Duration
}

// SMBConfig holds SMB connection parameters
type SMBConfig struct {
	Host     string
	Port     int
	Share    string
	Username string
	Password string
	Domain   string
	Timeout  time.Duration
}

// WebDAVConfig holds WebDAV connection parameters
type WebDAVConfig struct {
	URL      string
	Username string
	Password string
	Timeout  time.Duration
}

// ParseDSN parses a data source name
// Supported formats:
//   - :memory: - in-memory database
//   - /path/to/db - local file database
//   - file:/path/to/db - local file database
//   - ssh://user:pass@host:port/path/to/db - SSH with password
//   - ssh://user@host:port/path/to/db?private_key=/path/to/key - SSH with key
//   - smb://user:pass@host/share/path/to/db - SMB/CIFS share
//   - webdav://user:pass@host/path/to/db - WebDAV
//   - webdavs://user:pass@host/path/to/db - WebDAV over HTTPS
func ParseDSN(dsn string) (*DSN, error) {
	// Handle in-memory
	if dsn == ":memory:" {
		return &DSN{InMemory: true}, nil
	}

	// Handle SSH URL
	if strings.HasPrefix(dsn, "ssh://") {
		return parseSSHDSN(dsn)
	}

	// Handle SMB URL
	if strings.HasPrefix(dsn, "smb://") {
		return parseSMBDSN(dsn)
	}

	// Handle WebDAV URL
	if strings.HasPrefix(dsn, "webdav://") || strings.HasPrefix(dsn, "webdavs://") {
		return parseWebDAVDSN(dsn)
	}

	// Handle file: prefix
	path := dsn
	if strings.HasPrefix(path, "file:") {
		path = strings.TrimPrefix(path, "file:")
	}

	return &DSN{Path: path}, nil
}

// parseSSHDSN parses SSH DSN format
func parseSSHDSN(dsn string) (*DSN, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid SSH DSN: %w", err)
	}

	if u.Scheme != "ssh" {
		return nil, fmt.Errorf("expected ssh scheme, got %s", u.Scheme)
	}

	ssh := &SSHConfig{
		Host: u.Hostname(),
	}

	// Parse port
	if port := u.Port(); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		ssh.Port = p
	} else {
		ssh.Port = 22 // Default SSH port
	}

	// Parse username
	if u.User != nil {
		ssh.Username = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			ssh.Password = pass
		}
	}

	if ssh.Username == "" {
		return nil, fmt.Errorf("SSH username is required")
	}

	// Parse query parameters
	q := u.Query()

	if keyPath := q.Get("private_key"); keyPath != "" {
		ssh.PrivateKey = expandPath(keyPath)
	}

	if keyData := q.Get("private_key_data"); keyData != "" {
		// Key data passed directly (for programmatic use)
		ssh.PrivateKey = keyData
	}

	if passphrase := q.Get("passphrase"); passphrase != "" {
		ssh.Passphrase = passphrase
	}

	if timeout := q.Get("timeout"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		ssh.Timeout = t
	} else {
		ssh.Timeout = 30 * time.Second
	}

	// Database path is the URL path (without leading /)
	dbPath := u.Path
	if strings.HasPrefix(dbPath, "/") {
		dbPath = dbPath[1:]
	}

	// If path is empty, use the host as path indicator
	if dbPath == "" {
		return nil, fmt.Errorf("database path is required in SSH DSN")
	}

	return &DSN{
		Path: dbPath,
		SSH:  ssh,
	}, nil
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// parseSMBDSN parses SMB DSN format
// Format: smb://user:pass@host:port/share/path/to/db
func parseSMBDSN(dsn string) (*DSN, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid SMB DSN: %w", err)
	}

	if u.Scheme != "smb" {
		return nil, fmt.Errorf("expected smb scheme, got %s", u.Scheme)
	}

	smb := &SMBConfig{
		Host: u.Hostname(),
	}

	// Parse port
	if port := u.Port(); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		smb.Port = p
	} else {
		smb.Port = 445 // Default SMB port
	}

	// Parse username and password
	if u.User != nil {
		smb.Username = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			smb.Password = pass
		}
	}

	if smb.Username == "" {
		return nil, fmt.Errorf("SMB username is required")
	}

	// Parse path - first segment is share name
	path := u.Path
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// Split path into share and database path
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return nil, fmt.Errorf("SMB share name is required")
	}

	smb.Share = parts[0]

	var dbPath string
	if len(parts) > 1 {
		dbPath = parts[1]
	}

	if dbPath == "" {
		return nil, fmt.Errorf("database path is required in SMB DSN")
	}

	// Parse query parameters
	q := u.Query()

	if domain := q.Get("domain"); domain != "" {
		smb.Domain = domain
	}

	if timeout := q.Get("timeout"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		smb.Timeout = t
	} else {
		smb.Timeout = 30 * time.Second
	}

	return &DSN{
		Path: dbPath,
		SMB:  smb,
	}, nil
}

// parseWebDAVDSN parses WebDAV DSN format
// Format: webdav://user:pass@host/path/to/db or webdavs://user:pass@host/path/to/db
func parseWebDAVDSN(dsn string) (*DSN, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid WebDAV DSN: %w", err)
	}

	if u.Scheme != "webdav" && u.Scheme != "webdavs" {
		return nil, fmt.Errorf("expected webdav or webdavs scheme, got %s", u.Scheme)
	}

	webdav := &WebDAVConfig{}

	// Build URL
	scheme := "http"
	if u.Scheme == "webdavs" {
		scheme = "https"
	}

	host := u.Host
	webdav.URL = fmt.Sprintf("%s://%s", scheme, host)

	// Parse username and password
	if u.User != nil {
		webdav.Username = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			webdav.Password = pass
		}
	}

	if webdav.Username == "" {
		return nil, fmt.Errorf("WebDAV username is required")
	}

	// Database path
	dbPath := u.Path
	if strings.HasPrefix(dbPath, "/") {
		dbPath = dbPath[1:]
	}

	if dbPath == "" {
		return nil, fmt.Errorf("database path is required in WebDAV DSN")
	}

	// Parse query parameters
	q := u.Query()

	if timeout := q.Get("timeout"); timeout != "" {
		t, err := time.ParseDuration(timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout: %w", err)
		}
		webdav.Timeout = t
	} else {
		webdav.Timeout = 30 * time.Second
	}

	return &DSN{
		Path:    dbPath,
		WebDAV:  webdav,
	}, nil
}

// String returns the DSN as a string
func (d *DSN) String() string {
	if d.InMemory {
		return ":memory:"
	}

	if d.SSH != nil {
		u := url.URL{
			Scheme: "ssh",
			Host:   fmt.Sprintf("%s:%d", d.SSH.Host, d.SSH.Port),
			Path:   "/" + d.Path,
		}

		if d.SSH.Password != "" {
			u.User = url.UserPassword(d.SSH.Username, d.SSH.Password)
		} else {
			u.User = url.User(d.SSH.Username)
		}

		if d.SSH.PrivateKey != "" {
			q := u.Query()
			q.Set("private_key", d.SSH.PrivateKey)
			u.RawQuery = q.Encode()
		}

		return u.String()
	}

	if d.SMB != nil {
		u := url.URL{
			Scheme: "smb",
			Host:   fmt.Sprintf("%s:%d", d.SMB.Host, d.SMB.Port),
			Path:   "/" + d.SMB.Share + "/" + d.Path,
		}

		if d.SMB.Password != "" {
			u.User = url.UserPassword(d.SMB.Username, d.SMB.Password)
		} else {
			u.User = url.User(d.SMB.Username)
		}

		return u.String()
	}

	if d.WebDAV != nil {
		scheme := "webdav"
		if strings.HasPrefix(d.WebDAV.URL, "https://") {
			scheme = "webdavs"
		}

		host := strings.TrimPrefix(d.WebDAV.URL, "http://")
		host = strings.TrimPrefix(host, "https://")

		u := url.URL{
			Scheme: scheme,
			Host:   host,
			Path:   "/" + d.Path,
		}

		if d.WebDAV.Password != "" {
			u.User = url.UserPassword(d.WebDAV.Username, d.WebDAV.Password)
		} else {
			u.User = url.User(d.WebDAV.Username)
		}

		return u.String()
	}

	return d.Path
}

// IsRemote returns true if the DSN represents a remote connection
func (d *DSN) IsRemote() bool {
	return d.SSH != nil || d.SMB != nil || d.WebDAV != nil
}

// Validate validates the DSN configuration
func (d *DSN) Validate() error {
	if d.InMemory {
		return nil
	}

	if d.Path == "" {
		return fmt.Errorf("database path is required")
	}

	if d.SSH != nil {
		if d.SSH.Host == "" {
			return fmt.Errorf("SSH host is required")
		}
		if d.SSH.Username == "" {
			return fmt.Errorf("SSH username is required")
		}
		if d.SSH.Password == "" && d.SSH.PrivateKey == "" {
			return fmt.Errorf("SSH requires either password or private key")
		}
	}

	if d.SMB != nil {
		if d.SMB.Host == "" {
			return fmt.Errorf("SMB host is required")
		}
		if d.SMB.Username == "" {
			return fmt.Errorf("SMB username is required")
		}
		if d.SMB.Share == "" {
			return fmt.Errorf("SMB share name is required")
		}
	}

	if d.WebDAV != nil {
		if d.WebDAV.URL == "" {
			return fmt.Errorf("WebDAV URL is required")
		}
		if d.WebDAV.Username == "" {
			return fmt.Errorf("WebDAV username is required")
		}
	}

	return nil
}

// BuildSSHDSN builds an SSH DSN from components
func BuildSSHDSN(host string, port int, username, password, privateKey, dbPath string) string {
	u := url.URL{
		Scheme: "ssh",
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   "/" + dbPath,
	}

	if password != "" {
		u.User = url.UserPassword(username, password)
	} else {
		u.User = url.User(username)
	}

	if privateKey != "" {
		q := u.Query()
		q.Set("private_key", privateKey)
		u.RawQuery = q.Encode()
	}

	return u.String()
}
