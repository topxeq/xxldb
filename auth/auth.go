// Package auth provides authentication for xxldb
package auth

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// Auth handles database authentication
type Auth struct {
	mu           sync.RWMutex
	username     string
	password     string // Hashed password (SHA256)
	mysqlSha1    []byte // MySQL SHA1 hash: SHA1(password) for MySQL protocol
	enabled      bool
}

// NewAuth creates a new Auth instance
func NewAuth() *Auth {
	return &Auth{
		enabled: false,
	}
}

// SetCredentials sets the username and password
func (a *Auth) SetCredentials(username, password string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.username = username
	if password != "" {
		a.password = hashPassword(password)
		a.mysqlSha1 = hashPasswordMySQL(password)
	} else {
		a.password = "" // Clear password if empty
		a.mysqlSha1 = nil
	}
	a.enabled = username != ""
}

// Enable enables authentication
func (a *Auth) Enable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = true
}

// Disable disables authentication
func (a *Auth) Disable() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = false
}

// IsEnabled returns whether authentication is enabled
func (a *Auth) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// Authenticate verifies username and password
func (a *Auth) Authenticate(username, password string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if !a.enabled {
		return true // No authentication required
	}

	if username != a.username {
		return false
	}

	if a.password == "" {
		return true // No password set
	}

	return a.password == hashPassword(password)
}

// GetUsername returns the current username
func (a *Auth) GetUsername() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.username
}

// SetPassword sets a new password
func (a *Auth) SetPassword(newPassword string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.password = hashPassword(newPassword)
	a.mysqlSha1 = hashPasswordMySQL(newPassword)
}

// GetMySQLSha1 returns the MySQL SHA1 hash for MySQL protocol authentication
func (a *Auth) GetMySQLSha1() []byte {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.mysqlSha1
}

// hashPassword creates a SHA256 hash of the password
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte("xxldb_" + password + "_2026"))
	return hex.EncodeToString(hash[:])
}

// hashPasswordMySQL creates a SHA1 hash for MySQL native password authentication
func hashPasswordMySQL(password string) []byte {
	hash := sha1.Sum([]byte(password))
	return hash[:]
}

// ToMap exports auth config for persistence
func (a *Auth) ToMap() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := map[string]interface{}{
		"username": a.username,
		"password": a.password,
		"enabled":  a.enabled,
	}
	if a.mysqlSha1 != nil {
		result["mysql_sha1"] = hex.EncodeToString(a.mysqlSha1)
	}
	return result
}

// FromMap imports auth config from persistence
func (a *Auth) FromMap(m map[string]interface{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if username, ok := m["username"].(string); ok {
		a.username = username
	}
	if password, ok := m["password"].(string); ok {
		a.password = password
	}
	if enabled, ok := m["enabled"].(bool); ok {
		a.enabled = enabled
	}
	if mysqlSha1Hex, ok := m["mysql_sha1"].(string); ok {
		if data, err := hex.DecodeString(mysqlSha1Hex); err == nil {
			a.mysqlSha1 = data
		}
	}
}

// ClearCredentials clears all credentials
func (a *Auth) ClearCredentials() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.username = ""
	a.password = ""
	a.mysqlSha1 = nil
	a.enabled = false
}
