package main

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("не удалось открыть тестовую БД: %v", err)
	}
	return db
}

func TestCreateProductsTable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	err := createProductsTable(db)
	if err != nil {
		t.Fatalf("createProductsTable: %v", err)
	}

	// Проверяем, что таблица создана — можно выполнить запрос
	_, err = db.Exec("INSERT INTO products (model, company, price) VALUES (?, ?, ?)", "Test", "TestCo", 1000)
	if err != nil {
		t.Fatalf("вставка в созданную таблицу: %v", err)
	}
}

func TestInsertProduct(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}

	lastID, rowsAffected, err := insertProduct(db, "iPhone X", "Apple", 72000)
	if err != nil {
		t.Fatalf("insertProduct: %v", err)
	}
	if lastID != 1 {
		t.Errorf("ожидали lastID = 1, получили %d", lastID)
	}
	if rowsAffected != 1 {
		t.Errorf("ожидали rowsAffected = 1, получили %d", rowsAffected)
	}

	lastID2, _, err := insertProduct(db, "Galaxy S21", "Samsung", 65000)
	if err != nil {
		t.Fatalf("второй insertProduct: %v", err)
	}
	if lastID2 != 2 {
		t.Errorf("ожидали lastID = 2, получили %d", lastID2)
	}
}

func TestGetAllProducts(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	_, _, _ = insertProduct(db, "iPhone X", "Apple", 72000)
	_, _, _ = insertProduct(db, "Galaxy S21", "Samsung", 65000)

	products, err := getAllProducts(db)
	if err != nil {
		t.Fatalf("getAllProducts: %v", err)
	}
	if len(products) != 2 {
		t.Fatalf("ожидали 2 товара, получили %d", len(products))
	}

	if products[0].model != "iPhone X" || products[0].company != "Apple" || products[0].price != 72000 {
		t.Errorf("первый товар: ожидали iPhone X / Apple / 72000, получили %s / %s / %d", products[0].model, products[0].company, products[0].price)
	}
	if products[1].model != "Galaxy S21" || products[1].company != "Samsung" || products[1].price != 65000 {
		t.Errorf("второй товар: ожидали Galaxy S21 / Samsung / 65000, получили %s / %s / %d", products[1].model, products[1].company, products[1].price)
	}
}

func TestGetProductsByMinPrice(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	_, _, _ = insertProduct(db, "iPhone X", "Apple", 72000)
	_, _, _ = insertProduct(db, "Galaxy S21", "Samsung", 65000)
	_, _, _ = insertProduct(db, "Pixel 6", "Google", 55000)

	// Цена > 60000: только iPhone X и Galaxy S21
	expensive, err := getProductsByMinPrice(db, 60000)
	if err != nil {
		t.Fatalf("getProductsByMinPrice: %v", err)
	}
	if len(expensive) != 2 {
		t.Errorf("ожидали 2 товара с ценой > 60000, получили %d", len(expensive))
	}

	// Цена > 70000: только iPhone X
	veryExpensive, err := getProductsByMinPrice(db, 70000)
	if err != nil {
		t.Fatalf("getProductsByMinPrice(70000): %v", err)
	}
	if len(veryExpensive) != 1 {
		t.Errorf("ожидали 1 товар с ценой > 70000, получили %d", len(veryExpensive))
	}
	if veryExpensive[0].model != "iPhone X" {
		t.Errorf("ожидали модель iPhone X, получили %s", veryExpensive[0].model)
	}
}

func TestGetProductByID(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	_, _, _ = insertProduct(db, "iPhone X", "Apple", 72000)
	_, _, _ = insertProduct(db, "Galaxy S21", "Samsung", 65000)

	prod, err := getProductByID(db, 1)
	if err != nil {
		t.Fatalf("getProductByID(1): %v", err)
	}
	if prod.id != 1 || prod.model != "iPhone X" || prod.company != "Apple" || prod.price != 72000 {
		t.Errorf("ожидали id=1, iPhone X, Apple, 72000; получили id=%d, %s, %s, %d", prod.id, prod.model, prod.company, prod.price)
	}

	prod2, err := getProductByID(db, 2)
	if err != nil {
		t.Fatalf("getProductByID(2): %v", err)
	}
	if prod2.model != "Galaxy S21" {
		t.Errorf("ожидали Galaxy S21, получили %s", prod2.model)
	}

	_, err = getProductByID(db, 999)
	if err != sql.ErrNoRows {
		t.Errorf("ожидали sql.ErrNoRows для несуществующего ID, получили %v", err)
	}
}

func TestUpdateProductPrice(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	_, _, _ = insertProduct(db, "iPhone X", "Apple", 72000)

	rows, err := updateProductPrice(db, 1, 69000)
	if err != nil {
		t.Fatalf("updateProductPrice: %v", err)
	}
	if rows != 1 {
		t.Errorf("ожидали 1 обновлённую строку, получили %d", rows)
	}

	prod, _ := getProductByID(db, 1)
	if prod.price != 69000 {
		t.Errorf("ожидали цену 69000, получили %d", prod.price)
	}
}

func TestDeleteProduct(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	_, _, _ = insertProduct(db, "iPhone X", "Apple", 72000)
	_, _, _ = insertProduct(db, "Galaxy S21", "Samsung", 65000)

	rows, err := deleteProduct(db, 1)
	if err != nil {
		t.Fatalf("deleteProduct: %v", err)
	}
	if rows != 1 {
		t.Errorf("ожидали 1 удалённую строку, получили %d", rows)
	}

	products, _ := getAllProducts(db)
	if len(products) != 1 {
		t.Fatalf("после удаления ожидали 1 товар, получили %d", len(products))
	}
	if products[0].model != "Galaxy S21" {
		t.Errorf("остался не тот товар: %s", products[0].model)
	}

	_, err = getProductByID(db, 1)
	if err != sql.ErrNoRows {
		t.Errorf("удалённый товар не должен находиться по ID: %v", err)
	}
}
