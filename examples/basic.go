package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/topxeq/xxldb/driver"
)

func main() {
	// 打开数据库（文件模式）
	db, err := sql.Open("xxldb", "./testdb")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 创建表
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id SEQ,
		name VARCHAR(100),
		email VARCHAR(100),
		age INT,
		created_at DATETIME
	)`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("表创建成功")

	// 插入数据
	result, err := db.Exec(
		"INSERT INTO users (name, email, age, created_at) VALUES (?, ?, ?, ?)",
		"张三", "zhangsan@example.com", 25, "2026-01-01 10:00:00",
	)
	if err != nil {
		log.Fatal(err)
	}
	id, _ := result.LastInsertId()
	fmt.Printf("插入成功, ID: %d\n", id)

	// 查询数据
	rows, err := db.Query("SELECT id, name, email, age FROM users WHERE age > ?", 20)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\n查询结果:")
	for rows.Next() {
		var id int64
		var name, email string
		var age int
		if err := rows.Scan(&id, &name, &email, &age); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %d, Name: %s, Email: %s, Age: %d\n", id, name, email, age)
	}

	// 更新数据
	_, err = db.Exec("UPDATE users SET age = ? WHERE name = ?", 26, "张三")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("\n更新成功")

	// 使用聚合函数
	var count int64
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("用户总数: %d\n", count)

	// 删除数据
	_, err = db.Exec("DELETE FROM users WHERE id = ?", id)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("删除成功")
}
