package auth

import (
	"sync"
	"testing"
)

func TestNewAuth(t *testing.T) {
	a := NewAuth()
	if a == nil {
		t.Fatal("NewAuth returned nil")
	}
	if a.IsEnabled() {
		t.Error("New auth should be disabled by default")
	}
}

func TestAuthDisabled(t *testing.T) {
	a := NewAuth()

	// When auth is disabled, any credentials should pass
	if !a.Authenticate("", "") {
		t.Error("Empty credentials should pass when auth disabled")
	}
	if !a.Authenticate("anyone", "anything") {
		t.Error("Any credentials should pass when auth disabled")
	}
}

func TestSetCredentials(t *testing.T) {
	a := NewAuth()

	// Set credentials
	a.SetCredentials("admin", "secret123")

	// Should enable auth automatically
	if !a.IsEnabled() {
		t.Error("SetCredentials should enable auth")
	}

	// Username should match
	if a.GetUsername() != "admin" {
		t.Error("Username mismatch")
	}
}

func TestAuthenticateCorrect(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Correct credentials
	if !a.Authenticate("admin", "secret123") {
		t.Error("Correct credentials should pass")
	}
}

func TestAuthenticateWrongUsername(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Wrong username
	if a.Authenticate("wronguser", "secret123") {
		t.Error("Wrong username should fail")
	}
}

func TestAuthenticateWrongPassword(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Wrong password
	if a.Authenticate("admin", "wrongpassword") {
		t.Error("Wrong password should fail")
	}
}

func TestAuthenticateEmptyPassword(t *testing.T) {
	a := NewAuth()

	// Set username only
	a.mu.Lock()
	a.username = "admin"
	a.enabled = true
	a.mu.Unlock()

	// Should pass without password check
	if !a.Authenticate("admin", "") {
		t.Error("Empty password should pass when no password set")
	}
	if !a.Authenticate("admin", "anything") {
		t.Error("Any password should pass when no password set")
	}
}

func TestPasswordHashing(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Get the stored password hash
	a.mu.RLock()
	storedHash := a.password
	a.mu.RUnlock()

	// Verify it's not plaintext
	if storedHash == "secret123" {
		t.Error("Password should be hashed, not stored in plaintext")
	}

	// Verify hash length (SHA256 = 64 hex chars)
	if len(storedHash) != 64 {
		t.Errorf("SHA256 hash should be 64 chars, got %d", len(storedHash))
	}

	// Verify same password produces same hash
	hash1 := hashPassword("testpass")
	hash2 := hashPassword("testpass")
	if hash1 != hash2 {
		t.Error("Same password should produce same hash")
	}

	// Verify different passwords produce different hashes
	hash3 := hashPassword("otherpass")
	if hash1 == hash3 {
		t.Error("Different passwords should produce different hashes")
	}
}

func TestEnableDisable(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Should be enabled after setting credentials
	if !a.IsEnabled() {
		t.Error("Should be enabled after SetCredentials")
	}

	// Disable
	a.Disable()
	if a.IsEnabled() {
		t.Error("Should be disabled after Disable")
	}

	// Should pass any credentials when disabled
	if !a.Authenticate("anyone", "anything") {
		t.Error("Should pass any credentials when disabled")
	}

	// Re-enable
	a.Enable()
	if !a.IsEnabled() {
		t.Error("Should be enabled after Enable")
	}

	// Should check credentials again
	if a.Authenticate("wrong", "wrong") {
		t.Error("Should check credentials when enabled")
	}
	if !a.Authenticate("admin", "secret123") {
		t.Error("Correct credentials should pass")
	}
}

func TestSetPassword(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "oldpass")

	// Verify old password works
	if !a.Authenticate("admin", "oldpass") {
		t.Error("Old password should work")
	}

	// Change password
	a.SetPassword("newpass")

	// Old password should fail
	if a.Authenticate("admin", "oldpass") {
		t.Error("Old password should fail after change")
	}

	// New password should work
	if !a.Authenticate("admin", "newpass") {
		t.Error("New password should work")
	}
}

func TestClearCredentials(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Clear
	a.ClearCredentials()

	if a.IsEnabled() {
		t.Error("Should be disabled after clear")
	}

	if a.GetUsername() != "" {
		t.Error("Username should be empty after clear")
	}
}

func TestToFromMap(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	// Export to map
	m := a.ToMap()

	if m["username"] != "admin" {
		t.Error("Username should be in map")
	}
	if m["password"] == "" {
		t.Error("Password hash should be in map")
	}
	if m["password"] == "secret123" {
		t.Error("Password in map should be hashed")
	}
	if !m["enabled"].(bool) {
		t.Error("Enabled should be true in map")
	}

	// Import to new auth
	a2 := NewAuth()
	a2.FromMap(m)

	if a2.GetUsername() != "admin" {
		t.Error("Username should be restored")
	}
	if !a2.IsEnabled() {
		t.Error("Enabled should be restored")
	}
	if !a2.Authenticate("admin", "secret123") {
		t.Error("Credentials should work after restore")
	}
}

func TestConcurrentAccess(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("admin", "secret123")

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				a.Authenticate("admin", "secret123")
				a.IsEnabled()
				a.GetUsername()
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.SetPassword("newpass")
			a.Enable()
			a.Disable()
			a.Enable()
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestSpecialCharacters(t *testing.T) {
	a := NewAuth()

	// Test password with special characters
	specialPass := "p@ssw0rd!#$%^&*()_+-=[]{}|;':\",./<>?"
	a.SetCredentials("admin", specialPass)

	if !a.Authenticate("admin", specialPass) {
		t.Error("Password with special characters should work")
	}

	// Test username with spaces (should work)
	a.SetCredentials("admin user", "pass")
	if !a.Authenticate("admin user", "pass") {
		t.Error("Username with space should work")
	}
}

func TestEmptyCredentials(t *testing.T) {
	a := NewAuth()

	// Empty username should disable auth
	a.SetCredentials("", "pass")
	if a.IsEnabled() {
		t.Error("Empty username should disable auth")
	}

	// Empty password should still enable auth
	a.SetCredentials("admin", "")
	if !a.IsEnabled() {
		t.Error("Non-empty username should enable auth")
	}

	// But authentication with any password should work
	if !a.Authenticate("admin", "anything") {
		t.Error("Any password should work when empty password set")
	}
}

func TestCaseSensitiveUsername(t *testing.T) {
	a := NewAuth()
	a.SetCredentials("Admin", "pass")

	// Username should be case sensitive
	if a.Authenticate("admin", "pass") {
		t.Error("Username should be case sensitive")
	}
	if !a.Authenticate("Admin", "pass") {
		t.Error("Exact username case should work")
	}
}
