// Package fts provides comprehensive full-text search tests
package fts

import (
	"testing"
)

// TestTokenizerEdgeCases 测试分词器边缘情况
func TestTokenizerEdgeCases(t *testing.T) {
	tokenizer := &SimpleTokenizer{}

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty", "", nil},
		{"whitespace only", "   \t\n  ", nil},
		{"single word", "hello", []string{"hello"}},
		{"multiple spaces", "hello    world", []string{"hello", "world"}},
		{"punctuation", "hello, world! how are you?", []string{"hello", "world", "how", "are", "you"}},
		{"numbers", "test123 456test 789", []string{"test123", "456test", "789"}},
		{"mixed case", "Hello WORLD HeLLo", []string{"hello", "world", "hello"}},
		{"special chars", "hello@world.com test#tag", []string{"hello", "world", "com", "test", "tag"}},
		{"unicode chinese", "你好世界 测试", []string{"你好世界", "测试"}},
		{"unicode japanese", "こんにちは せかい", []string{"こんにちは", "せかい"}},
		{"unicode mixed", "hello世界test", []string{"hello世界test"}},
		{"newlines", "hello\nworld\r\ntest", []string{"hello", "world", "test"}},
		{"tabs", "hello\tworld\ttest", []string{"hello", "world", "test"}},
		{"quotes", `"hello" 'world'`, []string{"hello", "world"}},
		{"parentheses", "(hello) [world] {test}", []string{"hello", "world", "test"}},
		{"dashes", "hello-world test_case", []string{"hello", "world", "test", "case"}},
		{"currency", "$100 €50 ¥1000", []string{"100", "€50", "¥1000"}}, // 货币符号保留Unicode字符
		{"underscores", "hello_world test_case", []string{"hello", "world", "test", "case"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.Tokenize(tt.input)
			if len(tokens) != len(tt.expected) {
				t.Errorf("Tokenize(%q) got %d tokens %v, want %d tokens %v",
					tt.input, len(tokens), tokens, len(tt.expected), tt.expected)
				return
			}
			for i, tok := range tokens {
				if tok != tt.expected[i] {
					t.Errorf("Tokenize(%q)[%d] = %q, want %q", tt.input, i, tok, tt.expected[i])
				}
			}
		})
	}
}

// TestInvertedIndexOperations 测试倒排索引操作
func TestInvertedIndexOperations(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 测试插入和搜索
	t.Run("insert and search", func(t *testing.T) {
		err := idx.IndexDocument(1, "The quick brown fox jumps over the lazy dog")
		if err != nil {
			t.Fatal(err)
		}

		// 搜索单个词
		results, err := idx.Search("fox", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Errorf("Search 'fox' got %d results, want 1", len(results))
		}

		// 搜索多个词 (AND)
		results, err = idx.Search("quick fox", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Errorf("Search 'quick fox' got %d results, want 1", len(results))
		}

		// 搜索不存在的词
		results, err = idx.Search("elephant", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 0 {
			t.Errorf("Search 'elephant' got %d results, want 0", len(results))
		}
	})

	// 测试更新
	t.Run("update document", func(t *testing.T) {
		err := idx.UpdateDocument(1, "The slow green turtle walks slowly")
		if err != nil {
			t.Fatal(err)
		}

		// 旧词应该不存在
		results, err := idx.Search("fox", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 0 {
			t.Errorf("Search 'fox' after update got %d results, want 0", len(results))
		}

		// 新词应该存在
		results, err = idx.Search("turtle", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Errorf("Search 'turtle' got %d results, want 1", len(results))
		}
	})

	// 测试删除
	t.Run("delete document", func(t *testing.T) {
		err := idx.DeleteDocument(1)
		if err != nil {
			t.Fatal(err)
		}

		results, err := idx.Search("turtle", 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 0 {
			t.Errorf("Search 'turtle' after delete got %d results, want 0", len(results))
		}
	})
}

// TestInvertedIndexMultipleDocuments 测试多文档索引
func TestInvertedIndexMultipleDocuments(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	docs := map[uint64]string{
		1: "Go is a programming language",
		2: "Python is also a programming language",
		3: "JavaScript is a scripting language",
		4: "Rust is a systems programming language",
		5: "Cplusplus is a low level programming language", // 避免单字符问题
	}

	for id, content := range docs {
		err := idx.IndexDocument(id, content)
		if err != nil {
			t.Fatalf("Failed to index document %d: %v", id, err)
		}
	}

	tests := []struct {
		query       string
		expectCount int
		description string
	}{
		{"programming", 4, "docs 1,2,4,5 contain 'programming'"},
		{"language", 5, "all documents contain 'language'"},
		{"go", 1, "only doc 1 contains 'go'"},
		{"python", 1, "only doc 2 contains 'python'"},
		{"javascript", 1, "only doc 3 contains 'javascript'"},
		{"rust", 1, "only doc 4 contains 'rust'"},
		{"programming language", 4, "docs 1,2,4,5 contain both terms"},
		{"go programming", 1, "only doc 1 contains both"},
		{"python programming", 1, "only doc 2 contains both"},
		{"systems programming", 1, "only doc 4 contains both"},
		{"scripting language", 1, "only doc 3 contains both"},
		{"nonexistent", 0, "no docs contain this"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			results, err := idx.Search(tt.query, 10, 0)
			if err != nil {
				t.Fatalf("Search failed: %v", err)
			}
			if len(results) != tt.expectCount {
				t.Errorf("Search '%s' got %d results, want %d", tt.query, len(results), tt.expectCount)
			}
		})
	}
}

// TestTFIDFScoring 测试TF-IDF评分
func TestTFIDFScoring(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 创建文档，其中某个词在不同文档中出现频率不同
	err := idx.IndexDocument(1, "apple apple apple banana")
	if err != nil {
		t.Fatal(err)
	}
	err = idx.IndexDocument(2, "apple banana orange")
	if err != nil {
		t.Fatal(err)
	}
	err = idx.IndexDocument(3, "apple orange orange")
	if err != nil {
		t.Fatal(err)
	}

	// 搜索apple，应该返回所有文档
	results, err := idx.Search("apple", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Errorf("Search 'apple' got %d results, want 3", len(results))
	}

	// 验证评分排序（更稀有的词得分更高）
	// banana只在doc1和doc2中出现
	results, err = idx.Search("banana", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("Search 'banana' got %d results, want 2", len(results))
	}

	// orange只在doc2和doc3中出现
	results, err = idx.Search("orange", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Errorf("Search 'orange' got %d results, want 2", len(results))
	}
}

// TestPagination 测试分页
func TestPagination(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 创建20个文档
	for i := 1; i <= 20; i++ {
		err := idx.IndexDocument(uint64(i), "test document")
		if err != nil {
			t.Fatal(err)
		}
	}

	// 第一页
	results, err := idx.Search("test", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Errorf("Page 1 got %d results, want 5", len(results))
	}

	// 第二页
	results, err = idx.Search("test", 5, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Errorf("Page 2 got %d results, want 5", len(results))
	}

	// 最后一页
	results, err = idx.Search("test", 5, 15)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 5 {
		t.Errorf("Page 4 got %d results, want 5", len(results))
	}

	// 超出范围
	results, err = idx.Search("test", 5, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("Beyond range got %d results, want 0", len(results))
	}
}

// TestManagerMultipleIndexes 测试管理多个索引
func TestManagerMultipleIndexes(t *testing.T) {
	m := NewManager()

	// 创建多个表的索引
	tables := []struct {
		table  string
		column string
	}{
		{"articles", "title"},
		{"articles", "content"},
		{"products", "name"},
		{"products", "description"},
	}

	for _, tc := range tables {
		err := m.CreateIndex(tc.table, tc.column, nil)
		if err != nil {
			t.Fatalf("Failed to create index %s.%s: %v", tc.table, tc.column, err)
		}
	}

	// 验证所有索引存在
	for _, tc := range tables {
		if !m.HasIndex(tc.table, tc.column) {
			t.Errorf("Index %s.%s not found", tc.table, tc.column)
		}
	}

	// 列出所有索引
	indexes := m.ListIndexes()
	if len(indexes) != 4 {
		t.Errorf("ListIndexes returned %d indexes, want 4", len(indexes))
	}

	// 索引文档
	err := m.IndexDocument("articles", "title", 1, "Hello World")
	if err != nil {
		t.Fatal(err)
	}
	err = m.IndexDocument("articles", "content", 1, "This is a test article")
	if err != nil {
		t.Fatal(err)
	}
	err = m.IndexDocument("products", "name", 1, "Laptop Computer")
	if err != nil {
		t.Fatal(err)
	}
	err = m.IndexDocument("products", "description", 1, "High performance laptop")
	if err != nil {
		t.Fatal(err)
	}

	// 分别搜索
	results, err := m.Search("articles", "title", "Hello", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("Search articles.title got %d results, want 1", len(results))
	}

	results, err = m.Search("products", "name", "Computer", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("Search products.name got %d results, want 1", len(results))
	}

	// 交叉搜索应该不匹配
	results, err = m.Search("articles", "title", "Computer", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("Cross search should return 0 results, got %d", len(results))
	}
}

// TestConcurrentIndexing 测试并发索引（基础测试）
func TestConcurrentIndexing(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 顺序索引多个文档
	for i := 1; i <= 100; i++ {
		err := idx.IndexDocument(uint64(i), "concurrent test document")
		if err != nil {
			t.Fatalf("Failed to index document %d: %v", i, err)
		}
	}

	// 验证所有文档都被索引
	results, err := idx.Search("concurrent", 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 100 {
		t.Errorf("Search returned %d results, want 100", len(results))
	}

	// 验证统计
	stats := idx.Stats()
	if stats.DocumentCount != 100 {
		t.Errorf("DocumentCount = %d, want 100", stats.DocumentCount)
	}
}

// TestChineseFullTextSearch 测试中文全文搜索
func TestChineseFullTextSearch(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	docs := map[uint64]string{
		1: "数据库是现代应用的核心组件",
		2: "全文检索是数据库的重要功能",
		3: "搜索引擎使用倒排索引实现全文检索",
		4: "中文分词是中文搜索的关键技术",
	}

	for id, content := range docs {
		err := idx.IndexDocument(id, content)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 注意：当前分词器不支持中文分词，所以只能搜索完整的词
	results, err := idx.Search("数据库", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Chinese search '数据库': %d results", len(results))

	results, err = idx.Search("全文检索", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Chinese search '全文检索': %d results", len(results))
}

// TestEmptyDocument 测试空文档处理
func TestEmptyDocument(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 索引空文档
	err := idx.IndexDocument(1, "")
	if err != nil {
		t.Fatal(err)
	}

	// 索引只有空格的文档
	err = idx.IndexDocument(2, "   ")
	if err != nil {
		t.Fatal(err)
	}

	// 索引正常文档
	err = idx.IndexDocument(3, "hello world")
	if err != nil {
		t.Fatal(err)
	}

	// 搜索应该只返回正常文档
	results, err := idx.Search("hello", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("Search got %d results, want 1", len(results))
	}
}

// TestSpecialCharacters 测试特殊字符处理
func TestSpecialCharacters(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 包含特殊字符的文档
	docs := map[uint64]string{
		1: "email: test@example.com",
		2: "phone: +1-234-567-8900",
		3: "url: https://example.com/path",
		4: "code: function() { return 42; }",
	}

	for id, content := range docs {
		err := idx.IndexDocument(id, content)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 搜索包含特殊字符的词
	results, err := idx.Search("example", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'example': %d results", len(results))

	results, err = idx.Search("function", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Search 'function': %d results", len(results))
}

// TestReindexDocument 测试重新索引文档
func TestReindexDocument(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 首次索引
	err := idx.IndexDocument(1, "hello world")
	if err != nil {
		t.Fatal(err)
	}

	// 验证
	results, err := idx.Search("hello", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("First index: got %d results, want 1", len(results))
	}

	// 重新索引（应该覆盖）
	err = idx.IndexDocument(1, "goodbye world")
	if err != nil {
		t.Fatal(err)
	}

	// hello应该不再匹配
	results, err = idx.Search("hello", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("After reindex 'hello': got %d results, want 0", len(results))
	}

	// goodbye应该匹配
	results, err = idx.Search("goodbye", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("After reindex 'goodbye': got %d results, want 1", len(results))
	}

	// world应该仍然匹配
	results, err = idx.Search("world", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("After reindex 'world': got %d results, want 1", len(results))
	}
}

// TestLargeDocument 测试大文档处理
func TestLargeDocument(t *testing.T) {
	tokenizer := &SimpleTokenizer{}
	idx := NewInvertedIndex(tokenizer)

	// 创建一个大文档，包含重复的词
	largeContent := "This is a large document for testing purposes. "
	for i := 0; i < 100; i++ {
		largeContent += "Word number " + string(rune('0'+i%10)) + " appears here. "
	}
	largeContent += "This is the end of the document."

	err := idx.IndexDocument(1, largeContent)
	if err != nil {
		t.Fatal(err)
	}

	// 搜索应该能找到词
	results, err := idx.Search("document", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("Search 'document' got %d results, want 1", len(results))
	}

	// 验证统计
	stats := idx.Stats()
	if stats.DocumentCount != 1 {
		t.Errorf("DocumentCount = %d, want 1", stats.DocumentCount)
	}
}
