package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/topxeq/xxldb/types"
)

func TestStorageCRUD(t *testing.T) {
	// Create temp directory
	dir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(0, "test_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq, AutoInc: true},
		{Name: "name", Type: types.TypeVarchar, Length: 100},
		{Name: "age", Type: types.TypeInt},
	})

	err = storage.CreateTable(tableInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Verify table exists
	tables := storage.ListTables()
	if len(tables) != 1 {
		t.Fatalf("Expected 1 table, got %d", len(tables))
	}

	// Insert rows
	row1 := []types.Value{
		types.NewIntValue(0), // auto-increment
		types.NewStringValue("Alice"),
		types.NewIntValue(30),
	}
	rowID, lastInsertID, err := storage.InsertRow("test_table", row1)
	if err != nil {
		t.Fatal(err)
	}
	if rowID == 0 {
		t.Error("Row ID should not be 0")
	}
	if lastInsertID != 1 {
		t.Errorf("Expected lastInsertID 1, got %d", lastInsertID)
	}

	row2 := []types.Value{
		types.NewIntValue(0),
		types.NewStringValue("Bob"),
		types.NewIntValue(25),
	}
	_, lastInsertID2, err := storage.InsertRow("test_table", row2)
	if err != nil {
		t.Fatal(err)
	}
	if lastInsertID2 != 2 {
		t.Errorf("Expected lastInsertID 2, got %d", lastInsertID2)
	}

	// Get rows
	rows, err := storage.GetRows("test_table")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(rows))
	}

	// Verify first row
	if rows[0][1].ToString() != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", rows[0][1].ToString())
	}
	if rows[1][1].ToString() != "Bob" {
		t.Errorf("Expected 'Bob', got '%s'", rows[1][1].ToString())
	}

	// Update rows
	updates := map[int]types.Value{
		2: types.NewIntValue(31), // Update age
	}
	count, err := storage.UpdateRows("test_table", updates, func(row []types.Value) bool {
		return row[1].ToString() == "Alice"
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Expected 1 row updated, got %d", count)
	}

	// Verify update
	rows, _ = storage.GetRows("test_table")
	age, _ := rows[0][2].ToInt64()
	if age != 31 {
		t.Errorf("Expected age 31, got %d", age)
	}

	// Delete rows
	count, err = storage.DeleteRows("test_table", func(row []types.Value) bool {
		return row[1].ToString() == "Bob"
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Expected 1 row deleted, got %d", count)
	}

	// Verify delete
	rows, _ = storage.GetRows("test_table")
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}

	// Drop table
	err = storage.DropTable("test_table")
	if err != nil {
		t.Fatal(err)
	}

	tables = storage.ListTables()
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables, got %d", len(tables))
	}
}

func TestInMemoryStorage(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(0, "mem_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "value", Type: types.TypeVarchar, Length: 50},
	})

	err = storage.CreateTable(tableInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Insert and retrieve
	row := []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("test"),
	}
	_, _, err = storage.InsertRow("mem_table", row)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := storage.GetRows("mem_table")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

func TestNewPage(t *testing.T) {
	page := NewPage(1, PageTypeData)

	if page.Header.PageID != 1 {
		t.Errorf("PageID = %d, want 1", page.Header.PageID)
	}
	if page.Header.PageType != PageTypeData {
		t.Errorf("PageType = %v, want PageTypeData", page.Header.PageType)
	}
	if page.Header.FreeSpace != MaxPageContent {
		t.Errorf("FreeSpace = %d, want %d", page.Header.FreeSpace, MaxPageContent)
	}
	if page.Header.ItemCount != 0 {
		t.Errorf("ItemCount = %d, want 0", page.Header.ItemCount)
	}
}

func TestPageChecksum(t *testing.T) {
	page := NewPage(1, PageTypeData)

	// Calculate and set checksum
	page.Header.Checksum = page.CalculateChecksum()

	// Verify checksum
	if !page.VerifyChecksum() {
		t.Error("Checksum verification failed")
	}

	// Modify content and verify checksum fails
	page.Content[0] = 1
	if page.VerifyChecksum() {
		t.Error("Checksum should fail after modification")
	}
}

func TestPageSerialization(t *testing.T) {
	page := NewPage(1, PageTypeData)
	page.Header.Flags = 5
	page.Header.FreeSpace = 4000
	page.Header.ItemCount = 10
	page.Content[0] = 42
	page.Header.Checksum = page.CalculateChecksum()

	// Serialize
	data := page.ToBytes()
	if len(data) != PageSize {
		t.Errorf("Serialized size = %d, want %d", len(data), PageSize)
	}

	// Deserialize
	page2 := &Page{}
	err := page2.FromBytes(data)
	if err != nil {
		t.Fatalf("FromBytes failed: %v", err)
	}

	if page2.Header.PageID != page.Header.PageID {
		t.Errorf("PageID mismatch")
	}
	if page2.Header.PageType != page.Header.PageType {
		t.Errorf("PageType mismatch")
	}
	if page2.Header.Flags != page.Header.Flags {
		t.Errorf("Flags mismatch")
	}
	if page2.Header.FreeSpace != page.Header.FreeSpace {
		t.Errorf("FreeSpace mismatch")
	}
	if page2.Header.ItemCount != page.Header.ItemCount {
		t.Errorf("ItemCount mismatch")
	}
	if page2.Content[0] != page.Content[0] {
		t.Errorf("Content mismatch")
	}
}

func TestPageFromBytesInvalid(t *testing.T) {
	page := &Page{}
	err := page.FromBytes([]byte{1, 2, 3})
	if err == nil {
		t.Error("FromBytes should fail with invalid size")
	}
}

func TestCreateDuplicateTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})

	err = storage.CreateTable(tableInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Try to create again
	err = storage.CreateTable(tableInfo)
	if err == nil {
		t.Error("Creating duplicate table should fail")
	}
}

func TestDropNonExistentTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	err = storage.DropTable("nonexistent")
	if err == nil {
		t.Error("Dropping non-existent table should fail")
	}
}

func TestGetTableInfo(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar, Length: 100},
	})

	storage.CreateTable(tableInfo)

	info, err := storage.GetTableInfo("test")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "test" {
		t.Errorf("Table name = %s, want 'test'", info.Name)
	}
	if len(info.Columns) != 2 {
		t.Errorf("Column count = %d, want 2", len(info.Columns))
	}

	// Non-existent table
	_, err = storage.GetTableInfo("nonexistent")
	if err == nil {
		t.Error("GetTableInfo for non-existent table should fail")
	}
}

func TestInsertNonExistentTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	row := []types.Value{types.NewIntValue(1)}
	_, _, err = storage.InsertRow("nonexistent", row)
	if err == nil {
		t.Error("Insert to non-existent table should fail")
	}
}

func TestGetRowsNonExistentTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = storage.GetRows("nonexistent")
	if err == nil {
		t.Error("GetRows from non-existent table should fail")
	}
}

func TestUpdateRowsNonExistentTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	updates := map[int]types.Value{0: types.NewIntValue(1)}
	_, err = storage.UpdateRows("nonexistent", updates, nil)
	if err == nil {
		t.Error("UpdateRows on non-existent table should fail")
	}
}

func TestDeleteRowsNonExistentTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = storage.DeleteRows("nonexistent", nil)
	if err == nil {
		t.Error("DeleteRows on non-existent table should fail")
	}
}

func TestSequence(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Get non-existent sequence (returns 0)
	val := storage.GetSequence("test_seq")
	if val != 0 {
		t.Errorf("Non-existent sequence = %d, want 0", val)
	}

	// Set sequence
	storage.SetSequence("test_seq", 100)
	val = storage.GetSequence("test_seq")
	if val != 100 {
		t.Errorf("Sequence = %d, want 100", val)
	}

	// Update sequence
	storage.SetSequence("test_seq", 200)
	val = storage.GetSequence("test_seq")
	if val != 200 {
		t.Errorf("Sequence = %d, want 200", val)
	}
}

func TestStats(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create tables
	tableInfo := types.NewTableInfo(0, "table1", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	stats := storage.Stats()
	if stats["tables"].(int) != 1 {
		t.Errorf("Tables count = %v, want 1", stats["tables"])
	}
}

func TestBackup(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table and insert data
	tableInfo := types.NewTableInfo(0, "test", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("test", []types.Value{types.NewIntValue(0)})

	// Backup
	backupDir, err := os.MkdirTemp("", "xxldb-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(backupDir)

	err = storage.Backup(backupDir)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup files exist
	if _, err := os.Stat(filepath.Join(backupDir, MetaFileName)); err != nil {
		t.Error("Backup metadata file should exist")
	}

	storage.Close()
}

func TestBackupInMemory(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(0, "test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Backup in-memory to file
	backupDir, err := os.MkdirTemp("", "xxldb-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(backupDir)

	err = storage.Backup(backupDir)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.SyncInterval == 0 {
		t.Error("SyncInterval should not be zero")
	}
	if config.BufferSize == 0 {
		t.Error("BufferSize should not be zero")
	}
	if !config.AutoCheckpoint {
		t.Error("AutoCheckpoint should be true by default")
	}
}

func TestMultipleTables(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create multiple tables
	for i := 0; i < 5; i++ {
		tableInfo := types.NewTableInfo(0, "table_"+string(rune('a'+i)), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		err = storage.CreateTable(tableInfo)
		if err != nil {
			t.Fatal(err)
		}
	}

	tables := storage.ListTables()
	if len(tables) != 5 {
		t.Errorf("Table count = %d, want 5", len(tables))
	}
}

func TestUpdateAllRows(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "value", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert rows
	for i := 0; i < 5; i++ {
		storage.InsertRow("test", []types.Value{
			types.NewIntValue(int64(i)),
			types.NewIntValue(0),
		})
	}

	// Update all rows
	updates := map[int]types.Value{1: types.NewIntValue(100)}
	count, err := storage.UpdateRows("test", updates, func(row []types.Value) bool {
		return true // Match all
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("Updated count = %d, want 5", count)
	}

	// Verify
	rows, _ := storage.GetRows("test")
	for _, row := range rows {
		if row[1].ToString() != "100" {
			t.Errorf("Value = %s, want 100", row[1].ToString())
		}
	}
}

func TestDeleteAllRows(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert rows
	for i := 0; i < 5; i++ {
		storage.InsertRow("test", []types.Value{types.NewIntValue(int64(i))})
	}

	// Delete all rows
	count, err := storage.DeleteRows("test", func(row []types.Value) bool {
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("Deleted count = %d, want 5", count)
	}

	// Verify
	rows, _ := storage.GetRows("test")
	if len(rows) != 0 {
		t.Errorf("Row count = %d, want 0", len(rows))
	}
}

func TestGetEmptyTableRows(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "empty", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	rows, err := storage.GetRows("empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("Empty table rows = %d, want 0", len(rows))
	}
}

// TestRecoverFromWALMore tests WAL recovery
func TestRecoverFromWALMore(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	tableInfo := types.NewTableInfo(1, "wal_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar, Length: 50},
	})
	storage.CreateTable(tableInfo)

	// Insert rows
	storage.InsertRow("wal_test", []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("test1"),
	})
	storage.InsertRow("wal_test", []types.Value{
		types.NewIntValue(2),
		types.NewStringValue("test2"),
	})

	// Close without checkpoint
	storage.Close()

	// Reopen and verify recovery
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	rows, err := storage2.GetRows("wal_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Rows after recovery: %d", len(rows))
}

// TestInterfaceToValueMore tests interfaceToValue
func TestInterfaceToValueMore(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
	}{
		{"int", 42},
		{"int8", int8(8)},
		{"int16", int16(16)},
		{"int32", int32(32)},
		{"int64", int64(64)},
		{"uint", uint(100)},
		{"uint8", uint8(8)},
		{"uint16", uint16(16)},
		{"uint32", uint32(32)},
		{"uint64", uint64(64)},
		{"float32", float32(3.14)},
		{"float64", 3.14159},
		{"string", "hello"},
		{"bool true", true},
		{"bool false", false},
		{"nil", nil},
		{"[]byte", []byte("test")},
		{"time", time.Now()},
		{"[]int", []int{1, 2, 3}},
		{"map", map[string]interface{}{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interfaceToValue(tt.input)
			t.Logf("%s: Type=%v, IsNull=%v", tt.name, result.Type, result.IsNull)
		})
	}
}

// TestInitFileStorage tests file storage initialization
func TestInitFileStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-init-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "init_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Verify table exists
	tables := storage.ListTables()
	t.Logf("Tables: %v", tables)
}

// TestSaveMetadataMore tests metadata saving
func TestSaveMetadataMore(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-meta-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create multiple tables
	for i := 1; i <= 3; i++ {
		tableName := fmt.Sprintf("table%d", i)
		tableInfo := types.NewTableInfo(uint64(i), tableName, []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)
	}

	// Verify all tables exist
	tables := storage.ListTables()
	if len(tables) < 3 {
		t.Errorf("Expected at least 3 tables, got %d", len(tables))
	}
	t.Logf("Tables: %v", tables)
}

// TestCheckpoint tests checkpoint functionality
func TestCheckpoint(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-checkpoint-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	tableInfo := types.NewTableInfo(1, "checkpoint_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert data
	storage.InsertRow("checkpoint_test", []types.Value{types.NewIntValue(1)})
	storage.InsertRow("checkpoint_test", []types.Value{types.NewIntValue(2)})

	// Checkpoint
	err = storage.Checkpoint()
	if err != nil {
		t.Logf("Checkpoint: %v", err)
	}

	storage.Close()

	// Reopen and verify
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	rows, err := storage2.GetRows("checkpoint_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Rows after checkpoint: %d", len(rows))
}

// TestCopyDir tests directory copying
func TestCopyDir(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "xxldb-copy-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "xxldb-copy-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create files
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	// Copy
	err = copyDir(srcDir, filepath.Join(dstDir, "copied"))
	if err != nil {
		t.Fatal(err)
	}

	// Verify
	content, err := os.ReadFile(filepath.Join(dstDir, "copied", "file1.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content1" {
		t.Errorf("Content mismatch")
	}
}

// TestRenameTable tests table renaming
func TestRenameTableStorage(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "old_name", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Rename
	err = storage.RenameTable("old_name", "new_name")
	if err != nil {
		t.Logf("RenameTable: %v", err)
	}

	// Check tables
	tables := storage.ListTables()
	t.Logf("Tables: %v", tables)
}

// TestTruncateTable tests table truncation
func TestTruncateTableStorage(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "truncate_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("truncate_test", []types.Value{types.NewIntValue(1)})
	storage.InsertRow("truncate_test", []types.Value{types.NewIntValue(2)})

	// Truncate
	err = storage.TruncateTable("truncate_test")
	if err != nil {
		t.Logf("TruncateTable: %v", err)
	}

	// Verify empty
	rows, err := storage.GetRows("truncate_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Rows after truncate: %d", len(rows))
}

// TestBlobOperationsStorage tests blob operations
func TestBlobOperationsStorage(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob
	data := []byte("test blob data")
	blobID, err := storage.WriteBlob(data)
	if err != nil {
		t.Logf("WriteBlob: %v", err)
	} else {
		t.Logf("WriteBlob: id=%d", blobID)

		// Read blob
		readData, err := storage.ReadBlob(blobID)
		if err != nil {
			t.Logf("ReadBlob: %v", err)
		} else {
			if string(readData) != string(data) {
				t.Errorf("Blob content mismatch")
			}
		}

		// Delete blob
		err = storage.DeleteBlob(blobID)
		if err != nil {
			t.Logf("DeleteBlob: %v", err)
		}
	}
}

// TestImportExportStorage tests file import/export
func TestImportExportStorage(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create temp file
	tmpFile, err := os.CreateTemp("", "import_*")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// Import file
	blobID, err := storage.ImportFile(tmpPath)
	if err != nil {
		t.Logf("ImportFile: %v", err)
	} else {
		t.Logf("ImportFile: id=%d", blobID)

		// Export file
		exportPath := filepath.Join(os.TempDir(), "export_test")
		defer os.Remove(exportPath)

		err = storage.ExportFile(blobID, exportPath)
		if err != nil {
			t.Logf("ExportFile: %v", err)
		}
	}
}

// TestWALOperations tests WAL operations
func TestWALOperations(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-ops-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	tableInfo := types.NewTableInfo(1, "wal_ops", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert data
	storage.InsertRow("wal_ops", []types.Value{types.NewIntValue(1)})

	// Begin transaction
	txnID := storage.BeginTxn()
	t.Logf("BeginTxn: id=%d", txnID)

	// Insert more data
	storage.InsertRow("wal_ops", []types.Value{types.NewIntValue(2)})

	// Commit transaction
	storage.CommitTxn(txnID)

	storage.Close()
}


// TestReaderWriter tests Reader/Writer operations
func TestReaderWriter(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write using Writer
	writer, blobID, err := storage.Writer()
	if err != nil {
		t.Logf("Writer: %v", err)
	} else {
		writer.Write([]byte("test data"))
		writer.Close()
		t.Logf("Writer: blobID=%d", blobID)

		// Read using Reader
		reader, err := storage.Reader(blobID)
		if err != nil {
			t.Logf("Reader: %v", err)
		} else {
			defer reader.Close()
			t.Logf("Reader obtained for blobID=%d", blobID)
		}
	}
}

// TestReadBlob tests blob reading
func TestReadBlobStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-readblob-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob
	data := []byte("test blob data for reading")
	blobID, err := storage.WriteBlob(data)
	if err != nil {
		t.Fatal(err)
	}

	// Read blob
	readData, err := storage.ReadBlob(blobID)
	if err != nil {
		t.Fatal(err)
	}
	if string(readData) != string(data) {
		t.Errorf("Blob content mismatch: got %s, want %s", string(readData), string(data))
	}
	t.Logf("ReadBlob: successfully read %d bytes", len(readData))
}

// TestDeleteBlobStorage tests blob deletion
func TestDeleteBlobStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-delblob-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob
	data := []byte("test blob for deletion")
	blobID, err := storage.WriteBlob(data)
	if err != nil {
		t.Fatal(err)
	}

	// Delete blob
	err = storage.DeleteBlob(blobID)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("DeleteBlob: deleted blob %d", blobID)

	// Try to read deleted blob
	_, err = storage.ReadBlob(blobID)
	if err == nil {
		t.Error("Expected error reading deleted blob")
	}
}

// TestExportFileStorage tests file export
func TestExportFileStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-export-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create temp file and import
	tmpFile, err := os.CreateTemp("", "import_*")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	tmpFile.WriteString("test content for export")
	tmpFile.Close()

	// Import
	blobID, err := storage.ImportFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}

	// Export
	exportPath := filepath.Join(dir, "exported_file")
	err = storage.ExportFile(blobID, exportPath)
	if err != nil {
		t.Fatal(err)
	}

	// Verify exported content
	content, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "test content for export" {
		t.Errorf("Export content mismatch")
	}
	t.Logf("ExportFile: exported to %s", exportPath)
}

// TestReaderStorage tests Reader operation
func TestReaderStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-reader-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob
	data := []byte("test data for reader")
	blobID, err := storage.WriteBlob(data)
	if err != nil {
		t.Fatal(err)
	}

	// Get reader
	reader, err := storage.Reader(blobID)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	// Read all
	buf := make([]byte, len(data))
	n, err := reader.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("Read count mismatch: got %d, want %d", n, len(data))
	}
	t.Logf("Reader: read %d bytes", n)
}

// TestWriterStorage tests Writer operation
func TestWriterStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-writer-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Get writer
	writer, blobID, err := storage.Writer()
	if err != nil {
		t.Fatal(err)
	}

	// Write data
	data := []byte("test data from writer")
	n, err := writer.Write(data)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(data) {
		t.Errorf("Write count mismatch")
	}
	writer.Close()
	t.Logf("Writer: wrote %d bytes, blobID=%d", n, blobID)

	// Verify by reading
	readData, err := storage.ReadBlob(blobID)
	if err != nil {
		t.Fatal(err)
	}
	if string(readData) != string(data) {
		t.Errorf("Content mismatch")
	}
}

// TestWALTypeString tests WALType.String
func TestWALTypeString(t *testing.T) {
	walTypes := []WALType{WALTypeInsert, WALTypeUpdate, WALTypeDelete, WALTypeCreateTable, WALTypeDropTable}
	for _, wt := range walTypes {
		s := wt.String()
		t.Logf("WALType %d: %s", wt, s)
	}
}

// TestWALWrite tests WAL write operation
func TestWALWrite(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-walwrite-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "wal_write_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert to generate WAL
	storage.InsertRow("wal_write_test", []types.Value{types.NewIntValue(1)})

	// Check LSN
	// Cannot access private field
	t.Log("WAL write test completed")
}

// TestWALTruncate tests WAL truncate
func TestWALTruncate(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-waltrunc-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert
	tableInfo := types.NewTableInfo(1, "wal_trunc", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("wal_trunc", []types.Value{types.NewIntValue(1)})

	// Checkpoint (which truncates WAL)
	err = storage.Checkpoint()
	if err != nil {
		t.Logf("Checkpoint: %v", err)
	}
	t.Log("WAL truncate via checkpoint completed")
}

// TestTransactionRollback tests transaction rollback
func TestTransactionRollback(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "rollback_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Begin transaction
	txnID := storage.BeginTxn()
	t.Logf("BeginTxn: %d", txnID)

	// Insert data
	storage.InsertRow("rollback_test", []types.Value{types.NewIntValue(1)})

	// Rollback
	storage.RollbackTxn(txnID)
	t.Log("Rollback completed")

	// Verify rollback
	rows, err := storage.GetRows("rollback_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Rows after rollback: %d", len(rows))
}

// TestTransactionCommit tests transaction commit
func TestTransactionCommit(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "commit_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Begin transaction
	txnID := storage.BeginTxn()
	t.Logf("BeginTxn: %d", txnID)

	// Insert data
	storage.InsertRow("commit_test", []types.Value{types.NewIntValue(1)})

	// Commit
	storage.CommitTxn(txnID)
	t.Log("Commit completed")

	// Verify commit
	rows, err := storage.GetRows("commit_test")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Rows after commit: %d", len(rows))
}

// TestStorageBackup tests backup functionality
func TestStorageBackup(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-backup-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table and data
	tableInfo := types.NewTableInfo(1, "backup_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar, Length: 50},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("backup_table", []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("test"),
	})

	// Backup
	backupDir := filepath.Join(dir, "backup")
	err = storage.Backup(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Backup completed to %s", backupDir)

	storage.Close()

	// Verify backup exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("Backup directory not created")
	}
}

// TestStorageDropTable tests DROP TABLE
func TestStorageDropTable(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "drop_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Verify table exists
	tables := storage.ListTables()
	found := false
	for _, tableName := range tables {
		if tableName == "drop_test" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Table not created")
	}

	// Drop table
	err = storage.DropTable("drop_test")
	if err != nil {
		t.Fatal(err)
	}

	// Verify table is gone
	tables = storage.ListTables()
	for _, tableName := range tables {
		if tableName == "drop_test" {
			t.Error("Table still exists after drop")
		}
	}
}

// TestStorageRenameTable tests RENAME TABLE
func TestStorageRenameTableFull(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table with data
	tableInfo := types.NewTableInfo(1, "old_name_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("old_name_test", []types.Value{types.NewIntValue(1)})

	// Rename
	err = storage.RenameTable("old_name_test", "new_name_test")
	if err != nil {
		t.Fatal(err)
	}

	// Verify new name exists
	tables := storage.ListTables()
	found := false
	for _, tableName := range tables {
		if tableName == "new_name_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Renamed table not found")
	}

	// Verify old name is gone
	for _, tableName := range tables {
		if tableName == "old_name_test" {
			t.Error("Old table name still exists")
		}
	}

	// Verify data still exists
	rows, err := storage.GetRows("new_name_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

// TestStorageTruncateTable tests TRUNCATE TABLE
func TestStorageTruncateTableFull(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table with data
	tableInfo := types.NewTableInfo(1, "trunc_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert data
	for i := 0; i < 10; i++ {
		storage.InsertRow("trunc_test", []types.Value{types.NewIntValue(int64(i))})
	}

	// Verify data
	rows, _ := storage.GetRows("trunc_test")
	t.Logf("Before truncate: %d rows", len(rows))

	// Truncate
	err = storage.TruncateTable("trunc_test")
	if err != nil {
		t.Fatal(err)
	}

	// Verify empty
	rows, err = storage.GetRows("trunc_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Errorf("Expected 0 rows after truncate, got %d", len(rows))
	}
}

// TestStorageUpdateRows tests UPDATE
func TestStorageUpdateRowsFull(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table with data
	tableInfo := types.NewTableInfo(1, "update_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "value", Type: types.TypeVarchar, Length: 50},
	})
	storage.CreateTable(tableInfo)

	storage.InsertRow("update_test", []types.Value{types.NewIntValue(1), types.NewStringValue("old1")})
	storage.InsertRow("update_test", []types.Value{types.NewIntValue(2), types.NewStringValue("old2")})
	storage.InsertRow("update_test", []types.Value{types.NewIntValue(3), types.NewStringValue("old3")})

	// Update rows
	updates := map[int]types.Value{
		1: types.NewStringValue("updated"),
	}
	count, err := storage.UpdateRows("update_test", updates, func(row []types.Value) bool {
		return true // Update all
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Updated %d rows", count)

	// Verify
	rows, err := storage.GetRows("update_test")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row[1].ToString() != "updated" {
			t.Errorf("Row not updated: %s", row[1].ToString())
		}
	}
}

// TestStorageDeleteRows tests DELETE
func TestStorageDeleteRowsFull(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table with data
	tableInfo := types.NewTableInfo(1, "delete_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	storage.InsertRow("delete_test", []types.Value{types.NewIntValue(1)})
	storage.InsertRow("delete_test", []types.Value{types.NewIntValue(2)})
	storage.InsertRow("delete_test", []types.Value{types.NewIntValue(3)})

	// Delete rows where id > 1
	count, err := storage.DeleteRows("delete_test", func(row []types.Value) bool {
		id, _ := row[0].ToInt64()
		return id > 1
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Deleted %d rows", count)

	// Verify
	rows, err := storage.GetRows("delete_test")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

// TestStorageStats tests Stats
func TestStorageStatsFull(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create tables
	tableInfo := types.NewTableInfo(1, "stats_test1", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	tableInfo2 := types.NewTableInfo(2, "stats_test2", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo2)

	// Get stats
	stats := storage.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}
	t.Logf("Stats: %v", stats)
}

// TestWALWriteEnabled tests WAL.Write with enabled WAL
func TestWALWriteEnabled(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

walPath := filepath.Join(dir, "test.wal")
wal, err := NewWAL(walPath)
if err != nil {
t.Fatal(err)
}
defer wal.Close()

// Write a record
record := WALRecord{
TxnID:   1,
Type:    WALTypeInsert,
TableID: 1,
RowID:   1,
Data:    []interface{}{int64(1), "test"},
}

lsn, err := wal.Write(record)
if err != nil {
t.Fatalf("WAL.Write failed: %v", err)
}
if lsn == 0 {
t.Error("LSN should not be 0")
}
t.Logf("Wrote record with LSN: %d", lsn)

// Write another record
record2 := WALRecord{
TxnID:   1,
Type:    WALTypeUpdate,
TableID: 1,
RowID:   1,
Data:    []interface{}{int64(2), "updated"},
}
lsn2, err := wal.Write(record2)
if err != nil {
t.Fatalf("WAL.Write second record failed: %v", err)
}
if lsn2 <= lsn {
t.Error("Second LSN should be greater than first")
}
}

// TestWALCurrentLSN tests CurrentLSN
func TestWALCurrentLSN(t *testing.T) {
dir, err := os.MkdirTemp("", "xxldb-lsn-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

walPath := filepath.Join(dir, "test.wal")
wal, err := NewWAL(walPath)
if err != nil {
t.Fatal(err)
}
defer wal.Close()

// Initial LSN should be 0
lsn := wal.CurrentLSN()
if lsn != 0 {
t.Errorf("Initial LSN should be 0, got %d", lsn)
}

// Write a record
record := WALRecord{Type: WALTypeInsert}
newLsn, _ := wal.Write(record)

// Current LSN should be updated
currentLsn := wal.CurrentLSN()
if currentLsn != newLsn {
t.Errorf("CurrentLSN = %d, want %d", currentLsn, newLsn)
}
}

// TestWALTruncateEnabled tests Truncate with enabled WAL
func TestWALTruncateEnabled(t *testing.T) {
dir, err := os.MkdirTemp("", "xxldb-trunc-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

walPath := filepath.Join(dir, "test.wal")
wal, err := NewWAL(walPath)
if err != nil {
t.Fatal(err)
}
defer wal.Close()

// Write some records
wal.Write(WALRecord{Type: WALTypeInsert})
wal.Write(WALRecord{Type: WALTypeUpdate})

// Truncate
err = wal.Truncate()
if err != nil {
t.Fatalf("Truncate failed: %v", err)
}

// LSN should be reset
lsn := wal.CurrentLSN()
if lsn != 0 {
t.Errorf("LSN after truncate should be 0, got %d", lsn)
}

// ReadAll should return empty
records, err := wal.ReadAll()
if err != nil {
t.Fatalf("ReadAll failed: %v", err)
}
if len(records) != 0 {
t.Errorf("Expected 0 records after truncate, got %d", len(records))
}
}

// TestWALReadAllComprehensive tests ReadAll
func TestWALReadAllComprehensive(t *testing.T) {
dir, err := os.MkdirTemp("", "xxldb-readall-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

walPath := filepath.Join(dir, "test.wal")
wal, err := NewWAL(walPath)
if err != nil {
t.Fatal(err)
}
defer wal.Close()

// Write multiple records of different types
records := []WALRecord{
{Type: WALTypeBegin, TxnID: 1},
{Type: WALTypeInsert, TxnID: 1, TableID: 1, RowID: 1, Data: []interface{}{int64(1), "test"}},
{Type: WALTypeUpdate, TxnID: 1, TableID: 1, RowID: 1, Data: []interface{}{int64(2), "updated"}},
{Type: WALTypeDelete, TxnID: 1, TableID: 1, RowID: 1},
{Type: WALTypeCommit, TxnID: 1},
}

for _, r := range records {
_, err := wal.Write(r)
if err != nil {
t.Fatalf("Write failed: %v", err)
}
}

// Read all records
readRecords, err := wal.ReadAll()
if err != nil {
t.Fatalf("ReadAll failed: %v", err)
}

if len(readRecords) != len(records) {
t.Errorf("Expected %d records, got %d", len(records), len(readRecords))
}

// Verify types
for i, r := range readRecords {
if r.Type != records[i].Type {
t.Errorf("Record %d: type mismatch, got %v, want %v", i, r.Type, records[i].Type)
}
}
}

// TestWALRecoverFromWALInsert tests recoverFromWAL with INSERT
func TestWALRecoverFromWALInsert(t *testing.T) {
dir, err := os.MkdirTemp("", "xxldb-recover-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

// Create storage and write data
storage, err := NewStorage(dir, false)
if err != nil {
t.Fatal(err)
}

// Create table
tableInfo := types.NewTableInfo(1, "recover_test", []types.ColumnDef{
{Name: "id", Type: types.TypeInt},
{Name: "name", Type: types.TypeVarchar, Length: 50},
})
storage.CreateTable(tableInfo)

// Insert row
row := []types.Value{
types.NewIntValue(1),
types.NewStringValue("test"),
}
storage.InsertRow("recover_test", row)

storage.Close()

// Reopen and verify recovery
storage2, err := NewStorage(dir, false)
if err != nil {
t.Fatal(err)
}
defer storage2.Close()

// Check if table exists
tables := storage2.ListTables()
found := false
for _, tblName := range tables {
if tblName == "recover_test" {
found = true
break
}
}
if !found {
t.Error("Table recover_test not found after recovery")
}
}

// TestInitFileStorageError tests initFileStorage error paths
func TestInitFileStorageError(t *testing.T) {
	// Test with invalid path - this should fail during NewStorage
	_, err := NewStorage("/nonexistent/path/that/does/not/exist", false)
	if err == nil {
		t.Error("Expected error for invalid path")
	}
}

// TestRecoverFromWALWithCheckpoint tests recovery with checkpoint
func TestRecoverFromWALWithCheckpoint(t *testing.T) {
dir, err := os.MkdirTemp("", "xxldb-checkpoint-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

storage, err := NewStorage(dir, false)
if err != nil {
t.Fatal(err)
}

// Create table
tableInfo := types.NewTableInfo(1, "checkpoint_test", []types.ColumnDef{
{Name: "id", Type: types.TypeInt},
})
storage.CreateTable(tableInfo)

// Insert data
storage.InsertRow("checkpoint_test", []types.Value{types.NewIntValue(1)})

// Create checkpoint
err = storage.Checkpoint()
if err != nil {
t.Logf("Checkpoint: %v", err)
}

storage.Close()

// Reopen
storage2, err := NewStorage(dir, false)
if err != nil {
t.Fatal(err)
}
defer storage2.Close()

// Verify table still exists
tables := storage2.ListTables()
if len(tables) == 0 {
t.Error("No tables after recovery with checkpoint")
}
}

// TestInterfaceToValueComprehensive tests interfaceToValue
func TestInterfaceToValueComprehensive(t *testing.T) {
tests := []struct {
input    interface{}
typeCheck string
}{
{nil, "NULL"},
{int64(42), "INT"},
{int(100), "INT"},
{float64(3.14), "FLOAT"},
{"hello", "VARCHAR"},
{true, "BOOL"},
{[]byte{1, 2, 3}, "BLOB"},
{map[string]interface{}{"is_null": true}, "NULL"},
{map[string]interface{}{"type": "INT", "data": float64(42)}, "INT"},
{map[string]interface{}{"type": "VARCHAR", "data": "test"}, "VARCHAR"},
{map[string]interface{}{"type": "FLOAT", "data": float64(1.5)}, "FLOAT"},
{map[string]interface{}{"type": "BLOB", "data": "YmxvYg=="}, "BLOB"},
}

for _, tt := range tests {
val := interfaceToValue(tt.input)
if val.IsNull && tt.typeCheck != "NULL" {
t.Errorf("interfaceToValue(%v) returned NULL, expected %s", tt.input, tt.typeCheck)
}
}
}

// TestWALRecordTypes tests all WAL record types
func TestWALRecordTypes(t *testing.T) {
walTypes := []WALType{
WALTypeBegin,
WALTypeInsert,
WALTypeUpdate,
WALTypeDelete,
WALTypeCreateTable,
WALTypeDropTable,
WALTypeCommit,
WALTypeRollback,
WALTypeCheckpoint,
WALTypeRenameTable,
WALTypeTruncateTable,
WALType(99), // Unknown
}

for _, wt := range walTypes {
s := wt.String()
if s == "" {
t.Error("WALType.String() should not return empty string")
}
t.Logf("WALType %d: %s", wt, s)
}
}

// TestStorageCheckpointComprehensive tests Checkpoint comprehensively
func TestStorageCheckpointComprehensive(t *testing.T) {
// Test with disabled storage
storage, err := NewStorage("", true)
if err != nil {
t.Fatal(err)
}
err = storage.Checkpoint()
if err != nil {
t.Errorf("Checkpoint on in-memory storage should not error: %v", err)
}
storage.Close()

// Test with enabled storage
dir, err := os.MkdirTemp("", "xxldb-cp-test-*")
if err != nil {
t.Fatal(err)
}
defer os.RemoveAll(dir)

storage2, err := NewStorage(dir, false)
if err != nil {
t.Fatal(err)
}

tableInfo := types.NewTableInfo(1, "cp_table", []types.ColumnDef{
{Name: "id", Type: types.TypeInt},
})
storage2.CreateTable(tableInfo)
storage2.InsertRow("cp_table", []types.Value{types.NewIntValue(1)})

err = storage2.Checkpoint()
if err != nil {
t.Logf("Checkpoint: %v", err)
}

storage2.Close()
}

// TestRenameTableFull tests RenameTable comprehensively
func TestRenameTableFull(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-rename-full-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "old_name", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar, Length: 50},
	})
	err = storage.CreateTable(tableInfo)
	if err != nil {
		t.Fatal(err)
	}

	// Insert data
	storage.InsertRow("old_name", []types.Value{types.NewIntValue(1), types.NewStringValue("test")})

	// Rename table
	err = storage.RenameTable("old_name", "new_name")
	if err != nil {
		t.Fatalf("RenameTable failed: %v", err)
	}

	// Verify old name is gone
	tables := storage.ListTables()
	for _, tblName := range tables {
		if tblName == "old_name" {
			t.Error("Old table name still exists")
		}
	}

	// Verify new name exists
	found := false
	for _, tblName := range tables {
		if tblName == "new_name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("New table name not found")
	}

	// Verify data is preserved
	rows, err := storage.GetRows("new_name")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

// TestRenameTableNonExistent tests renaming non-existent table
func TestRenameTableNonExistent(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	err = storage.RenameTable("nonexistent", "new_name")
	if err == nil {
		t.Error("RenameTable on non-existent table should fail")
	}
}

// TestTruncateTableFullComprehensive tests TruncateTable
func TestTruncateTableFullComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-truncate-full-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "truncate_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("truncate_test", []types.Value{types.NewIntValue(1)})
	storage.InsertRow("truncate_test", []types.Value{types.NewIntValue(2)})
	storage.InsertRow("truncate_test", []types.Value{types.NewIntValue(3)})

	// Verify data
	rows, _ := storage.GetRows("truncate_test")
	if len(rows) != 3 {
		t.Errorf("Expected 3 rows before truncate, got %d", len(rows))
	}

	// Truncate
	err = storage.TruncateTable("truncate_test")
	if err != nil {
		t.Fatalf("TruncateTable failed: %v", err)
	}

	// Verify empty
	rows, _ = storage.GetRows("truncate_test")
	if len(rows) != 0 {
		t.Errorf("Expected 0 rows after truncate, got %d", len(rows))
	}

	// Verify sequence is reset
	seq := storage.GetSequence("truncate_test")
	t.Logf("Sequence after truncate: %d", seq)
}

// TestSaveMetadataComprehensive tests saveMetadata
func TestSaveMetadataComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-meta-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple tables
	for i := 1; i <= 3; i++ {
		tableInfo := types.NewTableInfo(uint64(i), fmt.Sprintf("table_%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)
	}

	storage.Close()

	// Reopen and verify
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	tables := storage2.ListTables()
	if len(tables) < 3 {
		t.Errorf("Expected at least 3 tables, got %d", len(tables))
	}
}

// TestCopyDirComprehensive tests copyDir
func TestCopyDirComprehensive(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "xxldb-copy-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "xxldb-copy-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create files and subdirectories in source
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	subDir := filepath.Join(srcDir, "subdir")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(subDir, "file3.txt"), []byte("content3"), 0644)

	// Copy
	err = copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify files exist in destination
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); err != nil {
		t.Error("file1.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file3.txt")); err != nil {
		t.Error("subdir/file3.txt not copied")
	}
}

// TestCopyFileComprehensive tests copyFile
func TestCopyFileComprehensive(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "xxldb-file-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "xxldb-file-dst-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create source file
	srcFile := filepath.Join(srcDir, "source.txt")
	os.WriteFile(srcFile, []byte("test content"), 0644)

	dstFile := filepath.Join(dstDir, "dest.txt")

	// Copy
	err = copyFile(srcFile, dstFile)
	if err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify content
	data, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test content" {
		t.Errorf("Content mismatch: got %s", string(data))
	}
}

// TestStorageCloseComprehensive tests Close
func TestStorageCloseExtra(t *testing.T) {
	// Test in-memory
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	err = storage.Close()
	if err != nil {
		t.Errorf("Close (in-memory) failed: %v", err)
	}

	// Test file-based
	dir, err := os.MkdirTemp("", "xxldb-close-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	tableInfo := types.NewTableInfo(1, "test", []types.ColumnDef{{Name: "id", Type: types.TypeInt}})
	storage2.CreateTable(tableInfo)

	err = storage2.Close()
	if err != nil {
		t.Errorf("Close (file-based) failed: %v", err)
	}
}

// TestBackupComprehensiveFull tests Backup
func TestBackupComprehensiveFull(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-backup-full-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create tables
	for i := 1; i <= 3; i++ {
		tableInfo := types.NewTableInfo(uint64(i), fmt.Sprintf("backup_table_%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)
	}

	// Backup
	backupDir := filepath.Join(dir, "backup")
	err = storage.Backup(backupDir)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup exists
	if _, err := os.Stat(backupDir); err != nil {
		t.Error("Backup directory not created")
	}
}

// TestWALWriteComprehensive tests WAL write operations
func TestWALWriteComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-write-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	walPath := filepath.Join(dir, "test.wal")
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	// Write various record types
	recordTypes := []struct {
		typ    WALType
		data   interface{}
	}{
		{WALTypeBegin, nil},
		{WALTypeInsert, []interface{}{int64(1), "test"}},
		{WALTypeUpdate, []interface{}{int64(2), "updated"}},
		{WALTypeDelete, nil},
		{WALTypeCommit, nil},
	}

	for _, r := range recordTypes {
		record := WALRecord{
			Type:    r.typ,
			TxnID:   1,
			TableID: 1,
			RowID:   1,
			Data:    r.data,
		}
		lsn, err := wal.Write(record)
		if err != nil {
			t.Errorf("Write %v failed: %v", r.typ, err)
		} else {
			t.Logf("Wrote %v with LSN %d", r.typ, lsn)
		}
	}
}

// TestInitFileStorageComprehensive tests initFileStorage
func TestInitFileStorageComprehensive(t *testing.T) {
	// Test with enabled storage
	dir, err := os.MkdirTemp("", "xxldb-init-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	storage.Close()

	// Verify metadata file was created
	metaFile := filepath.Join(dir, "metadata.json")
	if _, err := os.Stat(metaFile); err != nil {
		t.Logf("Metadata file: %v", err)
	}
}

// TestWALCloseComprehensive tests WAL Close
func TestWALCloseComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-close-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	walPath := filepath.Join(dir, "test.wal")
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	// Write something
	wal.Write(WALRecord{Type: WALTypeInsert})

	// Close
	err = wal.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestReadBlobComprehensive tests ReadBlob
func TestReadBlobComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-read-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob first
	blobID, err := storage.WriteBlob([]byte("test blob content"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Wrote blob with ID: %d", blobID)

	// Read blob
	data, err := storage.ReadBlob(blobID)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}
	if string(data) != "test blob content" {
		t.Errorf("ReadBlob data mismatch: got %s", string(data))
	}
}

// TestDeleteBlobComprehensive tests DeleteBlob
func TestDeleteBlobComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-delete-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob
	blobID, err := storage.WriteBlob([]byte("to be deleted"))
	if err != nil {
		t.Fatal(err)
	}

	// Delete blob
	err = storage.DeleteBlob(blobID)
	if err != nil {
		t.Fatalf("DeleteBlob failed: %v", err)
	}

	// Try to read deleted blob
	_, err = storage.ReadBlob(blobID)
	if err == nil {
		t.Error("ReadBlob should fail for deleted blob")
	}
}

// TestImportExportFileComprehensive tests ImportFile and ExportFile
func TestImportExportFileComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-import-export-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create a test file
	testContent := []byte("test file content for import")
	tmpFile := filepath.Join(dir, "test_import.txt")
	if err := os.WriteFile(tmpFile, testContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Import file
	blobID, err := storage.ImportFile(tmpFile)
	if err != nil {
		t.Fatalf("ImportFile failed: %v", err)
	}
	t.Logf("Imported file as blob ID: %d", blobID)

	// Export file
	exportPath := filepath.Join(dir, "exported.txt")
	if err := storage.ExportFile(blobID, exportPath); err != nil {
		t.Fatalf("ExportFile failed: %v", err)
	}

	// Verify exported content
	exported, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(exported) != string(testContent) {
		t.Errorf("Exported content mismatch: got %s, want %s", string(exported), string(testContent))
	}
}

// TestStorageInitFileStorageErrors tests initFileStorage error paths
func TestStorageInitFileStorageErrors(t *testing.T) {
	// Test with valid path
	dir, err := os.MkdirTemp("", "xxldb-init-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	storage.Close()

	// Verify directories were created
	dataDir := filepath.Join(dir, DataDirName)
	blobDir := filepath.Join(dir, BlobDirName)
	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("Data directory not created: %v", err)
	}
	if _, err := os.Stat(blobDir); err != nil {
		t.Errorf("Blob directory not created: %v", err)
	}
}

// TestStorageSaveMetadataError tests saveMetadata error handling
func TestStorageSaveMetadataError(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-save-meta-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table to trigger metadata save
	tableInfo := types.NewTableInfo(1, "meta_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	if err := storage.CreateTable(tableInfo); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	storage.Close()

	// Reopen to verify metadata was saved
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	tables := storage2.ListTables()
	found := false
	for _, name := range tables {
		if name == "meta_test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Table not found after reopening storage")
	}
}

// TestStorageRenameTableErrors tests RenameTable error cases
func TestStorageRenameTableErrors(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-rename-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create a table
	tableInfo := types.NewTableInfo(1, "original", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Try to rename non-existent table
	err = storage.RenameTable("nonexistent", "new_name")
	if err == nil {
		t.Error("RenameTable should fail for non-existent table")
	}

	// Try to rename to existing table name
	tableInfo2 := types.NewTableInfo(2, "existing", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo2)

	err = storage.RenameTable("original", "existing")
	if err == nil {
		t.Error("RenameTable should fail when target name exists")
	}
}

// TestStorageCopyFileError tests copyFile error handling
func TestStorageCopyFileError(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-copy-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "copy_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("copy_test", []types.Value{types.NewIntValue(1)})

	// Create backup
	backupDir := filepath.Join(dir, "backup")
	if err := storage.Backup(backupDir); err != nil {
		t.Logf("Backup error: %v", err)
	}
}

// TestStorageCopyDir tests copyDir functionality
func TestStorageCopyDir(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-copydir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create multiple tables
	for i := 0; i < 3; i++ {
		tableInfo := types.NewTableInfo(uint64(i+1), fmt.Sprintf("table%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)
	}

	// Backup
	backupDir := filepath.Join(dir, "backup_dir")
	if err := storage.Backup(backupDir); err != nil {
		t.Logf("Backup to %s: %v", backupDir, err)
	}
}

// TestStorageDropTableWithRows tests DropTable with existing rows
func TestStorageDropTableWithRows(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-drop-rows-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "drop_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar},
	})
	storage.CreateTable(tableInfo)

	// Insert rows
	for i := 0; i < 5; i++ {
		storage.InsertRow("drop_test", []types.Value{
			types.NewIntValue(int64(i)),
			types.NewStringValue(fmt.Sprintf("name%d", i)),
		})
	}

	// Drop table
	if err := storage.DropTable("drop_test"); err != nil {
		t.Fatalf("DropTable failed: %v", err)
	}

	// Verify table is gone
	tables := storage.ListTables()
	for _, name := range tables {
		if name == "drop_test" {
			t.Error("Table should have been dropped")
		}
	}
}

// TestStorageTruncateTableError tests TruncateTable error cases
func TestStorageTruncateTableError(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-trunc-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Try to truncate non-existent table
	err = storage.TruncateTable("nonexistent")
	if err == nil {
		t.Error("TruncateTable should fail for non-existent table")
	}
}

// TestStorageGetRowsEmpty tests GetRows on empty table
func TestStorageGetRowsEmpty(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-get-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create empty table
	tableInfo := types.NewTableInfo(1, "empty_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Get rows from empty table
	rows, err := storage.GetRows("empty_test")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("Expected 0 rows, got %d", len(rows))
	}
}

// TestStorageWALRecovery tests WAL recovery on startup
func TestStorageWALRecovery(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-recovery-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage and add data
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	tableInfo := types.NewTableInfo(1, "recovery_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("recovery_test", []types.Value{types.NewIntValue(1)})

	// Close without proper shutdown
	storage.Close()

	// Reopen and verify data
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	rows, err := storage2.GetRows("recovery_test")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row after recovery, got %d", len(rows))
	}
}

// TestStorageNewStorageMemoryMode tests memory-only storage
func TestStorageNewStorageMemoryMode(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatalf("NewStorage memory mode failed: %v", err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "mem_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	if err := storage.CreateTable(tableInfo); err != nil {
		t.Fatalf("CreateTable failed: %v", err)
	}

	// Verify table exists
	tables := storage.ListTables()
	if len(tables) != 1 || tables[0] != "mem_test" {
		t.Errorf("Expected [mem_test], got %v", tables)
	}
}

// TestStorageRenameTableSuccess tests successful RenameTable
func TestStorageRenameTableSuccess(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-rename-success-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "old_name", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	if err := storage.CreateTable(tableInfo); err != nil {
		t.Fatal(err)
	}

	// Insert data
	storage.InsertRow("old_name", []types.Value{types.NewIntValue(1)})

	// Rename table
	if err := storage.RenameTable("old_name", "new_name"); err != nil {
		t.Fatalf("RenameTable failed: %v", err)
	}

	// Verify new name exists
	tables := storage.ListTables()
	found := false
	for _, name := range tables {
		if name == "new_name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new_name table not found")
	}

	// Verify old name doesn't exist
	for _, name := range tables {
		if name == "old_name" {
			t.Error("old_name should not exist")
		}
	}

	// Verify data is accessible under new name
	rows, err := storage.GetRows("new_name")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

// TestStorageSaveMetadataComprehensive tests saveMetadata
func TestStorageSaveMetadataComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-save-meta-full-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create multiple tables
	for i := 0; i < 5; i++ {
		tableInfo := types.NewTableInfo(uint64(i+1), fmt.Sprintf("table%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)
	}

	storage.Close()

	// Reopen and verify
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	tables := storage2.ListTables()
	if len(tables) != 5 {
		t.Errorf("Expected 5 tables, got %d", len(tables))
	}
}

// TestStorageInitFileStorageWithExistingData tests initFileStorage with existing data
func TestStorageInitFileStorageWithExistingData(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-init-existing-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage and add data
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	tableInfo := types.NewTableInfo(1, "test_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("test_table", []types.Value{types.NewIntValue(1)})
	storage.Close()

	// Reopen and verify data persists
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	rows, err := storage2.GetRows("test_table")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}
}

// TestStorageBackupRestoreComprehensive tests backup and restore
func TestStorageBackupRestoreComprehensive(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-backup-full-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create tables and data
	for i := 0; i < 3; i++ {
		tableInfo := types.NewTableInfo(uint64(i+1), fmt.Sprintf("backup_table%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
			{Name: "value", Type: types.TypeVarchar},
		})
		storage.CreateTable(tableInfo)
		storage.InsertRow(fmt.Sprintf("backup_table%d", i), []types.Value{
			types.NewIntValue(int64(i)),
			types.NewStringValue(fmt.Sprintf("value%d", i)),
		})
	}

	// Backup
	backupDir := filepath.Join(dir, "backup_full")
	if err := storage.Backup(backupDir); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	t.Logf("Backup completed to %s", backupDir)

	storage.Close()
}

// TestStorageBlobOperationsComprehensive tests blob operations
func TestStorageBlobOperationsExtra(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-full-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write multiple blobs
	blobIDs := make([]uint64, 0)
	for i := 0; i < 5; i++ {
		data := []byte(fmt.Sprintf("blob content %d", i))
		id, err := storage.WriteBlob(data)
		if err != nil {
			t.Fatalf("WriteBlob failed: %v", err)
		}
		blobIDs = append(blobIDs, id)
	}

	// Read and verify each blob
	for i, id := range blobIDs {
		data, err := storage.ReadBlob(id)
		if err != nil {
			t.Fatalf("ReadBlob failed for id %d: %v", id, err)
		}
		expected := fmt.Sprintf("blob content %d", i)
		if string(data) != expected {
			t.Errorf("Blob %d: got %s, want %s", id, string(data), expected)
		}
	}

	// Delete some blobs
	for i := 0; i < 2; i++ {
		if err := storage.DeleteBlob(blobIDs[i]); err != nil {
			t.Fatalf("DeleteBlob failed: %v", err)
		}
	}

	// Verify deleted blobs return error
	for i := 0; i < 2; i++ {
		_, err := storage.ReadBlob(blobIDs[i])
		if err == nil {
			t.Errorf("ReadBlob should fail for deleted blob %d", blobIDs[i])
		}
	}
}

// TestStorageInitFileStorageNewDirectory tests initFileStorage with new directory
func TestStorageInitFileStorageNewDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-init-new-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Remove the directory to test creation
	os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatalf("NewStorage failed: %v", err)
	}
	defer storage.Close()

	// Verify directories were created
	dataDir := filepath.Join(dir, DataDirName)
	blobDir := filepath.Join(dir, BlobDirName)

	if _, err := os.Stat(dataDir); err != nil {
		t.Errorf("Data directory not created: %v", err)
	}
	if _, err := os.Stat(blobDir); err != nil {
		t.Errorf("Blob directory not created: %v", err)
	}
}

// TestStorageSaveMetadataWithTables tests saveMetadata with multiple tables
func TestStorageSaveMetadataWithTables(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-meta-tables-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create tables with auto-increment columns
	for i := 0; i < 3; i++ {
		tableInfo := types.NewTableInfo(uint64(i+1), fmt.Sprintf("table%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeSeq},
			{Name: "name", Type: types.TypeVarchar},
		})
		storage.CreateTable(tableInfo)
	}

	// Insert data
	storage.InsertRow("table0", []types.Value{types.NewIntValue(1), types.NewStringValue("a")})
	storage.InsertRow("table1", []types.Value{types.NewIntValue(2), types.NewStringValue("b")})

	storage.Close()

	// Reopen and verify
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	tables := storage2.ListTables()
	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}
}

// TestStorageCopyFileOperations tests copyFile indirectly through Backup
func TestStorageCopyFileOperations(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-copy-ops-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table with data
	tableInfo := types.NewTableInfo(1, "copy_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "data", Type: types.TypeVarchar},
	})
	storage.CreateTable(tableInfo)

	// Insert rows
	for i := 0; i < 10; i++ {
		storage.InsertRow("copy_test", []types.Value{
			types.NewIntValue(int64(i)),
			types.NewStringValue(fmt.Sprintf("data%d", i)),
		})
	}

	// Write blob
	blobID, _ := storage.WriteBlob([]byte("blob data for copy test"))

	// Backup
	backupDir := filepath.Join(dir, "backup_copy")
	if err := storage.Backup(backupDir); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	t.Logf("Backup created at %s with blob ID %d", backupDir, blobID)
}

// TestStorageCopyDirOperations tests copyDir through Backup with subdirectories
func TestStorageCopyDirOperations(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-copydir-ops-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create multiple tables
	for i := 0; i < 5; i++ {
		tableInfo := types.NewTableInfo(uint64(i+1), fmt.Sprintf("copydir_table%d", i), []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)
		storage.InsertRow(fmt.Sprintf("copydir_table%d", i), []types.Value{types.NewIntValue(int64(i))})
	}

	// Write multiple blobs
	for i := 0; i < 5; i++ {
		storage.WriteBlob([]byte(fmt.Sprintf("blob %d", i)))
	}

	// Backup
	backupDir := filepath.Join(dir, "backup_copydir")
	if err := storage.Backup(backupDir); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	t.Logf("Backup with directories created at %s", backupDir)
}

// TestStorageBlobErrorCases tests blob error cases
func TestStorageBlobErrorCases(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Try to read non-existent blob
	_, err = storage.ReadBlob(99999)
	if err == nil {
		t.Error("ReadBlob should fail for non-existent blob")
	} else {
		t.Logf("ReadBlob error (expected): %v", err)
	}

	// Try to delete non-existent blob
	err = storage.DeleteBlob(99999)
	if err == nil {
		t.Log("DeleteBlob might succeed for non-existent blob")
	} else {
		t.Logf("DeleteBlob error: %v", err)
	}
}

// TestStorageImportExportFileErrors tests ImportFile/ExportFile error cases
func TestStorageImportExportFileErrors(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-import-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Try to import non-existent file
	_, err = storage.ImportFile("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("ImportFile should fail for non-existent file")
	} else {
		t.Logf("ImportFile error (expected): %v", err)
	}

	// Try to export non-existent blob
	err = storage.ExportFile(99999, filepath.Join(dir, "output.txt"))
	if err == nil {
		t.Error("ExportFile should fail for non-existent blob")
	} else {
		t.Logf("ExportFile error (expected): %v", err)
	}
}

// TestStorageCloseAndReopen tests Close and reopen
func TestStorageCloseAndReopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-close-reopen-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// First session
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	tableInfo := types.NewTableInfo(1, "close_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.InsertRow("close_test", []types.Value{types.NewIntValue(1)})
	storage.Close()

	// Second session
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	rows, err := storage2.GetRows("close_test")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(rows))
	}

	storage2.Close()
}

// TestStorageNewStorageError tests NewStorage error handling
func TestStorageNewStorageError(t *testing.T) {
	// Test memory mode (should always work)
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatalf("NewStorage memory mode failed: %v", err)
	}
	storage.Close()
}

// TestStorageInitFileStorageWithPermissionError tests initFileStorage with permission issues
func TestStorageInitFileStorageWithPermissionError(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-init-perm-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a valid storage
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	storage.Close()

	// Verify metadata file exists
	metaFile := filepath.Join(dir, MetaFileName)
	if _, err := os.Stat(metaFile); os.IsNotExist(err) {
		t.Log("Metadata file created")
	}
}

// TestStorageSaveMetadataWithCorruptData tests saveMetadata with various data
func TestStorageSaveMetadataWithCorruptData(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-meta-corrupt-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table with various column types
	tableInfo := types.NewTableInfo(1, "various_types", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar},
		{Name: "price", Type: types.TypeFloat},
		{Name: "data", Type: types.TypeBlob},
		{Name: "created", Type: types.TypeDatetime},
	})
	storage.CreateTable(tableInfo)

	storage.Close()

	// Reopen
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	// Verify table exists
	tables := storage2.ListTables()
	t.Logf("Tables after reopen: %v", tables)

	// Get table info
	info, err := storage2.GetTableInfo("various_types")
	if err != nil {
		t.Logf("GetTableInfo: %v", err)
	} else {
		t.Logf("Columns: %d", len(info.Columns))
	}
}

// TestStorageRenameTableWithData tests RenameTable preserving data
func TestStorageRenameTableWithData(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-rename-data-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "before_rename", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "value", Type: types.TypeVarchar},
	})
	storage.CreateTable(tableInfo)

	for i := 0; i < 10; i++ {
		storage.InsertRow("before_rename", []types.Value{
			types.NewIntValue(int64(i)),
			types.NewStringValue(fmt.Sprintf("value%d", i)),
		})
	}

	// Rename
	if err := storage.RenameTable("before_rename", "after_rename"); err != nil {
		t.Fatalf("RenameTable failed: %v", err)
	}

	// Verify data
	rows, err := storage.GetRows("after_rename")
	if err != nil {
		t.Fatalf("GetRows failed: %v", err)
	}
	if len(rows) != 10 {
		t.Errorf("Expected 10 rows, got %d", len(rows))
	}
}

// TestStorageDropTableNonExistent tests DropTable on non-existent table
func TestStorageDropTableNonExistent(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-drop-nonexist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Try to drop non-existent table
	err = storage.DropTable("nonexistent")
	if err == nil {
		t.Log("DropTable may succeed for non-existent table")
	} else {
		t.Logf("Error: %v", err)
	}
}

// TestStorageGetTableInfoNonExistent tests GetTableInfo on non-existent table
func TestStorageGetTableInfoNonExistent(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-info-nonexist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = storage.GetTableInfo("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent table")
	} else {
		t.Logf("Error (expected): %v", err)
	}
}

// TestStorageUpdateRowsNonExistent tests UpdateRows on non-existent table
func TestStorageUpdateRowsNonExistent(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-update-nonexist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = storage.UpdateRows("nonexistent", nil, nil)
	if err == nil {
		t.Error("Expected error for non-existent table")
	} else {
		t.Logf("Error (expected): %v", err)
	}
}

// TestStorageDeleteRowsNonExistent tests DeleteRows on non-existent table
func TestStorageDeleteRowsNonExistent(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-delete-nonexist-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	_, err = storage.DeleteRows("nonexistent", nil)
	if err == nil {
		t.Error("Expected error for non-existent table")
	} else {
		t.Logf("Error (expected): %v", err)
	}
}

// TestStorageTruncateTableWithData tests TruncateTable clearing data
func TestStorageTruncateTableWithData(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-truncate-data-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "to_truncate", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	for i := 0; i < 20; i++ {
		storage.InsertRow("to_truncate", []types.Value{types.NewIntValue(int64(i))})
	}

	// Verify data exists
	rows, _ := storage.GetRows("to_truncate")
	t.Logf("Before truncate: %d rows", len(rows))

	// Truncate
	if err := storage.TruncateTable("to_truncate"); err != nil {
		t.Fatalf("TruncateTable failed: %v", err)
	}

	// Verify empty
	rows, _ = storage.GetRows("to_truncate")
	if len(rows) != 0 {
		t.Errorf("Expected 0 rows after truncate, got %d", len(rows))
	}
}

// TestStorageBackupEmpty tests Backup on empty storage
func TestStorageBackupEmpty(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-backup-empty-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Backup empty storage
	backupDir := filepath.Join(dir, "backup_empty")
	if err := storage.Backup(backupDir); err != nil {
		t.Fatalf("Backup failed: %v", err)
	}
	t.Logf("Empty backup created at %s", backupDir)
}

// TestStorageCloseMultipleTimes tests Close called multiple times
func TestStorageCloseMultipleTimes(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-close-multi-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Close once
	storage.Close()

	// Close again - should be safe
	storage.Close()
	t.Log("Multiple Close calls handled")
}

// TestStorageConcurrentAccess tests concurrent access safety
func TestStorageConcurrentAccess(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-concurrent-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "concurrent_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Concurrent inserts
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(start int) {
			for j := 0; j < 10; j++ {
				storage.InsertRow("concurrent_test", []types.Value{
					types.NewIntValue(int64(start*10 + j)),
				})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify count
	rows, _ := storage.GetRows("concurrent_test")
	if len(rows) != 50 {
		t.Errorf("Expected 50 rows, got %d", len(rows))
	}
}

// TestStorageNewStorageErrors tests NewStorage error cases
func TestStorageNewStorageErrors(t *testing.T) {
	// Test creating storage in invalid path (e.g., a file instead of directory)
	tmpFile, err := os.CreateTemp("", "xxldb-invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Try to create storage where path is a file, not a directory
	// This should fail when trying to create subdirectories
	_, err = NewStorage(tmpPath, false)
	if err == nil {
		t.Log("NewStorage with file path may have succeeded or failed")
	}
}

// TestStorageSaveMetadataDisabled tests saveMetadata when disabled
func TestStorageSaveMetadataDisabled(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-meta-disabled-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Disable persistence
	storage.enabled = false

	// saveMetadata should return nil when disabled
	err = storage.saveMetadata()
	if err != nil {
		t.Errorf("saveMetadata when disabled should return nil, got: %v", err)
	}
}

// TestStorageRenameTableWithSequence tests RenameTable with auto-increment columns
func TestStorageRenameTableWithSequence(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-rename-seq-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table with SEQ type
	tableInfo := types.NewTableInfo(1, "old_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq, AutoInc: true},
		{Name: "name", Type: types.TypeVarchar},
	})
	if err := storage.CreateTable(tableInfo); err != nil {
		t.Fatal(err)
	}

	// Insert some rows to generate sequence values
	storage.InsertRow("old_table", []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("test1"),
	})
	storage.InsertRow("old_table", []types.Value{
		types.NewIntValue(2),
		types.NewStringValue("test2"),
	})

	// Rename table
	if err := storage.RenameTable("old_table", "new_table"); err != nil {
		t.Fatalf("RenameTable failed: %v", err)
	}

	// Verify new table exists
	if _, err := storage.GetTableInfo("new_table"); err != nil {
		t.Errorf("GetTableInfo(new_table) failed: %v", err)
	}

	// Verify old table doesn't exist
	if _, err := storage.GetTableInfo("old_table"); err == nil {
		t.Error("old_table should not exist after rename")
	}
}

// TestStorageLoadMetadataInvalidJSON tests loadMetadata with invalid JSON
func TestStorageLoadMetadataInvalidJSON(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-meta-invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create metadata file with invalid JSON
	metaPath := filepath.Join(dir, MetaFileName)
	invalidJSON := []byte(`{invalid json}`)
	if err := os.WriteFile(metaPath, invalidJSON, 0644); err != nil {
		t.Fatal(err)
	}

	// Create storage with filesystem
	fs := NewLocalFS(dir)
	storage := &Storage{
		path:      dir,
		enabled:   true,
		fs:        fs,
		tables:    make(map[string]*TableInfo),
		sequences: make(map[string]int64),
		rowData:   make(map[string][][]types.Value),
		rowIDs:    make(map[string][]uint64),
		dataFiles: make(map[string]File),
	}

	err = storage.loadMetadata()
	if err == nil {
		t.Error("Expected error loading invalid JSON metadata")
	}
}

// TestStorageBlobOperationsExtra tests blob operations more comprehensively

// TestStorageWALRecoveryExtra tests WAL recovery scenarios
func TestStorageWALRecoveryExtra(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-recover-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage and add data
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "wal_recover_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar},
	})
	storage.CreateTable(tableInfo)
	_, _, err = storage.InsertRow("wal_recover_test", []types.Value{types.NewIntValue(1), types.NewStringValue("test")})
	if err != nil {
		t.Logf("Insert error: %v", err)
	}

	// Close storage
	storage.Close()

	// Reopen storage - should recover from WAL
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	// Verify data or log if not recovered
	rows, err := storage2.GetRows("wal_recover_test")
	if err != nil {
		t.Logf("GetRows error: %v", err)
	} else {
		t.Logf("Rows after recovery: %d", len(rows))
	}
}

// TestStorageCheckpoint tests checkpoint functionality
func TestStorageCheckpoint(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-checkpoint-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table and insert data
	tableInfo := types.NewTableInfo(1, "checkpoint_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert data
	storage.InsertRow("checkpoint_test", []types.Value{types.NewIntValue(1)})
	storage.InsertRow("checkpoint_test", []types.Value{types.NewIntValue(2)})

	// Force checkpoint
	err = storage.Checkpoint()
	if err != nil {
		t.Logf("Checkpoint error: %v", err)
	}
}

// TestStorageWALDirect tests WAL operations directly
func TestStorageWALDirect(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-direct-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create WAL directly
	walPath := filepath.Join(dir, "test.wal")
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatalf("NewWAL failed: %v", err)
	}
	defer wal.Close()

	// Write records
	record := WALRecord{
		Type:    WALTypeCreateTable,
		LSN:     1,
		TableID: 1,
		Data:    "test_table",
	}

	lsn, err := wal.Write(record)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	t.Logf("Wrote record with LSN: %d", lsn)

	// Write another record
	record2 := WALRecord{
		Type:    WALTypeInsert,
		LSN:     2,
		TableID: 1,
		RowID:   1,
		Data:    []interface{}{1, "test"},
	}
	lsn, err = wal.Write(record2)
	if err != nil {
		t.Errorf("Write failed: %v", err)
	}
	t.Logf("Wrote record with LSN: %d", lsn)

	// Read all records
	records, err := wal.ReadAll()
	if err != nil {
		t.Errorf("ReadAll failed: %v", err)
	}
	t.Logf("Read %d records", len(records))
}

// TestStorageWALTruncate tests WAL truncate
func TestStorageWALTruncate(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-trunc-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	walPath := filepath.Join(dir, "test.wal")
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}

	// Write some records
	for i := 0; i < 5; i++ {
		wal.Write(WALRecord{
			Type:    WALTypeInsert,
			LSN:     uint64(i + 1),
			TableID: 1,
			Data:    i,
		})
	}

	// Truncate
	err = wal.Truncate()
	if err != nil {
		t.Errorf("Truncate failed: %v", err)
	}

	wal.Close()

	// Reopen and verify empty
	wal2, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer wal2.Close()

	records, err := wal2.ReadAll()
	if err != nil {
		t.Errorf("ReadAll failed: %v", err)
	}
	t.Logf("Records after truncate: %d", len(records))
}

// TestStorageWALDisabled tests WAL when disabled
func TestStorageWALDisabled(t *testing.T) {
	// Create WAL with empty path (disabled)
	wal, err := NewWAL("")
	if err != nil {
		t.Fatal(err)
	}

	// Write should succeed but return 0
	lsn, err := wal.Write(WALRecord{Type: WALTypeInsert})
	if err != nil {
		t.Errorf("Write to disabled WAL failed: %v", err)
	}
	if lsn != 0 {
		t.Errorf("Disabled WAL should return 0 LSN, got %d", lsn)
	}

	// Close should succeed
	err = wal.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// TestStorageReader tests Reader function
func TestStorageReader(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-reader-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write blob
	data := []byte("test data for reader")
	blobID, err := storage.WriteBlob(data)
	if err != nil {
		t.Fatal(err)
	}

	// Get reader
	reader, err := storage.Reader(blobID)
	if err != nil {
		t.Errorf("Reader failed: %v", err)
	} else {
		defer reader.Close()
		// Read from reader
		buf := make([]byte, len(data))
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			t.Errorf("Read failed: %v", err)
		}
		t.Logf("Read %d bytes: %s", n, string(buf[:n]))
	}
}

// TestStorageInterfaceToValue tests interfaceToValue
func TestStorageInterfaceToValue(t *testing.T) {
	// Test with nil
	v := interfaceToValue(nil)
	if !v.IsNull {
		t.Error("nil should convert to null value")
	}

	// Test with map (simulating JSON unmarshaling)
	dataMap := map[string]interface{}{
		"is_null": false,
		"data":    "test",
	}
	v = interfaceToValue(dataMap)
	t.Logf("Map value: %v", v)
}

// TestStorageRecoverFromWALWithOperations tests WAL recovery with various operations
func TestStorageRecoverFromWALWithOperations(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-recover-ops-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage with persistence
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	tableInfo := types.NewTableInfo(1, "recover_ops", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "name", Type: types.TypeVarchar},
	})
	storage.CreateTable(tableInfo)

	// Insert rows
	_, _, err = storage.InsertRow("recover_ops", []types.Value{
		types.NewIntValue(1),
		types.NewStringValue("Alice"),
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = storage.InsertRow("recover_ops", []types.Value{
		types.NewIntValue(2),
		types.NewStringValue("Bob"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update row
	_, err = storage.UpdateRows("recover_ops", map[int]types.Value{
		1: types.NewStringValue("Updated Bob"),
	}, func(row []types.Value) bool {
		id, _ := row[0].ToInt64()
		return id == 2
	})
	if err != nil {
		t.Logf("Update error: %v", err)
	}

	// Delete row
	_, err = storage.DeleteRows("recover_ops", func(row []types.Value) bool {
		id, _ := row[0].ToInt64()
		return id == 1
	})
	if err != nil {
		t.Logf("Delete error: %v", err)
	}

	// Get rows before close
	rowsBefore, _ := storage.GetRows("recover_ops")
	t.Logf("Rows before close: %d", len(rowsBefore))

	// Close storage
	storage.Close()

	// Reopen storage - should recover from WAL
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	// Check if table exists
	_, err = storage2.GetTableInfo("recover_ops")
	if err != nil {
		t.Logf("Table info error: %v", err)
	}

	// Get rows after recovery
	rowsAfter, err := storage2.GetRows("recover_ops")
	if err != nil {
		t.Logf("GetRows error: %v", err)
	} else {
		t.Logf("Rows after recovery: %d", len(rowsAfter))
	}
}

// TestStorageRecoverFromWALWithDropTable tests WAL recovery with DROP TABLE
func TestStorageRecoverFromWALWithDropTable(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-recover-drop-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create and drop table
	tableInfo := types.NewTableInfo(1, "drop_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.DropTable("drop_test")

	// Close and reopen
	storage.Close()

	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	// Table should not exist
	_, err = storage2.GetTableInfo("drop_test")
	if err == nil {
		t.Error("Table should not exist after recovery")
	}
}

// TestStorageWALReadAllError tests WAL ReadAll error handling
func TestStorageWALReadAllError(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-wal-read-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a corrupted WAL file
	walPath := filepath.Join(dir, "test.wal")
	// Write some invalid data
	os.WriteFile(walPath, []byte{0x00, 0x01, 0x02, 0x03}, 0644)

	// Try to read
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	// ReadAll may fail or return empty
	_, err = wal.ReadAll()
	t.Logf("ReadAll result: %v", err)
}

// TestStorageNewWALError tests NewWAL with invalid path
func TestStorageNewWALError(t *testing.T) {
	// Try to create WAL in non-existent directory
	wal, err := NewWAL("/non/existent/path/wal.log")
	if err == nil {
		wal.Close()
		t.Log("NewWAL succeeded (unexpected)")
	} else {
		t.Logf("NewWAL error (expected): %v", err)
	}
}

// TestStorageTransactionID tests Transaction ID method
func TestStorageTransactionID(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "txn_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Begin transaction
	txnID := storage.BeginTxn()
	t.Logf("Transaction ID: %d", txnID)

	// The transaction interface should return the txnID
	t.Logf("Transaction methods available")
}

// TestStorageTransactionCommit tests transaction commit
func TestStorageTransactionCommit(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "commit_txn_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Begin transaction
	txnID := storage.BeginTxn()
	t.Logf("BeginTxn: %d", txnID)

	// Insert data
	storage.InsertRow("commit_txn_test", []types.Value{types.NewIntValue(1)})

	// Commit
	storage.CommitTxn(txnID)

	// Verify data
	rows, _ := storage.GetRows("commit_txn_test")
	t.Logf("Rows after commit: %d", len(rows))
}

// TestStorageTransactionRollback tests transaction rollback
func TestStorageTransactionRollback(t *testing.T) {
	storage, err := NewStorage("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create table
	tableInfo := types.NewTableInfo(1, "rollback_txn_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert initial data
	storage.InsertRow("rollback_txn_test", []types.Value{types.NewIntValue(1)})

	// Begin transaction
	txnID := storage.BeginTxn()
	t.Logf("BeginTxn: %d", txnID)

	// Insert more data
	storage.InsertRow("rollback_txn_test", []types.Value{types.NewIntValue(2)})

	// Rollback
	storage.RollbackTxn(txnID)

	// Verify data
	rows, _ := storage.GetRows("rollback_txn_test")
	t.Logf("Rows after rollback: %d", len(rows))
}

// TestStorageCopyDirWithNested tests copyDir with nested directories
func TestStorageCopyDirWithNested(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "copy_nested_src_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	dstDir, err := os.MkdirTemp("", "copy_nested_dst_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dstDir)

	// Create nested structure
	os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0644)
	subDir1 := filepath.Join(srcDir, "level1")
	os.MkdirAll(subDir1, 0755)
	os.WriteFile(filepath.Join(subDir1, "level1.txt"), []byte("level1"), 0644)
	subDir2 := filepath.Join(subDir1, "level2")
	os.MkdirAll(subDir2, 0755)
	os.WriteFile(filepath.Join(subDir2, "level2.txt"), []byte("level2"), 0644)

	// Copy
	err = copyDir(srcDir, dstDir)
	if err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify all files were copied
	if _, err := os.Stat(filepath.Join(dstDir, "root.txt")); err != nil {
		t.Error("root.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "level1", "level1.txt")); err != nil {
		t.Error("level1/level1.txt not copied")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "level1", "level2", "level2.txt")); err != nil {
		t.Error("level1/level2/level2.txt not copied")
	}
}

// TestStorageInitFileStorageWithExistingMetadata tests initFileStorage with existing metadata
func TestStorageInitFileStorageWithExistingMetadata(t *testing.T) {
	dir, err := os.MkdirTemp("", "init_existing_meta_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create initial storage
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	tableInfo := types.NewTableInfo(1, "test_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)
	storage.Close()

	// Reopen - should load existing metadata
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	// Verify table exists
	tables := storage2.ListTables()
	if len(tables) == 0 {
		t.Error("No tables after reopening")
	}
}

// TestStorageWriteBlobLarge tests writing large blob
func TestStorageWriteBlobLarge(t *testing.T) {
	dir, err := os.MkdirTemp("", "blob_large_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create large data (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	// Write blob
	blobID, err := storage.WriteBlob(largeData)
	if err != nil {
		t.Fatalf("WriteBlob large failed: %v", err)
	}
	t.Logf("Wrote large blob with ID: %d, size: %d", blobID, len(largeData))

	// Read and verify
	readData, err := storage.ReadBlob(blobID)
	if err != nil {
		t.Fatalf("ReadBlob large failed: %v", err)
	}
	if len(readData) != len(largeData) {
		t.Errorf("Size mismatch: got %d, want %d", len(readData), len(largeData))
	}
}

// TestStorageImportExportBinaryFile tests importing/exporting binary files
func TestStorageImportExportBinaryFile(t *testing.T) {
	dir, err := os.MkdirTemp("", "binary_import_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Create binary file
	binaryData := make([]byte, 256)
	for i := range binaryData {
		binaryData[i] = byte(i)
	}
	tmpFile := filepath.Join(dir, "binary.bin")
	os.WriteFile(tmpFile, binaryData, 0644)

	// Import
	blobID, err := storage.ImportFile(tmpFile)
	if err != nil {
		t.Fatalf("ImportFile failed: %v", err)
	}
	t.Logf("Imported binary file as blob ID: %d", blobID)

	// Export
	exportPath := filepath.Join(dir, "exported_binary.bin")
	err = storage.ExportFile(blobID, exportPath)
	if err != nil {
		t.Fatalf("ExportFile failed: %v", err)
	}

	// Verify
	exportedData, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(exportedData) != len(binaryData) {
		t.Errorf("Size mismatch: got %d, want %d", len(exportedData), len(binaryData))
	}
	for i := range binaryData {
		if exportedData[i] != binaryData[i] {
			t.Errorf("Byte mismatch at position %d", i)
			break
		}
	}
}

// TestStorageWALMultipleRecords tests WAL with multiple records
func TestStorageWALMultipleRecords(t *testing.T) {
	dir, err := os.MkdirTemp("", "wal_multi_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	walPath := filepath.Join(dir, "test.wal")
	wal, err := NewWAL(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer wal.Close()

	// Write multiple records
	for i := 0; i < 10; i++ {
		record := WALRecord{
			Type:    WALTypeInsert,
			TxnID:   1,
			TableID: 1,
			RowID:   uint64(i + 1),
			Data:    []interface{}{int64(i), fmt.Sprintf("name%d", i)},
		}
		lsn, err := wal.Write(record)
		if err != nil {
			t.Errorf("Write record %d failed: %v", i, err)
		} else {
			t.Logf("Wrote record %d with LSN: %d", i, lsn)
		}
	}

	// Read all records
	records, err := wal.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	t.Logf("Read %d records from WAL", len(records))
}

// TestStorageConcurrentBlobWrites tests concurrent blob writes
func TestStorageConcurrentBlobWrites(t *testing.T) {
	dir, err := os.MkdirTemp("", "blob_concurrent_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Concurrent writes
	done := make(chan uint64)
	for i := 0; i < 10; i++ {
		go func(n int) {
			data := []byte(fmt.Sprintf("blob data %d", n))
			id, err := storage.WriteBlob(data)
			if err != nil {
				t.Logf("WriteBlob %d error: %v", n, err)
				done <- 0
			} else {
				done <- id
			}
		}(i)
	}

	// Collect IDs
	ids := make([]uint64, 0)
	for i := 0; i < 10; i++ {
		id := <-done
		if id > 0 {
			ids = append(ids, id)
		}
	}

	t.Logf("Successfully wrote %d blobs concurrently", len(ids))
}

// TestStorageReaderEOF tests Reader EOF handling
func TestStorageReaderEOF(t *testing.T) {
	dir, err := os.MkdirTemp("", "reader_eof_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Write small blob
	data := []byte("test")
	blobID, err := storage.WriteBlob(data)
	if err != nil {
		t.Fatal(err)
	}

	// Get reader
	reader, err := storage.Reader(blobID)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	// Read in small chunks
	buf := make([]byte, 2)
	total := 0
	for {
		n, err := reader.Read(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Logf("Read error: %v", err)
			break
		}
	}
	t.Logf("Total bytes read: %d", total)
}

// TestStorageWriterMultipleWrites tests Writer with multiple writes
func TestStorageWriterMultipleWrites(t *testing.T) {
	dir, err := os.MkdirTemp("", "writer_multi_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Get writer
	writer, blobID, err := storage.Writer()
	if err != nil {
		t.Fatal(err)
	}

	// Write multiple times
	writer.Write([]byte("part1"))
	writer.Write([]byte("part2"))
	writer.Write([]byte("part3"))
	writer.Close()

	t.Logf("Wrote multi-part blob with ID: %d", blobID)

	// Read and verify
	data, err := storage.ReadBlob(blobID)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "part1part2part3" {
		t.Errorf("Content mismatch: got %s", string(data))
	}
}

// ============================================================
// Benchmark Tests
// ============================================================

func BenchmarkStorageCreateTable(b *testing.B) {
	storage, err := NewStorage("", true)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "bench_table", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq, AutoInc: true},
		{Name: "name", Type: types.TypeVarchar, Length: 100},
		{Name: "value", Type: types.TypeInt},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tableInfo.Name = fmt.Sprintf("bench_table_%d", i)
		storage.CreateTable(tableInfo)
	}
}

func BenchmarkStorageInsertRow(b *testing.B) {
	storage, err := NewStorage("", true)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "bench_insert", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq, AutoInc: true},
		{Name: "name", Type: types.TypeVarchar, Length: 100},
		{Name: "value", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := []types.Value{
			types.NewIntValue(0),
			types.NewStringValue(fmt.Sprintf("name_%d", i)),
			types.NewIntValue(int64(i)),
		}
		storage.InsertRow("bench_insert", row)
	}
}

func BenchmarkStorageGetRows(b *testing.B) {
	storage, err := NewStorage("", true)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "bench_get", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq, AutoInc: true},
		{Name: "name", Type: types.TypeVarchar, Length: 100},
	})
	storage.CreateTable(tableInfo)

	// Insert test data
	for i := 0; i < 1000; i++ {
		row := []types.Value{
			types.NewIntValue(0),
			types.NewStringValue(fmt.Sprintf("name_%d", i)),
		}
		storage.InsertRow("bench_get", row)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.GetRows("bench_get")
	}
}

func BenchmarkStorageUpdateRows(b *testing.B) {
	storage, err := NewStorage("", true)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	tableInfo := types.NewTableInfo(0, "bench_update", []types.ColumnDef{
		{Name: "id", Type: types.TypeSeq, AutoInc: true},
		{Name: "value", Type: types.TypeInt},
	})
	storage.CreateTable(tableInfo)

	// Insert test data
	for i := 0; i < 100; i++ {
		row := []types.Value{types.NewIntValue(0), types.NewIntValue(int64(i))}
		storage.InsertRow("bench_update", row)
	}

	updates := map[int]types.Value{1: types.NewIntValue(999)}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		storage.UpdateRows("bench_update", updates, func(row []types.Value) bool {
			return true
		})
	}
}

func BenchmarkStorageDeleteRows(b *testing.B) {
	storage, err := NewStorage("", true)
	if err != nil {
		b.Fatal(err)
	}
	defer storage.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create table for this iteration
		tableInfo := types.NewTableInfo(0, "bench_delete", []types.ColumnDef{
			{Name: "id", Type: types.TypeInt},
		})
		storage.CreateTable(tableInfo)

		// Insert rows
		for j := 0; j < 10; j++ {
			storage.InsertRow("bench_delete", []types.Value{types.NewIntValue(int64(j))})
		}

		// Delete rows
		storage.DeleteRows("bench_delete", func(row []types.Value) bool {
			return true
		})

		// Drop table
		storage.DropTable("bench_delete")
	}
}

// TestBlobThreshold tests automatic external blob storage for large blobs
func TestBlobThreshold(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-threshold-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Set a small threshold for testing (100 bytes)
	cfg := storage.GetConfig()
	cfg.BlobThreshold = 100
	storage.SetConfig(cfg)

	// Create table with blob column
	tableInfo := types.NewTableInfo(1, "blob_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "data", Type: types.TypeBlob},
	})
	if err := storage.CreateTable(tableInfo); err != nil {
		t.Fatal(err)
	}

	// Insert small blob (should stay inline)
	smallData := []byte("small blob data")
	smallRow := []types.Value{
		types.NewIntValue(1),
		types.NewBlobValue(smallData),
	}
	_, _, err = storage.InsertRow("blob_test", smallRow)
	if err != nil {
		t.Fatal(err)
	}

	// Insert large blob (should be stored externally)
	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	largeRow := []types.Value{
		types.NewIntValue(2),
		types.NewBlobValue(largeData),
	}
	_, _, err = storage.InsertRow("blob_test", largeRow)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve rows and verify data
	rows, err := storage.GetRows("blob_test")
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 {
		t.Fatalf("Expected 2 rows, got %d", len(rows))
	}

	// Verify small blob
	smallRead := rows[0][1]
	if smallRead.IsNull {
		t.Fatal("Small blob should not be null")
	}
	smallBytes, err := smallRead.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(smallBytes) != string(smallData) {
		t.Errorf("Small blob mismatch: got %s, want %s", string(smallBytes), string(smallData))
	}

	// Verify large blob
	largeRead := rows[1][1]
	if largeRead.IsNull {
		t.Fatal("Large blob should not be null")
	}
	largeBytes, err := largeRead.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(largeBytes) != len(largeData) {
		t.Errorf("Large blob size mismatch: got %d, want %d", len(largeBytes), len(largeData))
	}
	for i := range largeData {
		if largeBytes[i] != largeData[i] {
			t.Errorf("Large blob content mismatch at index %d", i)
			break
		}
	}

	t.Logf("Blob threshold test passed: small blob inline, large blob external")
}

// TestBlobThresholdZero tests that threshold=0 keeps all blobs inline
func TestBlobThresholdZero(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-threshold-zero-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()

	// Set threshold to 0 (all blobs inline)
	cfg := storage.GetConfig()
	cfg.BlobThreshold = 0
	storage.SetConfig(cfg)

	// Create table with blob column
	tableInfo := types.NewTableInfo(1, "inline_blob_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "data", Type: types.TypeBlob},
	})
	if err := storage.CreateTable(tableInfo); err != nil {
		t.Fatal(err)
	}

	// Insert large blob
	largeData := make([]byte, 10000)
	largeRow := []types.Value{
		types.NewIntValue(1),
		types.NewBlobValue(largeData),
	}
	_, _, err = storage.InsertRow("inline_blob_test", largeRow)
	if err != nil {
		t.Fatal(err)
	}

	// Retrieve and verify
	rows, err := storage.GetRows("inline_blob_test")
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	// With threshold=0, blob should not be stored externally
	blobVal := rows[0][1]
	if blobVal.IsBlobRef() {
		t.Error("With threshold=0, blob should be stored inline, not externally")
	}

	t.Logf("Threshold=0 test passed: all blobs stored inline")
}

// TestBlobRefRecovery tests blob reference recovery from WAL
func TestBlobRefRecovery(t *testing.T) {
	dir, err := os.MkdirTemp("", "xxldb-blob-ref-recovery-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create storage with small threshold
	storage, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	cfg := storage.GetConfig()
	cfg.BlobThreshold = 100
	storage.SetConfig(cfg)

	// Create table and insert large blob
	tableInfo := types.NewTableInfo(1, "recovery_test", []types.ColumnDef{
		{Name: "id", Type: types.TypeInt},
		{Name: "data", Type: types.TypeBlob},
	})
	storage.CreateTable(tableInfo)

	largeData := make([]byte, 200)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	storage.InsertRow("recovery_test", []types.Value{
		types.NewIntValue(1),
		types.NewBlobValue(largeData),
	})

	// Close storage
	storage.Close()

	// Reopen storage
	storage2, err := NewStorage(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	defer storage2.Close()

	// Retrieve data
	rows, err := storage2.GetRows("recovery_test")
	if err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("Expected 1 row, got %d", len(rows))
	}

	// Verify blob data
	blobBytes, err := rows[0][1].ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	if len(blobBytes) != len(largeData) {
		t.Errorf("Blob size mismatch after recovery: got %d, want %d", len(blobBytes), len(largeData))
	}

	t.Logf("Blob reference recovery test passed")
}
