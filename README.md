# XxLdb - Lightweight SQL Database

[中文文档](README_CN.md)

[![Go Report Card](https://goreportcard.com/badge/github.com/topxeq/xxldb)](https://goreportcard.com/report/github.com/topxeq/xxldb)
[![GoDoc](https://godoc.org/github.com/topxeq/xxldb?status.svg)](https://godoc.org/github.com/topxeq/xxldb)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A lightweight embedded SQL database implemented in pure Go.

## Features

- **Pure Go Implementation** - No CGO dependencies, cross-platform support (Linux/macOS/Windows)
- **Full SQL Support** - SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER
- **JOIN and UNION** - INNER/LEFT/RIGHT JOIN and UNION operations
- **Built-in Functions** - String, numeric, date, aggregate, and image functions
- **Script Functions** - Custom script functions with `xx_` prefix
- **File Storage** - BLOB, FILE and IMAGE types for storing files/images/folders
- **Unicode Support** - Full Unicode support for string functions
- **WAL Logging** - Write-Ahead Logging for crash recovery
- **Authentication** - Username/password authentication support
- **Configurable Logging** - DEBUG/INFO/WARN/ERROR levels
- **Standard Driver** - Implements Go standard database/sql driver interface
- **Data Import** - Import from MySQL, PostgreSQL, SQLite, Oracle, MS SQL Server

## Data Types

| Type | Description |
|------|-------------|
| SEQ | Auto-increment sequence (int64) |
| INT | Integer (int64) |
| FLOAT | Floating point (float64) |
| CHAR(n) | Fixed-length string |
| VARCHAR(n) | Variable-length string |
| TEXT | Large text |
| DATE | Date |
| TIME | Time |
| DATETIME | Date and time |
| BLOB | Binary large object |
| FILE | File reference |
| IMAGE | Image with metadata (supports PNG, JPEG, GIF, BMP, TIFF, WebP) |

## Installation

```bash
go get github.com/topxeq/xxldb
```

## Quick Start

### Basic Usage

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    _ "github.com/topxeq/xxldb/driver"
)

func main() {
    // Open database
    db, err := sql.Open("xxldb", "/path/to/database")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create table
    _, err = db.Exec(`CREATE TABLE users (
        id SEQ,
        name VARCHAR(100),
        email VARCHAR(100),
        age INT,
        created_at DATETIME
    )`)
    if err != nil {
        log.Fatal(err)
    }

    // Insert data
    result, err := db.Exec(
        "INSERT INTO users (name, email, age, created_at) VALUES (?, ?, ?, ?)",
        "John", "john@example.com", 25, "2026-01-01 10:00:00",
    )
    if err != nil {
        log.Fatal(err)
    }
    id, _ := result.LastInsertId()
    fmt.Printf("Inserted ID: %d\n", id)

    // Query data
    rows, err := db.Query("SELECT id, name, email, age FROM users WHERE age > ?", 20)
    if err != nil {
        log.Fatal(err)
    }
    defer rows.Close()

    for rows.Next() {
        var id int64
        var name, email string
        var age int
        if err := rows.Scan(&id, &name, &email, &age); err != nil {
            log.Fatal(err)
        }
        fmt.Printf("ID: %d, Name: %s, Email: %s, Age: %d\n", id, name, email, age)
    }
}
```

### In-Memory Mode

```go
db, err := sql.Open("xxldb", ":memory:")
```

### File Storage

```sql
-- Create table with file support
CREATE TABLE documents (
    id SEQ,
    name VARCHAR(255),
    content BLOB,
    created_at DATETIME
);

-- Insert file content
INSERT INTO documents (name, content, created_at)
VALUES ('report.pdf', LOAD_FILE('/path/to/report.pdf'), NOW());

-- Export file
SELECT content INTO OUTFILE '/tmp/report_copy.pdf' FROM documents WHERE id = 1;
```

### Folder Storage (Featured)

XxLdb supports storing entire folders in the database:

```sql
-- Create folder storage table
CREATE TABLE folders (
    id SEQ,
    name VARCHAR(255),
    data BLOB,
    created_at DATETIME
);

-- Load entire folder
INSERT INTO folders (name, data, created_at)
VALUES ('my_project', LOAD_FOLDER('/path/to/project'), NOW());

-- View folder contents
SELECT LIST_FOLDER(data) FROM folders WHERE name = 'my_project';

-- Count files
SELECT FOLDER_FILES(data) FROM folders WHERE name = 'my_project';

-- Export folder to specified path
SELECT EXPORT_FOLDER(data, '/tmp/restored_project') FROM folders WHERE name = 'my_project';
```

**Folder Functions:**

| Function | Description |
|----------|-------------|
| `LOAD_FOLDER(path)` | Load folder, returns BLOB data with complete structure |
| `EXPORT_FOLDER(data, path)` | Export folder data to specified path |
| `LIST_FOLDER(data)` | List folder contents (tree structure) |
| `FOLDER_FILES(data)` | Count files in folder |

**Notes:**
- Folder structure stored in JSON format within BLOB
- File size limit configurable via `MaxFileSize` in config (default: unlimited)

## Command Line Client

```bash
# Open database
xxldb -db /path/to/database

# In-memory mode
xxldb -memory

# Execute single SQL statement
xxldb -db /path/to/db -e "SELECT * FROM users"

# Set username and password
xxldb -db /path/to/db -user admin -password secret
```

### Client Commands

| Command | Description |
|---------|-------------|
| .help | Show help |
| .tables | List all tables |
| .schema <table> | Show table schema |
| .backup <path> | Backup database |
| .restore <path> | Restore database |
| .user <username> | Set username |
| .password <password> | Set password |
| .log <level> | Set log level |
| .quit | Exit program |

## Built-in Functions

### String Functions
- `CONCAT(str1, str2, ...)` - Concatenate strings
- `LENGTH(str)` - String length in characters (Unicode-aware)
- `BYTE_LENGTH(str)` / `OCTET_LENGTH(str)` - String length in bytes
- `CHAR_LENGTH(str)` / `CHARACTER_LENGTH(str)` - Alias for LENGTH
- `UPPER(str)` - Convert to uppercase
- `LOWER(str)` - Convert to lowercase
- `TRIM(str)` - Remove leading/trailing spaces
- `SUBSTRING(str, start, len)` - Substring
- `REPLACE(str, old, new)` - Replace string

### Numeric Functions
- `ABS(n)` - Absolute value
- `ROUND(n, precision)` - Round number
- `FLOOR(n)` - Floor
- `CEIL(n)` - Ceiling
- `POWER(base, exp)` - Power
- `SQRT(n)` - Square root
- `MOD(a, b)` - Modulo

### Aggregate Functions
- `COUNT(*)` - Count rows
- `SUM(col)` - Sum
- `AVG(col)` - Average
- `MIN(col)` - Minimum
- `MAX(col)` - Maximum

### Date Functions
- `NOW()` - Current datetime
- `CURRENT_DATE()` - Current date
- `YEAR(date)` - Year
- `MONTH(date)` - Month
- `DAY(date)` - Day
- `DATEDIFF(d1, d2)` - Date difference
- `DATE_ADD(date, days)` - Add days to date

### Conversion Functions
- `CAST(val AS type)` - Type conversion
- `COALESCE(val1, val2, ...)` - Return first non-null value
- `IFNULL(val, default)` - Null replacement

### Image Functions
- `LOAD_IMAGE(path)` - Load image from file
- `IMAGE_FROM_BASE64(str)` - Create image from BASE64 string
- `IMAGE_TO_BASE64(img)` - Convert image to BASE64
- `IMAGE_TO_BASE64(img, 'datauri')` - Convert to Data URI format
- `IMAGE_WIDTH(img)` - Get image width
- `IMAGE_HEIGHT(img)` - Get image height
- `IMAGE_FORMAT(img)` - Get image format (png/jpeg/gif/...)
- `IMAGE_SIZE(img)` - Get image size in bytes
- `IMAGE_MIME(img)` - Get MIME type

#### Image Example
```sql
CREATE TABLE photos (id SEQ, name VARCHAR(100), img IMAGE);

-- Load from file
INSERT INTO photos (name, img) VALUES ('sunset', LOAD_IMAGE('/path/to/sunset.jpg'));

-- Load from BASE64
INSERT INTO photos (name, img) VALUES ('avatar', IMAGE_FROM_BASE64('iVBORw0KGgo...'));

-- Load from Data URI
INSERT INTO photos (name, img) VALUES ('logo', IMAGE_FROM_BASE64('data:image/png;base64,iVBORw0KGgo...'));

-- Query image info
SELECT name, IMAGE_WIDTH(img), IMAGE_HEIGHT(img), IMAGE_FORMAT(img) FROM photos;

-- Export as Data URI (for HTML embedding)
SELECT name, IMAGE_TO_BASE64(img, 'datauri') FROM photos;
```

## Script Functions

Script functions use `xx_` prefix and are stored in the `xxscript` system table:

```sql
-- Create script function
INSERT INTO xxscript (name, script, description)
VALUES ('xx_discount', '$1 * 0.9', 'Calculate discounted price');

-- Use script function
SELECT name, xx_discount(price) AS discount_price FROM products;
```

## Project Structure

```
xxldb/
├── xxldb.go           # Main entry
├── types/             # Type definitions
├── storage/           # Storage engine
│   ├── storage.go     # Storage management
│   ├── page.go        # Page management
│   └── wal.go         # WAL logging
├── parser/            # SQL parser
│   ├── lexer.go       # Lexer
│   ├── parser.go      # Parser
│   └── ast.go         # AST definitions
├── executor/          # Query executor
├── function/          # Built-in functions
├── script/            # Script functions
├── auth/              # Authentication module
├── logger/            # Logging module
├── driver/            # Go SQL driver
└── cmd/xxldb/         # Command line client
```

## Configuration

```go
config := xxldb.Config{
    Path:         "/path/to/db",    // Database path
    InMemory:     false,            // In-memory mode
    LogLevel:     "INFO",           // Log level
    Username:     "admin",          // Username
    Password:     "secret",         // Password
    AutoCommit:   true,             // Auto commit
    SyncInterval: 1000,             // Sync interval (ms)
}
engine, err := xxldb.OpenWithConfig(config)
```

## Backup and Restore

### Backup

```bash
# In client
xxldb> .backup /path/to/backup

# Or using SQL
BACKUP TO '/path/to/backup';
```

### Restore

```bash
# In client
xxldb> .restore /path/to/backup

# Or using SQL
RESTORE FROM '/path/to/backup';
```

## Data Import

XxLdb supports importing data from other databases including MySQL, PostgreSQL, SQLite, Oracle, and MS SQL Server.

### Command Line Import

```bash
# Import single table from MySQL
xxldb -db my.db -import "mysql://user:pass@localhost/dbname" -table users

# Import all tables from PostgreSQL
xxldb -db my.db -import "postgresql://user:pass@localhost/dbname" -import-all

# Import from SQLite
xxldb -db my.db -import "sqlite:///path/to/source.db" -import-all

# Import from Oracle
xxldb -db my.db -import "oracle://user:pass@host:1521/sid" -table employees -to staff

# Import from MS SQL Server
xxldb -db my.db -import "mssql://user:pass@host:1433/dbname" -import-all
```

### REPL Import

```sql
-- Import single table
xxldb> .import mysql://user:pass@localhost/dbname users

-- Import with different target table name
xxldb> .import postgresql://user:pass@localhost/dbname old_table new_table

-- Import all tables
xxldb> .import-all sqlite:///path/to/source.db
```

### Import Options

| Option | Description |
|--------|-------------|
| `-import <dsn>` | Source database connection string |
| `-table <name>` | Source table to import |
| `-to <name>` | Target table name (default: same as source) |
| `-import-all` | Import all tables from source |
| `-batch <size>` | Batch size for import (default: 1000) |
| `-overwrite` | Overwrite existing tables |

### Supported Constraints

XxLdb imports the following constraints:

| Constraint | MySQL | PostgreSQL | SQLite | Oracle | MSSQL |
|------------|-------|------------|--------|--------|-------|
| PRIMARY KEY | ✅ | ✅ | ✅ | ✅ | ✅ |
| FOREIGN KEY | ✅ | ✅ | ✅ | ✅ | ✅ |
| UNIQUE | ✅ | ✅ | ✅ | ✅ | ✅ |
| CHECK | ✅ (8.0+) | ✅ | ✅ | ✅ | ✅ |
| INDEX | ✅ | ✅ | ✅ | ✅ | ✅ |

### Type Mapping

Import automatically maps source database types to XxLdb types:

| Source Type | XxLdb Type |
|-------------|------------|
| INT, INTEGER, BIGINT | INT |
| FLOAT, DOUBLE, DECIMAL | FLOAT |
| CHAR, NCHAR | CHAR |
| VARCHAR, NVARCHAR | VARCHAR |
| TEXT, CLOB | TEXT |
| DATE | DATE |
| TIME | TIME |
| DATETIME, TIMESTAMP | DATETIME |
| BLOB, BINARY | BLOB |

## Performance

- Single table query: < 1ms (under 1000 rows)
- Insert operations: > 10000 ops/sec
- Concurrent reads: > 50000 ops/sec
- Startup time: < 100ms
- Memory usage: < 50MB (empty database)

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run benchmark tests
go test -bench=. ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see [LICENSE](LICENSE) for details.

## Author

topxeq
