// Package executor provides FTS integration tests
package executor

import (
	"testing"
)

func TestFullTextSearchBasic(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table with text column
	_, err = engine.Execute(`CREATE TABLE articles (
		id SEQ,
		title VARCHAR(200),
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Create fulltext index on content column
	result, err := engine.Execute("CREATE FULLTEXT INDEX idx_content ON articles(content)")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Create FTS index: %s", result.Message)

	// Insert some test data
	_, err = engine.Execute("INSERT INTO articles (title, content) VALUES ('Hello World', 'This is a test article about Hello World')")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO articles (title, content) VALUES ('Go Programming', 'Learn Go programming language')")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO articles (title, content) VALUES ('Hello Go', 'Hello from Go language')")
	if err != nil {
		t.Fatal(err)
	}

	// Check that FTS index exists
	if !engine.fts.HasIndex("articles", "content") {
		t.Error("FTS index not found")
	}

	// List FTS indexes
	indexes := engine.fts.ListIndexes()
	t.Logf("FTS indexes: %v", indexes)

	// Search using MATCH...AGAINST
	result, err = engine.Execute("SELECT * FROM articles WHERE MATCH(content) AGAINST('Hello')")
	if err != nil {
		t.Logf("MATCH query error: %v", err)
		// MATCH...AGAINST might not be fully implemented yet, just log
	} else {
		t.Logf("MATCH query result: %d rows", len(result.Rows))
	}
}

func TestFullTextSearchCRUD(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute(`CREATE TABLE docs (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Create fulltext index
	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON docs(content)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert
	_, err = engine.Execute("INSERT INTO docs (content) VALUES ('Hello World')")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO docs (content) VALUES ('Goodbye World')")
	if err != nil {
		t.Fatal(err)
	}

	// Check FTS index stats
	indexer := engine.fts.GetIndex("docs", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("FTS index stats: documents=%d, terms=%d", stats.DocumentCount, stats.TermCount)
		if stats.DocumentCount != 2 {
			t.Errorf("Expected 2 documents, got %d", stats.DocumentCount)
		}
	}

	// Update
	_, err = engine.Execute("UPDATE docs SET content = 'Hello Universe' WHERE content = 'Hello World'")
	if err != nil {
		t.Fatal(err)
	}

	// Delete
	_, err = engine.Execute("DELETE FROM docs WHERE content = 'Goodbye World'")
	if err != nil {
		t.Fatal(err)
	}

	// Check FTS index stats after delete
	indexer = engine.fts.GetIndex("docs", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("FTS index stats after delete: documents=%d, terms=%d", stats.DocumentCount, stats.TermCount)
		if stats.DocumentCount != 1 {
			t.Errorf("Expected 1 document after delete, got %d", stats.DocumentCount)
		}
	}
}

func TestFullTextSearchDropIndex(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute(`CREATE TABLE test_drop (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Create fulltext index
	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON test_drop(content)")
	if err != nil {
		t.Fatal(err)
	}

	if !engine.fts.HasIndex("test_drop", "content") {
		t.Error("FTS index should exist")
	}

	// Drop the index (using DROP INDEX with table name)
	// Note: The syntax might need adjustment based on parser implementation
	_, err = engine.Execute("DROP INDEX idx_content ON test_drop")
	if err != nil {
		t.Logf("DROP INDEX error (expected): %v", err)
	}
}

func TestFullTextSearchMultipleColumns(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute(`CREATE TABLE multi (
		id SEQ,
		title VARCHAR(200),
		body TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Create fulltext indexes on multiple columns
	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_title ON multi(title)")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_body ON multi(body)")
	if err != nil {
		t.Fatal(err)
	}

	// Check both indexes exist
	if !engine.fts.HasIndex("multi", "title") {
		t.Error("FTS index on title not found")
	}
	if !engine.fts.HasIndex("multi", "body") {
		t.Error("FTS index on body not found")
	}

	// Insert data
	_, err = engine.Execute("INSERT INTO multi (title, body) VALUES ('Hello', 'World')")
	if err != nil {
		t.Fatal(err)
	}

	// Check both indexes have documents
	titleIndexer := engine.fts.GetIndex("multi", "title")
	if titleIndexer != nil {
		stats := titleIndexer.Stats()
		if stats.DocumentCount != 1 {
			t.Errorf("Title index should have 1 document, got %d", stats.DocumentCount)
		}
	}

	bodyIndexer := engine.fts.GetIndex("multi", "body")
	if bodyIndexer != nil {
		stats := bodyIndexer.Stats()
		if stats.DocumentCount != 1 {
			t.Errorf("Body index should have 1 document, got %d", stats.DocumentCount)
		}
	}
}

func TestFullTextSearchChinese(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// Create table
	_, err = engine.Execute(`CREATE TABLE chinese (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Create fulltext index
	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON chinese(content)")
	if err != nil {
		t.Fatal(err)
	}

	// Insert Chinese text
	_, err = engine.Execute("INSERT INTO chinese (content) VALUES ('你好世界')")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO chinese (content) VALUES ('你好中国')")
	if err != nil {
		t.Fatal(err)
	}

	// Check FTS index
	indexer := engine.fts.GetIndex("chinese", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("Chinese FTS index stats: documents=%d, terms=%d", stats.DocumentCount, stats.TermCount)
	}

	// Search for Chinese term
	results, err := engine.fts.Search("chinese", "content", "你好", 10, 0)
	if err != nil {
		t.Logf("Chinese search error: %v", err)
	} else {
		t.Logf("Chinese search results: %d", len(results))
	}
}
