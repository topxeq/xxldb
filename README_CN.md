# XxLdb - 轻量级SQL数据库

[English](README.md)

[![Go Report Card](https://goreportcard.com/badge/github.com/topxeq/xxldb)](https://goreportcard.com/report/github.com/topxeq/xxldb)
[![GoDoc](https://godoc.org/github.com/topxeq/xxldb?status.svg)](https://godoc.org/github.com/topxeq/xxldb)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

一个用纯 Go 语言实现的轻量级嵌入式 SQL 数据库。

## 特性

- **纯 Go 实现** - 无 CGO 依赖，跨平台支持 (Linux/macOS/Windows)
- **完整的 SQL 支持** - SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, ALTER
- **JOIN 和 UNION** - 支持 INNER/LEFT/RIGHT JOIN 和 UNION 操作
- **内置函数** - 字符串、数值、日期、聚合、图像函数
- **脚本函数** - 支持 `xx_` 前缀的自定义脚本函数
- **文件存储** - 支持 BLOB、FILE 和 IMAGE 类型存储文件/图片/文件夹
- **Unicode 支持** - 字符串函数完整支持 Unicode
- **WAL 日志** - Write-Ahead Logging 支持崩溃恢复
- **认证机制** - 支持用户名/密码认证
- **可配置日志** - 支持 DEBUG/INFO/WARN/ERROR 级别
- **标准驱动** - 实现 Go 标准数据库驱动接口
- **数据导入** - 支持 MySQL、PostgreSQL、SQLite、Oracle、MS SQL Server 数据导入

## 数据类型

| 类型 | 说明 |
|------|------|
| SEQ | 自增序号 (int64) |
| INT | 整数 (int64) |
| FLOAT | 浮点数 (float64) |
| CHAR(n) | 定长字符串 |
| VARCHAR(n) | 变长字符串 |
| TEXT | 大文本 |
| DATE | 日期 |
| TIME | 时间 |
| DATETIME | 日期时间 |
| BLOB | 二进制大对象 |
| FILE | 文件引用 |
| IMAGE | 图像（支持 PNG, JPEG, GIF, BMP, TIFF, WebP） |

## 安装

```bash
go get github.com/topxeq/xxldb
```

## 快速开始

### 基本用法

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    _ "github.com/topxeq/xxldb/driver"
)

func main() {
    // 打开数据库
    db, err := sql.Open("xxldb", "/path/to/database")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 创建表
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

    // 插入数据
    result, err := db.Exec(
        "INSERT INTO users (name, email, age, created_at) VALUES (?, ?, ?, ?)",
        "张三", "zhangsan@example.com", 25, "2026-01-01 10:00:00",
    )
    if err != nil {
        log.Fatal(err)
    }
    id, _ := result.LastInsertId()
    fmt.Printf("插入 ID: %d\n", id)

    // 查询数据
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
        fmt.Printf("ID: %d, 姓名: %s, 邮箱: %s, 年龄: %d\n", id, name, email, age)
    }
}
```

### 内存模式

```go
db, err := sql.Open("xxldb", ":memory:")
```

### 文件存储

```sql
-- 创建支持文件的表
CREATE TABLE documents (
    id SEQ,
    name VARCHAR(255),
    content BLOB,
    created_at DATETIME
);

-- 插入文件内容
INSERT INTO documents (name, content, created_at)
VALUES ('report.pdf', LOAD_FILE('/path/to/report.pdf'), NOW());

-- 导出文件
SELECT content INTO OUTFILE '/tmp/report_copy.pdf' FROM documents WHERE id = 1;
```

### 文件夹存储 (特色功能)

XxLdb 支持将整个文件夹存储到数据库中：

```sql
-- 创建文件夹存储表
CREATE TABLE folders (
    id SEQ,
    name VARCHAR(255),
    data BLOB,
    created_at DATETIME
);

-- 加载整个文件夹
INSERT INTO folders (name, data, created_at)
VALUES ('my_project', LOAD_FOLDER('/path/to/project'), NOW());

-- 查看文件夹内容
SELECT LIST_FOLDER(data) FROM folders WHERE name = 'my_project';

-- 统计文件数量
SELECT FOLDER_FILES(data) FROM folders WHERE name = 'my_project';

-- 导出文件夹到指定路径
SELECT EXPORT_FOLDER(data, '/tmp/restored_project') FROM folders WHERE name = 'my_project';
```

**文件夹函数说明：**

| 函数 | 说明 |
|------|------|
| `LOAD_FOLDER(path)` | 加载文件夹，返回包含完整结构的BLOB数据 |
| `EXPORT_FOLDER(data, path)` | 将文件夹数据导出到指定路径 |
| `LIST_FOLDER(data)` | 列出文件夹内容（树形结构） |
| `FOLDER_FILES(data)` | 统计文件夹中的文件数量 |

**说明：**
- 文件夹结构以JSON格式存储在BLOB中
- 文件大小限制可通过配置 `MaxFileSize` 设置（默认：无限制）

## 命令行客户端

```bash
# 打开数据库
xxldb -db /path/to/database

# 内存模式
xxldb -memory

# 执行单条SQL
xxldb -db /path/to/db -e "SELECT * FROM users"

# 设置用户名密码
xxldb -db /path/to/db -user admin -password secret
```

### 客户端命令

| 命令 | 说明 |
|------|------|
| .help | 显示帮助 |
| .tables | 列出所有表 |
| .schema <表名> | 显示表结构 |
| .backup <路径> | 备份数据库 |
| .restore <路径> | 恢复数据库 |
| .user <用户名> | 设置用户名 |
| .password <密码> | 设置密码 |
| .log <级别> | 设置日志级别 |
| .quit | 退出程序 |

## 内置函数

### 字符串函数
- `CONCAT(str1, str2, ...)` - 连接字符串
- `LENGTH(str)` - 字符串长度（字符数，支持Unicode）
- `BYTE_LENGTH(str)` / `OCTET_LENGTH(str)` - 字符串长度（字节数）
- `CHAR_LENGTH(str)` / `CHARACTER_LENGTH(str)` - LENGTH的别名
- `UPPER(str)` - 转大写
- `LOWER(str)` - 转小写
- `TRIM(str)` - 去除首尾空格
- `SUBSTRING(str, start, len)` - 子字符串
- `REPLACE(str, old, new)` - 替换字符串

### 数值函数
- `ABS(n)` - 绝对值
- `ROUND(n, precision)` - 四舍五入
- `FLOOR(n)` - 向下取整
- `CEIL(n)` - 向上取整
- `POWER(base, exp)` - 幂运算
- `SQRT(n)` - 平方根
- `MOD(a, b)` - 取模

### 聚合函数
- `COUNT(*)` - 计数
- `SUM(col)` - 求和
- `AVG(col)` - 平均值
- `MIN(col)` - 最小值
- `MAX(col)` - 最大值

### 日期函数
- `NOW()` - 当前日期时间
- `CURRENT_DATE()` - 当前日期
- `YEAR(date)` - 年份
- `MONTH(date)` - 月份
- `DAY(date)` - 日
- `DATEDIFF(d1, d2)` - 日期差
- `DATE_ADD(date, days)` - 日期加

### 转换函数
- `CAST(val AS type)` - 类型转换
- `COALESCE(val1, val2, ...)` - 返回第一个非空值
- `IFNULL(val, default)` - 空值替换

### 图像函数
- `LOAD_IMAGE(path)` - 从文件加载图像
- `IMAGE_FROM_BASE64(str)` - 从BASE64字符串创建图像
- `IMAGE_TO_BASE64(img)` - 转换为BASE64
- `IMAGE_TO_BASE64(img, 'datauri')` - 转换为Data URI格式
- `IMAGE_WIDTH(img)` - 获取图像宽度
- `IMAGE_HEIGHT(img)` - 获取图像高度
- `IMAGE_FORMAT(img)` - 获取图像格式 (png/jpeg/gif/...)
- `IMAGE_SIZE(img)` - 获取图像大小（字节）
- `IMAGE_MIME(img)` - 获取MIME类型

#### 图像示例
```sql
CREATE TABLE photos (id SEQ, name VARCHAR(100), img IMAGE);

-- 从文件加载
INSERT INTO photos (name, img) VALUES ('sunset', LOAD_IMAGE('/path/to/sunset.jpg'));

-- 从BASE64加载
INSERT INTO photos (name, img) VALUES ('avatar', IMAGE_FROM_BASE64('iVBORw0KGgo...'));

-- 从Data URI加载
INSERT INTO photos (name, img) VALUES ('logo', IMAGE_FROM_BASE64('data:image/png;base64,iVBORw0KGgo...'));

-- 查询图像信息
SELECT name, IMAGE_WIDTH(img), IMAGE_HEIGHT(img), IMAGE_FORMAT(img) FROM photos;

-- 导出为Data URI（可直接嵌入HTML）
SELECT name, IMAGE_TO_BASE64(img, 'datauri') FROM photos;
```

## 脚本函数

脚本函数以 `xx_` 为前缀，存储在 `xxscript` 系统表中：

```sql
-- 创建脚本函数
INSERT INTO xxscript (name, script, description)
VALUES ('xx_discount', '$1 * 0.9', '计算折扣价');

-- 使用脚本函数
SELECT name, xx_discount(price) AS discount_price FROM products;
```

## 项目结构

```
xxldb/
├── xxldb.go           # 主入口
├── types/             # 类型定义
├── storage/           # 存储引擎
│   ├── storage.go     # 存储管理
│   ├── page.go        # 页管理
│   └── wal.go         # WAL 日志
├── parser/            # SQL 解析器
│   ├── lexer.go       # 词法分析
│   ├── parser.go      # 语法解析
│   └── ast.go         # AST 定义
├── executor/          # 查询执行器
├── function/          # 内置函数
├── script/            # 脚本函数
├── auth/              # 认证模块
├── logger/            # 日志模块
├── driver/            # Go SQL 驱动
└── cmd/xxldb/         # 命令行客户端
```

## 配置选项

```go
config := xxldb.Config{
    Path:         "/path/to/db",    // 数据库路径
    InMemory:     false,            // 是否内存模式
    LogLevel:     "INFO",           // 日志级别
    Username:     "admin",          // 用户名
    Password:     "secret",         // 密码
    AutoCommit:   true,             // 自动提交
    SyncInterval: 1000,             // 同步间隔(毫秒)
}
engine, err := xxldb.OpenWithConfig(config)
```

## 备份恢复

### 备份

```bash
# 在客户端中
xxldb> .backup /path/to/backup

# 或使用SQL
BACKUP TO '/path/to/backup';
```

### 恢复

```bash
# 在客户端中
xxldb> .restore /path/to/backup

# 或使用SQL
RESTORE FROM '/path/to/backup';
```

## 数据导入

XxLdb 支持从 MySQL、PostgreSQL、SQLite、Oracle 和 MS SQL Server 导入数据。

### 命令行导入

```bash
# 从 MySQL 导入单个表
xxldb -db my.db -import "mysql://user:pass@localhost/dbname" -table users

# 从 PostgreSQL 导入所有表
xxldb -db my.db -import "postgresql://user:pass@localhost/dbname" -import-all

# 从 SQLite 导入
xxldb -db my.db -import "sqlite:///path/to/source.db" -import-all

# 从 Oracle 导入
xxldb -db my.db -import "oracle://user:pass@host:1521/sid" -table employees -to staff

# 从 MS SQL Server 导入
xxldb -db my.db -import "mssql://user:pass@host:1433/dbname" -import-all
```

### REPL 导入

```sql
-- 导入单个表
xxldb> .import mysql://user:pass@localhost/dbname users

-- 导入并指定不同的目标表名
xxldb> .import postgresql://user:pass@localhost/dbname old_table new_table

-- 导入所有表
xxldb> .import-all sqlite:///path/to/source.db
```

### 导入选项

| 选项 | 说明 |
|------|------|
| `-import <dsn>` | 源数据库连接字符串 |
| `-table <name>` | 要导入的源表 |
| `-to <name>` | 目标表名（默认：与源表同名） |
| `-import-all` | 导入源数据库中的所有表 |
| `-batch <size>` | 导入批次大小（默认：1000） |
| `-overwrite` | 覆盖已存在的表 |

### 支持的约束

XxLdb 导入时会保留以下约束：

| 约束 | MySQL | PostgreSQL | SQLite | Oracle | MSSQL |
|------|-------|------------|--------|--------|-------|
| PRIMARY KEY | ✅ | ✅ | ✅ | ✅ | ✅ |
| FOREIGN KEY | ✅ | ✅ | ✅ | ✅ | ✅ |
| UNIQUE | ✅ | ✅ | ✅ | ✅ | ✅ |
| CHECK | ✅ (8.0+) | ✅ | ✅ | ✅ | ✅ |
| INDEX | ✅ | ✅ | ✅ | ✅ | ✅ |

### 类型映射

导入时自动将源数据库类型映射为 XxLdb 类型：

| 源类型 | XxLdb 类型 |
|--------|-----------|
| INT, INTEGER, BIGINT | INT |
| FLOAT, DOUBLE, DECIMAL | FLOAT |
| CHAR, NCHAR | CHAR |
| VARCHAR, NVARCHAR | VARCHAR |
| TEXT, CLOB | TEXT |
| DATE | DATE |
| TIME | TIME |
| DATETIME, TIMESTAMP | DATETIME |
| BLOB, BINARY | BLOB |

## 性能特点

- 单表查询: < 1ms (1000行以内)
- 插入操作: > 10000 ops/sec
- 并发读取: > 50000 ops/sec
- 启动时间: < 100ms
- 内存占用: < 50MB (空数据库)

## 测试

```bash
# 运行所有测试
go test ./...

# 运行测试并显示覆盖率
go test -cover ./...

# 运行基准测试
go test -bench=. ./...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

## 贡献

欢迎贡献！请随时提交 Pull Request。

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建 Pull Request

## 许可证

MIT License - 详见 [LICENSE](LICENSE) 文件。

## 作者

topxeq
