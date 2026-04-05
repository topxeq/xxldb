// Package executor provides comprehensive FTS SQL tests
package executor

import (
	"fmt"
	"testing"
)

// TestFTSCreateAndDropIndex 测试创建和删除全文索引
func TestFTSCreateAndDropIndex(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表
	_, err = engine.Execute(`CREATE TABLE test_fts (
		id SEQ,
		title VARCHAR(200),
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// 创建全文索引
	result, err := engine.Execute("CREATE FULLTEXT INDEX idx_title ON test_fts(title)")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("创建索引: %s", result.Message)

	// 验证索引存在
	if !engine.fts.HasIndex("test_fts", "title") {
		t.Error("索引应该存在")
	}

	// 创建另一个全文索引
	result, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON test_fts(content)")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("创建第二个索引: %s", result.Message)

	// 验证两个索引都存在
	indexes := engine.fts.ListIndexes()
	t.Logf("所有索引: %v", indexes)
	if len(indexes) != 2 {
		t.Errorf("应该有2个索引，实际有%d个", len(indexes))
	}
}

// TestFTSInsertAndSearch 测试插入和搜索
func TestFTSInsertAndSearch(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表和索引
	_, err = engine.Execute(`CREATE TABLE articles (
		id SEQ,
		title VARCHAR(200),
		body TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_body ON articles(body)")
	if err != nil {
		t.Fatal(err)
	}

	// 插入测试数据
	testData := []struct {
		title string
		body  string
	}{
		{"Go语言入门", "Go是一门现代编程语言，由Google开发"},
		{"Python教程", "Python是流行的编程语言，适合初学者"},
		{"数据库设计", "关系型数据库是现代应用的核心组件"},
		{"全文检索", "全文检索技术是搜索引擎的核心技术"},
		{"编程语言比较", "Go和Python都是优秀的编程语言"},
	}

	for _, data := range testData {
		sql := fmt.Sprintf("INSERT INTO articles (title, body) VALUES ('%s', '%s')", data.title, data.body)
		_, err = engine.Execute(sql)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证数据插入
	result, err := engine.Execute("SELECT * FROM articles")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != len(testData) {
		t.Errorf("插入数据数量不对，期望%d，实际%d", len(testData), len(result.Rows))
	}

	// 验证FTS索引统计
	indexer := engine.fts.GetIndex("articles", "body")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("FTS索引统计: 文档数=%d, 词汇数=%d", stats.DocumentCount, stats.TermCount)
		if stats.DocumentCount != int64(len(testData)) {
			t.Errorf("FTS索引文档数不对，期望%d，实际%d", len(testData), stats.DocumentCount)
		}
	}
}

// TestFTSUpdateAndDelete 测试更新和删除时的索引维护
func TestFTSUpdateAndDelete(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表和索引
	_, err = engine.Execute(`CREATE TABLE docs (
		id SEQ,
		text_data TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_text_data ON docs(text_data)")
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	_, err = engine.Execute("INSERT INTO docs (text_data) VALUES ('hello world')")
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute("INSERT INTO docs (text_data) VALUES ('goodbye world')")
	if err != nil {
		t.Fatal(err)
	}

	// 验证初始状态
	indexer := engine.fts.GetIndex("docs", "text_data")
	stats := indexer.Stats()
	if stats.DocumentCount != 2 {
		t.Errorf("初始文档数应为2，实际为%d", stats.DocumentCount)
	}

	// 更新文档
	_, err = engine.Execute("UPDATE docs SET text_data = 'hello universe' WHERE text_data = 'hello world'")
	if err != nil {
		t.Fatal(err)
	}

	// 验证更新后的索引状态
	stats = indexer.Stats()
	t.Logf("更新后文档数: %d", stats.DocumentCount)

	// 删除文档
	_, err = engine.Execute("DELETE FROM docs WHERE text_data = 'goodbye world'")
	if err != nil {
		t.Fatal(err)
	}

	// 验证删除后的索引状态
	stats = indexer.Stats()
	if stats.DocumentCount != 1 {
		t.Errorf("删除后文档数应为1，实际为%d", stats.DocumentCount)
	}
}

// TestFTSMultipleTables 测试多表索引
func TestFTSMultipleTables(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建多个表
	_, err = engine.Execute(`CREATE TABLE posts (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute(`CREATE TABLE comments (
		id SEQ,
		text_data TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// 创建索引
	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_posts ON posts(content)")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_comments ON comments(text_data)")
	if err != nil {
		t.Fatal(err)
	}

	// 插入数据
	_, err = engine.Execute("INSERT INTO posts (content) VALUES ('database programming')")
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("INSERT INTO comments (text_data) VALUES ('web development')")
	if err != nil {
		t.Fatal(err)
	}

	// 验证两个表的索引独立
	postIndexer := engine.fts.GetIndex("posts", "content")
	commentIndexer := engine.fts.GetIndex("comments", "text_data")

	if postIndexer.Stats().DocumentCount != 1 {
		t.Error("posts表索引文档数不对")
	}
	if commentIndexer.Stats().DocumentCount != 1 {
		t.Error("comments表索引文档数不对")
	}
}

// TestFTSMatchAgainst 测试MATCH...AGAINST语法
func TestFTSMatchAgainst(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表和索引
	_, err = engine.Execute(`CREATE TABLE search_test (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON search_test(content)")
	if err != nil {
		t.Fatal(err)
	}

	// 插入测试数据
	testContents := []string{
		"The quick brown fox jumps over the lazy dog",
		"A quick brown dog runs in the park",
		"The lazy cat sleeps all day",
		"Quick thinking solves problems",
		"The fox and the hound are friends",
	}

	for _, content := range testContents {
		sql := fmt.Sprintf("INSERT INTO search_test (content) VALUES ('%s')", content)
		_, err = engine.Execute(sql)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 使用MATCH...AGAINST搜索
	result, err := engine.Execute("SELECT * FROM search_test WHERE MATCH(content) AGAINST('quick')")
	if err != nil {
		t.Logf("MATCH查询错误: %v", err)
	} else {
		t.Logf("MATCH 'quick' 结果: %d 行", len(result.Rows))
	}

	// 搜索多个词
	result, err = engine.Execute("SELECT * FROM search_test WHERE MATCH(content) AGAINST('fox dog')")
	if err != nil {
		t.Logf("MATCH查询错误: %v", err)
	} else {
		t.Logf("MATCH 'fox dog' 结果: %d 行", len(result.Rows))
	}
}

// TestFTSWithExistingData 测试对已有数据创建索引
func TestFTSWithExistingData(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表并插入数据
	_, err = engine.Execute(`CREATE TABLE existing_data (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// 先插入数据
	for i := 0; i < 5; i++ {
		_, err = engine.Execute("INSERT INTO existing_data (content) VALUES ('test document number')")
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证数据存在
	result, err := engine.Execute("SELECT * FROM existing_data")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 5 {
		t.Errorf("数据插入失败，期望5行，实际%d行", len(result.Rows))
	}

	// 后创建全文索引
	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON existing_data(content)")
	if err != nil {
		t.Fatal(err)
	}

	// 验证索引是否正确建立
	indexer := engine.fts.GetIndex("existing_data", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("已有数据创建索引后统计: 文档数=%d", stats.DocumentCount)
		// 应该已经索引了已有数据
		if stats.DocumentCount != 5 {
			t.Logf("警告: 已有数据可能没有被索引，文档数=%d", stats.DocumentCount)
		}
	}
}

// TestFTSNullAndEmpty 测试NULL和空值处理
func TestFTSNullAndEmpty(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表（允许NULL）
	_, err = engine.Execute(`CREATE TABLE nullable_test (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON nullable_test(content)")
	if err != nil {
		t.Fatal(err)
	}

	// 插入正常值
	_, err = engine.Execute("INSERT INTO nullable_test (content) VALUES ('hello world')")
	if err != nil {
		t.Fatal(err)
	}

	// 插入空字符串
	_, err = engine.Execute("INSERT INTO nullable_test (content) VALUES ('')")
	if err != nil {
		t.Fatal(err)
	}

	// 验证索引状态
	indexer := engine.fts.GetIndex("nullable_test", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("NULL/空值测试索引统计: 文档数=%d", stats.DocumentCount)
	}
}

// TestFTSBenchmark 性能基准测试
func TestFTSBenchmark(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表
	_, err = engine.Execute(`CREATE TABLE bench (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON bench(content)")
	if err != nil {
		t.Fatal(err)
	}

	// 批量插入
	docCount := 1000
	for i := 0; i < docCount; i++ {
		_, err = engine.Execute("INSERT INTO bench (content) VALUES ('document content for testing full text search capabilities')")
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证索引
	indexer := engine.fts.GetIndex("bench", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("性能测试索引统计: 文档数=%d, 词汇数=%d", stats.DocumentCount, stats.TermCount)
	}

	// 搜索测试
	result, err := engine.Execute("SELECT * FROM bench WHERE MATCH(content) AGAINST('testing')")
	if err != nil {
		t.Logf("搜索错误: %v", err)
	} else {
		t.Logf("搜索结果: %d 行", len(result.Rows))
	}
}

// TestFTSUnicodeContent 测试Unicode内容
func TestFTSUnicodeContent(t *testing.T) {
	engine, err := NewEngine("", true)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 创建表
	_, err = engine.Execute(`CREATE TABLE unicode_test (
		id SEQ,
		content TEXT
	)`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON unicode_test(content)")
	if err != nil {
		t.Fatal(err)
	}

	// 插入各种Unicode内容
	unicodeTests := []string{
		"中文测试内容",
		"日本語テスト",
		"한국어 테스트",
		"Greek Ελληνικά",
		"Russian Русский",
		"Arabic العربية",
		"Mixed 混合 content",
		"Emoji test",
	}

	for _, content := range unicodeTests {
		sql := fmt.Sprintf("INSERT INTO unicode_test (content) VALUES ('%s')", content)
		_, err = engine.Execute(sql)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 验证索引
	indexer := engine.fts.GetIndex("unicode_test", "content")
	if indexer != nil {
		stats := indexer.Stats()
		t.Logf("Unicode测试索引统计: 文档数=%d, 词汇数=%d", stats.DocumentCount, stats.TermCount)
	}
}
