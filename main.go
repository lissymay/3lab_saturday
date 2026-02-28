package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type product struct {
	id      int
	model   string
	company string
	price   int
}

func createProductsTable(db *sql.DB) error {
	createTableSQL := `CREATE TABLE IF NOT EXISTS products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		model TEXT,
		company TEXT,
		price INTEGER
	);`
	_, err := db.Exec(createTableSQL)
	return err
}

func insertProduct(db *sql.DB, model, company string, price int) (lastID, rowsAffected int64, err error) {
	result, err := db.Exec("INSERT INTO products (model, company, price) VALUES (?, ?, ?)", model, company, price)
	if err != nil {
		return 0, 0, err
	}
	lastID, _ = result.LastInsertId()
	rowsAffected, _ = result.RowsAffected()
	return lastID, rowsAffected, nil
}

func getAllProducts(db *sql.DB) ([]product, error) {
	rows, err := db.Query("SELECT * FROM products")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []product
	for rows.Next() {
		var p product
		if err := rows.Scan(&p.id, &p.model, &p.company, &p.price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func getProductsByMinPrice(db *sql.DB, minPrice int) ([]product, error) {
	rows, err := db.Query("SELECT * FROM products WHERE price > ?", minPrice)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []product
	for rows.Next() {
		var p product
		if err := rows.Scan(&p.id, &p.model, &p.company, &p.price); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func getProductByID(db *sql.DB, id int) (product, error) {
	row := db.QueryRow("SELECT * FROM products WHERE id = ?", id)
	var p product
	err := row.Scan(&p.id, &p.model, &p.company, &p.price)
	return p, err
}

func updateProductPrice(db *sql.DB, id int, price int) (int64, error) {
	result, err := db.Exec("UPDATE products SET price = ? WHERE id = ?", price, id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func deleteProduct(db *sql.DB, id int) (int64, error) {
	result, err := db.Exec("DELETE FROM products WHERE id = ?", id)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func main() {
	// Открытие соединения с базой данных
	db, err := sql.Open("sqlite", "store.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		panic(err)
	}
	fmt.Println("Таблица products готова к использованию.")

	fmt.Println("\n--- Добавление данных ---")
	lastID, rowsAffected, err := insertProduct(db, "iPhone X", "Apple", 72000)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Добавлен товар с ID: %d, затронуто строк: %d\n", lastID, rowsAffected)

	fmt.Println("\n--- Получение всех товаров ---")
	products, err := getAllProducts(db)
	if err != nil {
		panic(err)
	}
	for _, p := range products {
		fmt.Printf("ID: %d, Модель: %s, Компания: %s, Цена: %d\n", p.id, p.model, p.company, p.price)
	}

	// Получение товаров с ценой > 70 000
	fmt.Println("\n--- Товары с ценой > 70 000 ---")
	expensive, err := getProductsByMinPrice(db, 70000)
	if err != nil {
		panic(err)
	}
	for _, p := range expensive {
		fmt.Printf("ID: %d, Модель: %s, Компания: %s, Цена: %d\n", p.id, p.model, p.company, p.price)
	}

	productID := int(lastID)
	fmt.Printf("\n--- Товар с ID = %d ---\n", productID)
	prod, err := getProductByID(db, productID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("ID: %d, Модель: %s, Компания: %s, Цена: %d\n", prod.id, prod.model, prod.company, prod.price)

	fmt.Printf("\n--- Обновление данных (цена товара с ID = %d) ---\n", productID)
	rowsAffected, err = updateProductPrice(db, productID, 69000)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Обновлено строк: %d\n", rowsAffected)

	fmt.Printf("\n--- Удаление данных (товар с ID = %d) ---\n", productID)
	rowsAffected, err = deleteProduct(db, productID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Удалено строк: %d\n", rowsAffected)
}
