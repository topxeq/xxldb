package mysql

import (
	"fmt"
	"testing"

	"github.com/topxeq/xxldb/executor"
)

// TestMySQLServerIntegration tests the MySQL server with a real MySQL client
// This test requires the MySQL driver to be installed separately
// Run with: go test -v -run TestMySQLServerIntegration ./mysql/...
func TestMySQLServerIntegration(t *testing.T) {
	// Skip this test by default - requires external MySQL driver
	// To run: go get github.com/go-sql-driver/mysql
	t.Skip("Integration test requires MySQL driver - run manually")
}

// TestMySQLServerBasic tests basic server functionality without external driver
func TestMySQLServerBasic(t *testing.T) {
	// Create engine
	engine, err := executor.NewEngine("", true)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	// Create test table
	_, err = engine.Execute("CREATE TABLE users (id SEQ, name VARCHAR(100), age INT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	for i := 0; i < 5; i++ {
		_, err = engine.Execute(fmt.Sprintf("INSERT INTO users (name, age) VALUES ('user%d', %d)", i, 20+i))
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}

	// Create MySQL server
	server, err := NewServer(engine, Config{
		Addr:     ":13307",
		Username: "admin",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(":13307"); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	t.Log("MySQL server started successfully on :13307")

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	t.Log("MySQL server stopped successfully")
}
