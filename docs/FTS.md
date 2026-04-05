# Full-Text Search (FTS) Documentation

[中文版](FTS_CN.md)

## Overview

XxLdb provides full-text search capabilities using an inverted index with TF-IDF scoring. This allows efficient searching of text content in your database tables.

## Features

- **Inverted Index**: Fast text search using inverted index structure
- **TF-IDF Scoring**: Results ranked by relevance using TF-IDF algorithm
- **AND Search**: Multiple search terms combined with AND logic
- **Unicode Support**: Full support for Chinese, Japanese, and other Unicode text
- **Automatic Index Maintenance**: Indexes updated automatically on INSERT/UPDATE/DELETE
- **Multiple Indexes**: Support for multiple full-text indexes on different columns

## Creating Full-Text Index

### SQL Syntax

```sql
CREATE FULLTEXT INDEX index_name ON table_name(column_name);
```

### Examples

```sql
-- Create table
CREATE TABLE articles (
    id SEQ,
    title VARCHAR(200),
    content TEXT,
    created_at DATETIME
);

-- Create full-text index on single column
CREATE FULLTEXT INDEX idx_content ON articles(content);

-- Create multiple full-text indexes
CREATE FULLTEXT INDEX idx_title ON articles(title);
CREATE FULLTEXT INDEX idx_body ON articles(body);
```

## Searching

### Basic Search

```sql
-- Search for documents containing "database"
SELECT * FROM articles 
WHERE MATCH(content) AGAINST('database');
```

### Multi-term Search

Multiple search terms are combined with AND logic - all terms must match:

```sql
-- Search for documents containing both "database" and "programming"
SELECT * FROM articles 
WHERE MATCH(content) AGAINST('database programming');
```

### With ORDER BY

```sql
SELECT id, title, content 
FROM articles 
WHERE MATCH(content) AGAINST('search terms') 
ORDER BY id DESC
LIMIT 10;
```

## Index Maintenance

Full-text indexes are automatically maintained when data changes:

### INSERT

```sql
INSERT INTO articles (title, content) 
VALUES ('Introduction', 'This article is about databases');
-- Content is automatically indexed
```

### UPDATE

```sql
UPDATE articles 
SET content = 'Updated content about programming' 
WHERE id = 1;
-- Index is automatically updated
```

### DELETE

```sql
DELETE FROM articles WHERE id = 1;
-- Document is automatically removed from index
```

## Advanced Usage

### Indexing Existing Data

When you create a full-text index on a table that already contains data, the existing rows are automatically indexed:

```sql
-- Insert data first
INSERT INTO articles (title, content) VALUES ('Article 1', 'Content 1');
INSERT INTO articles (title, content) VALUES ('Article 2', 'Content 2');

-- Then create index - existing data is indexed automatically
CREATE FULLTEXT INDEX idx_content ON articles(content);
```

### Multiple Columns

You can create separate full-text indexes on multiple columns:

```sql
CREATE TABLE documents (
    id SEQ,
    title VARCHAR(200),
    body TEXT,
    keywords VARCHAR(500)
);

CREATE FULLTEXT INDEX idx_title ON documents(title);
CREATE FULLTEXT INDEX idx_body ON documents(body);
CREATE FULLTEXT INDEX idx_keywords ON documents(keywords);

-- Search in specific column
SELECT * FROM documents WHERE MATCH(title) AGAINST('important');
SELECT * FROM documents WHERE MATCH(body) AGAINST('database');
```

## API Reference

### Go API

```go
package main

import (
    "fmt"
    "github.com/topxeq/xxldb"
)

func main() {
    // Open database
    engine, err := xxldb.Open("/path/to/database")
    if err != nil {
        panic(err)
    }
    defer engine.Close()
    
    // Create full-text index
    _, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON mytable(content)")
    if err != nil {
        panic(err)
    }
    
    // Check if index exists
    hasIndex := engine.FTS().HasIndex("mytable", "content")
    fmt.Printf("Index exists: %v\n", hasIndex)
    
    // Get index statistics
    indexer := engine.FTS().GetIndex("mytable", "content")
    if indexer != nil {
        stats := indexer.Stats()
        fmt.Printf("Documents: %d\n", stats.DocumentCount)
        fmt.Printf("Terms: %d\n", stats.TermCount)
    }
    
    // Perform search
    result, err := engine.Execute(
        "SELECT * FROM mytable WHERE MATCH(content) AGAINST('search term')",
    )
    if err != nil {
        panic(err)
    }
    
    for _, row := range result.Rows {
        fmt.Println(row)
    }
}
```

## Implementation Details

### Tokenization

- Text is converted to lowercase
- Split by whitespace and punctuation
- Unicode characters are supported (treated as word characters)
- Each unique term is indexed

### Inverted Index Structure

```
Term        -> [DocID1, DocID2, DocID3, ...]
----------------------------------------
"database"  -> [1, 5, 8, 12]
"search"    -> [1, 3, 5]
"full"      -> [2, 4, 8]
```

### TF-IDF Scoring

Results are scored using simplified TF-IDF:

```
score = tf * idf

where:
  tf  = term frequency in document
  idf = log(total_docs / docs_containing_term)
```

Higher scores indicate more relevant results.

## Limitations

1. **No Phrase Search**: Cannot search for exact phrases like `"full text"`
2. **No OR Search**: All terms use AND logic
3. **No Boolean Operators**: Cannot use `+`, `-`, `AND`, `OR` operators
4. **No Wildcards**: No support for `*` or `?` wildcards
5. **Simple Tokenization**: Chinese/Japanese text is not word-segmented

## Performance Tips

1. **Create indexes after bulk inserts**: Creating the index after inserting data is faster than inserting into an already-indexed table
2. **Limit result sets**: Use `LIMIT` to restrict the number of results
3. **Index only necessary columns**: Don't index columns that don't need full-text search

## Testing

```bash
# Run FTS tests
go test ./fts/... -v

# Run FTS integration tests
go test ./executor/... -v -run TestFTS

# Run with coverage
go test ./fts/... -cover
```

## Example Application

```go
package main

import (
    "fmt"
    "log"
    "github.com/topxeq/xxldb"
)

func main() {
    // Create in-memory database
    engine, err := xxldb.NewEngine("", true)
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close()
    
    // Create table
    _, err = engine.Execute(`
        CREATE TABLE wiki (
            id SEQ,
            title VARCHAR(200),
            content TEXT
        )
    `)
    if err != nil {
        log.Fatal(err)
    }
    
    // Create full-text index
    _, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON wiki(content)")
    if err != nil {
        log.Fatal(err)
    }
    
    // Insert documents
    documents := []struct {
        title, content string
    }{
        {"Go Programming", "Go is a modern programming language"},
        {"Python Tutorial", "Python is popular for beginners"},
        {"Database Design", "Database design is crucial for applications"},
    }
    
    for _, doc := range documents {
        sql := fmt.Sprintf(
            "INSERT INTO wiki (title, content) VALUES ('%s', '%s')",
            doc.title, doc.content,
        )
        _, err = engine.Execute(sql)
        if err != nil {
            log.Fatal(err)
        }
    }
    
    // Search
    result, err := engine.Execute(
        "SELECT * FROM wiki WHERE MATCH(content) AGAINST('programming')",
    )
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("Search results:")
    for _, row := range result.Rows {
        fmt.Printf("  %s: %s\n", row.Data[1], row.Data[2])
    }
}
```
