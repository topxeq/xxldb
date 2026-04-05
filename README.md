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
- **Full-Text Search** - Full-text indexing and MATCH...AGAINST search
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

## Full-Text Search

XxLdb provides full-text search capabilities using inverted indexes with TF-IDF scoring.

### Creating Full-Text Index

```sql
-- Create table with text content
CREATE TABLE articles (
    id SEQ,
    title VARCHAR(200),
    content TEXT
);

-- Create full-text index on content column
CREATE FULLTEXT INDEX idx_content ON articles(content);

-- Create multiple full-text indexes
CREATE FULLTEXT INDEX idx_title ON articles(title);
CREATE FULLTEXT INDEX idx_body ON articles(body);
```

### Full-Text Search Query

Use `MATCH...AGAINST` syntax for full-text search:

```sql
-- Search for documents containing 'database'
SELECT * FROM articles WHERE MATCH(content) AGAINST('database');

-- Search for multiple terms (AND search - all terms must match)
SELECT * FROM articles WHERE MATCH(content) AGAINST('database programming');

-- Search with ORDER BY for relevance ranking
SELECT id, title FROM articles 
WHERE MATCH(content) AGAINST('full-text search') 
ORDER BY id DESC;
```

### How It Works

- **Inverted Index**: Text is tokenized and indexed using an inverted index structure
- **TF-IDF Scoring**: Results are ranked using simplified TF-IDF scoring
- **AND Search**: Multiple search terms are combined with AND logic
- **Unicode Support**: Full support for Unicode text including Chinese, Japanese, etc.

### Automatic Index Maintenance

Full-text indexes are automatically updated when data changes:

```sql
-- Insert - automatically indexed
INSERT INTO articles (title, content) VALUES ('Introduction', 'This is about databases');

-- Update - index is updated
UPDATE articles SET content = 'Updated content about programming' WHERE id = 1;

-- Delete - removed from index
DELETE FROM articles WHERE id = 1;
```

### FTS API

```go
// Create full-text index programmatically
engine, _ := xxldb.Open("/path/to/db")
engine.Execute("CREATE FULLTEXT INDEX idx_content ON mytable(content)")

// Check if index exists
hasIndex := engine.FTS().HasIndex("mytable", "content")

// Get index statistics
indexer := engine.FTS().GetIndex("mytable", "content")
stats := indexer.Stats()
fmt.Printf("Documents: %d, Terms: %d\n", stats.DocumentCount, stats.TermCount)
```

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
- `IMAGE_FROM_HEX(str)` - Create image from hex string (supports 0x prefix)
- `IMAGE_TO_BASE64(img)` - Convert image to BASE64
- `IMAGE_TO_BASE64(img, 'datauri')` - Convert to Data URI format
- `IMAGE_TO_HEX(img)` - Convert image to hex string
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

-- Load from hex string
INSERT INTO photos (name, img) VALUES ('icon', IMAGE_FROM_HEX('89504e470d0a...'));

-- Load from Data URI
INSERT INTO photos (name, img) VALUES ('logo', IMAGE_FROM_BASE64('data:image/png;base64,iVBORw0KGgo...'));

-- Query image info
SELECT name, IMAGE_WIDTH(img), IMAGE_HEIGHT(img), IMAGE_FORMAT(img) FROM photos;

-- Export as Data URI (for HTML embedding)
SELECT name, IMAGE_TO_BASE64(img, 'datauri') FROM photos;

-- Export as hex string
SELECT name, IMAGE_TO_HEX(img) FROM photos;
```

### BLOB Functions
- `BLOB_FROM_BASE64(str)` - Create BLOB from BASE64 string
- `BLOB_FROM_HEX(str)` - Create BLOB from hex string (supports 0x prefix)
- `BLOB_TO_BASE64(blob)` - Convert BLOB to BASE64 string
- `BLOB_TO_HEX(blob)` - Convert BLOB to hex string

#### BLOB Example
```sql
CREATE TABLE files (id SEQ, name VARCHAR(100), data BLOB);

-- Insert from BASE64
INSERT INTO files (name, data) VALUES ('config', BLOB_FROM_BASE64('SGVsbG8gV29ybGQh'));

-- Insert from hex
INSERT INTO files (name, data) VALUES ('binary', BLOB_FROM_HEX('48656c6c6f'));

-- Insert from hex with 0x prefix
INSERT INTO files (name, data) VALUES ('binary2', BLOB_FROM_HEX('0x48656c6c6f'));

-- Export as BASE64
SELECT name, BLOB_TO_BASE64(data) FROM files;

-- Export as hex
SELECT name, BLOB_TO_HEX(data) FROM files;
```

### Embedded BLOB/IMAGE API

When using XxLdb as an embedded library, you can work with BLOB and IMAGE data directly from Go code:

#### Using the Driver

```go
import (
    "database/sql"
    "io"
    "strings"
    
    _ "github.com/topxeq/xxldb/driver"
)

// Insert BLOB from []byte
data := []byte("binary data")
db.Exec("INSERT INTO files (name, data) VALUES (?, ?)", "test", driver.NewBlob(data))

// Insert IMAGE from []byte
imgData := []byte{0x89, 0x50, 0x4e, 0x47, ...} // PNG data
db.Exec("INSERT INTO photos (name, img) VALUES (?, ?)", "photo", driver.NewImage(imgData))

// Insert from io.Reader
reader := strings.NewReader("data from reader")
blob, _ := driver.BlobFromReader(reader)
db.Exec("INSERT INTO files (name, data) VALUES (?, ?)", "from_reader", blob)

// Insert from io.Reader for IMAGE
imgReader := openImageFile() // returns io.Reader
img, _ := driver.ImageFromReader(imgReader)
db.Exec("INSERT INTO photos (name, img) VALUES (?, ?)", "photo", img)
```

#### Using the Engine Directly

```go
import "github.com/topxeq/xxldb/executor"

engine, _ := executor.NewEngine("/path/to/db", false)

// Insert BLOB directly (accepts []byte, io.Reader, or hex string)
id, _ := engine.InsertBlobDirect("files", "data", []byte("binary data"))
id, _ = engine.InsertBlobDirect("files", "data", "48656c6c6f") // hex string
id, _ = engine.InsertBlobDirect("files", "data", reader) // io.Reader

// Insert IMAGE directly
id, _ := engine.InsertImageDirect("photos", "img", imageData)

// Retrieve BLOB/IMAGE directly
data, _ := engine.GetBlobDirect("files", "data", "id = 1")
imgData, _ := engine.GetImageDirect("photos", "img", "id = 1")

// Update BLOB/IMAGE directly
engine.UpdateBlobDirect("files", "data", newData, "id = 1")
engine.UpdateImageDirect("photos", "img", newImgData, "id = 1")
```

#### Using the types Package

```go
import "github.com/topxeq/xxldb/types"
import "strings"

// Create BLOB value from []byte
blobVal := types.NewBlobValue([]byte("data"))

// Create IMAGE value from []byte
imgVal := types.NewImageValue(imageBytes)

// Create from io.Reader
blobVal, _ = types.NewBlobValueFromReader(reader)
imgVal, _ = types.NewImageValueFromReader(reader)

// Create from hex string
blobVal, _ = types.NewBlobValueFromHex("48656c6c6f")
imgVal, _ = types.NewImageValueFromHex("89504e47...")

// Convert to hex string
hexStr, _ := blobVal.ToHex()
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
├── fts/               # Full-text search
│   └── fts.go         # FTS manager and inverted index
├── script/            # Script functions
├── auth/              # Authentication module
├── logger/            # Logging module
├── driver/            # Go SQL driver
└── cmd/xxldb/         # Command line client
```

## Configuration

```go
config := xxldb.Config{
    Path:          "/path/to/db",       // Database path
    InMemory:      false,               // In-memory mode
    LogLevel:      "INFO",              // Log level
    Username:      "admin",             // Username
    Password:      "secret",            // Password
    AutoCommit:    true,                // Auto commit
    SyncInterval:  1000,                // Sync interval (ms)
    BlobThreshold: 1024 * 1024 * 1024,  // Blob size threshold (default: 1GB)
}
engine, err := xxldb.OpenWithConfig(config)
```

### Blob Storage Threshold

XxLdb automatically stores large BLOBs in separate files to reduce memory usage:

- **Default threshold**: 1GB (1073741824 bytes)
- **Blobs larger than threshold**: Stored in `blobs/` directory
- **Blobs smaller than threshold**: Stored inline in memory
- **Set threshold to 0**: Store all blobs inline

```go
// Store all blobs inline (no external files)
config.BlobThreshold = 0

// Store blobs larger than 100MB externally
config.BlobThreshold = 100 * 1024 * 1024
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
