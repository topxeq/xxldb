package storage

import (
	"os"
	"path/filepath"
	"testing"

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
