# 全文检索 (FTS) 文档

[English](FTS.md)

## 概述

XxLdb 提供基于倒排索引的全文检索功能，支持 TF-IDF 评分排序。可以高效地搜索数据库表中的文本内容。

## 功能特性

- **倒排索引**：使用倒排索引结构实现快速文本搜索
- **TF-IDF 评分**：使用 TF-IDF 算法对搜索结果进行相关性排序
- **AND 搜索**：多个搜索词采用 AND 逻辑组合
- **Unicode 支持**：完整支持中文、日文等 Unicode 文本
- **自动索引维护**：INSERT/UPDATE/DELETE 时自动更新索引
- **多索引支持**：支持在不同列上创建多个全文索引

## 创建全文索引

### SQL 语法

```sql
CREATE FULLTEXT INDEX 索引名 ON 表名(列名);
```

### 示例

```sql
-- 创建表
CREATE TABLE articles (
    id SEQ,
    title VARCHAR(200),
    content TEXT,
    created_at DATETIME
);

-- 在单列上创建全文索引
CREATE FULLTEXT INDEX idx_content ON articles(content);

-- 创建多个全文索引
CREATE FULLTEXT INDEX idx_title ON articles(title);
CREATE FULLTEXT INDEX idx_body ON articles(body);
```

## 搜索

### 基本搜索

```sql
-- 搜索包含"数据库"的文档
SELECT * FROM articles 
WHERE MATCH(content) AGAINST('数据库');
```

### 多词搜索

多个搜索词采用 AND 逻辑组合，所有词都必须匹配：

```sql
-- 搜索同时包含"数据库"和"编程"的文档
SELECT * FROM articles 
WHERE MATCH(content) AGAINST('数据库 编程');
```

### 排序

```sql
SELECT id, title, content 
FROM articles 
WHERE MATCH(content) AGAINST('搜索词') 
ORDER BY id DESC
LIMIT 10;
```

## 索引维护

全文索引在数据变更时自动维护：

### INSERT

```sql
INSERT INTO articles (title, content) 
VALUES ('简介', '这篇文章介绍数据库技术');
-- 内容自动被索引
```

### UPDATE

```sql
UPDATE articles 
SET content = '更新后的编程内容' 
WHERE id = 1;
-- 索引自动更新
```

### DELETE

```sql
DELETE FROM articles WHERE id = 1;
-- 文档自动从索引中移除
```

## 高级用法

### 索引已有数据

在已有数据的表上创建全文索引时，现有行会被自动索引：

```sql
-- 先插入数据
INSERT INTO articles (title, content) VALUES ('文章1', '内容1');
INSERT INTO articles (title, content) VALUES ('文章2', '内容2');

-- 然后创建索引 - 已有数据被自动索引
CREATE FULLTEXT INDEX idx_content ON articles(content);
```

### 多列索引

可以在不同列上创建独立的全文索引：

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

-- 在特定列搜索
SELECT * FROM documents WHERE MATCH(title) AGAINST('重要');
SELECT * FROM documents WHERE MATCH(body) AGAINST('数据库');
```

## API 参考

### Go API

```go
package main

import (
    "fmt"
    "github.com/topxeq/xxldb"
)

func main() {
    // 打开数据库
    engine, err := xxldb.Open("/path/to/database")
    if err != nil {
        panic(err)
    }
    defer engine.Close()
    
    // 创建全文索引
    _, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON mytable(content)")
    if err != nil {
        panic(err)
    }
    
    // 检查索引是否存在
    hasIndex := engine.FTS().HasIndex("mytable", "content")
    fmt.Printf("索引存在: %v\n", hasIndex)
    
    // 获取索引统计信息
    indexer := engine.FTS().GetIndex("mytable", "content")
    if indexer != nil {
        stats := indexer.Stats()
        fmt.Printf("文档数: %d\n", stats.DocumentCount)
        fmt.Printf("词汇数: %d\n", stats.TermCount)
    }
    
    // 执行搜索
    result, err := engine.Execute(
        "SELECT * FROM mytable WHERE MATCH(content) AGAINST('搜索词')",
    )
    if err != nil {
        panic(err)
    }
    
    for _, row := range result.Rows {
        fmt.Println(row)
    }
}
```

## 实现细节

### 分词

- 文本转换为小写
- 按空格和标点符号分割
- 支持 Unicode 字符（视为单词字符）
- 每个唯一的词被索引

### 倒排索引结构

```
词          -> [文档ID1, 文档ID2, 文档ID3, ...]
----------------------------------------
"数据库"    -> [1, 5, 8, 12]
"搜索"      -> [1, 3, 5]
"全文"      -> [2, 4, 8]
```

### TF-IDF 评分

搜索结果使用简化的 TF-IDF 评分：

```
得分 = tf * idf

其中:
  tf  = 词在文档中的频率
  idf = log(总文档数 / 包含该词的文档数)
```

分数越高表示结果越相关。

## 限制

1. **不支持短语搜索**：无法搜索精确短语如 `"全文检索"`
2. **不支持 OR 搜索**：所有词都使用 AND 逻辑
3. **不支持布尔操作符**：不能使用 `+`、`-`、`AND`、`OR` 操作符
4. **不支持通配符**：不支持 `*` 或 `?` 通配符
5. **简单分词**：中文/日文文本不分词（作为整体词处理）

## 性能建议

1. **批量插入后创建索引**：插入数据后再创建索引比向已有索引插入数据更快
2. **限制结果集**：使用 `LIMIT` 限制返回结果数量
3. **只索引必要的列**：不需要全文搜索的列不要创建索引

## 测试

```bash
# 运行 FTS 测试
go test ./fts/... -v

# 运行 FTS 集成测试
go test ./executor/... -v -run TestFTS

# 运行并显示覆盖率
go test ./fts/... -cover
```

## 示例应用

```go
package main

import (
    "fmt"
    "log"
    "github.com/topxeq/xxldb"
)

func main() {
    // 创建内存数据库
    engine, err := xxldb.NewEngine("", true)
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close()
    
    // 创建表
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
    
    // 创建全文索引
    _, err = engine.Execute("CREATE FULLTEXT INDEX idx_content ON wiki(content)")
    if err != nil {
        log.Fatal(err)
    }
    
    // 插入文档
    documents := []struct {
        title, content string
    }{
        {"Go编程", "Go是一门现代编程语言"},
        {"Python教程", "Python适合初学者学习"},
        {"数据库设计", "数据库设计对应用开发至关重要"},
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
    
    // 搜索
    result, err := engine.Execute(
        "SELECT * FROM wiki WHERE MATCH(content) AGAINST('编程')",
    )
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Println("搜索结果:")
    for _, row := range result.Rows {
        fmt.Printf("  %s: %s\n", row.Data[1], row.Data[2])
    }
}
```
