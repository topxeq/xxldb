package storage

import (
	"os"
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
