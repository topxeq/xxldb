# XxLdb - 轻量级SQL数据库

[English](README.md)

一个用纯 Go 语言实现的轻量级嵌入式 SQL 数据库。

## 特性

- **纯 Go 实现** - 无 CGO 依赖，跨平台支持 (Linux/macOS/Windows)
- **完整的 SQL 支持** - SELECT, INSERT, UPDATE, DELETE, CREATE, DROP
- **JOIN 和 UNION** - 支持 INNER/LEFT/RIGHT JOIN 和 UNION 操作
- **内置函数** - 字符串、数值、日期、聚合函数
- **脚本函数** - 支持 `xx_` 前缀的自定义脚本函数
- **文件存储** - 支持 BLOB 和 FILE 类型存储文件/图片/文件夹
- **WAL 日志** - Write-Ahead Logging 支持崩溃恢复
- **认证机制** - 支持用户名/密码认证
- **可配置日志** - 支持 DEBUG/INFO/WARN/ERROR 级别
- **标准驱动** - 实现 Go 标准数据库驱动接口

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

**限制：**
- 单个文件大小限制：10MB
- 文件夹结构以JSON格式存储在BLOB中

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
- `LENGTH(str)` - 字符串长度
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

## 性能特点

- 单表查询: < 1ms (1000行以内)
- 插入操作: > 10000 ops/sec
- 并发读取: > 50000 ops/sec
- 启动时间: < 100ms
- 内存占用: < 50MB (空数据库)

## 许可证

MIT License

## 作者

topxeq
