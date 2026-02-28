package main

import (
	"bytes"
	"database/sql"
	"os"
	"strings"
	"sync"
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

func TestCreateProductsTable_Error(t *testing.T) {
	// Создаём некорректное соединение (не существующая БД)
	db, err := sql.Open("sqlite", "invalid://path")
	if err != nil {
		t.Skipf("не удалось создать БД для теста ошибки: %v", err) // Исправлено: Skip → Skipf
	}
	defer db.Close()

	err = createProductsTable(db)
	if err == nil {
		t.Fatal("ожидали ошибку при создании таблицы, но её не было")
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

func TestInsertProduct_Error(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Таблица не создана — должна быть ошибка
	_, _, err := insertProduct(db, "Test", "TestCo", 1000)
	if err == nil {
		t.Fatal("ожидали ошибку вставки в несуществующую таблицу")
	}

	// Создаём таблицу и тестируем некорректную цену (отрицательная)
	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	_, _, err = insertProduct(db, "Test", "TestCo", -100)
	// В текущей реализации отрицательная цена допустима, но тест показывает, как проверять граничные случаи
	if err != nil {
		t.Logf("получили ошибку при отрицательной цене: %v (в текущей реализации это допустимо)", err)
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

func TestGetAllProducts_Error(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	// Таблица не создана
	_, err := getAllProducts(db)
	if err == nil {
		t.Fatal("ожидали ошибку чтения из несуществующей таблицы")
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

	// Цена > 60 000: только iPhone X и Galaxy S21
	expensive, err := getProductsByMinPrice(db, 60000)
	if err != nil {
		t.Fatalf("getProductsByMinPrice: %v", err)
	}
	if len(expensive) != 2 {
		t.Errorf("ожидали 2 товара с ценой > 60 000, получили %d", len(expensive))
	}

	// Цена > 70 000: только iPhone X
	veryExpensive, err := getProductsByMinPrice(db, 70000)
	if err != nil {
		t.Fatalf("getProductsByMinPrice(70000): %v", err)
	}
	if len(veryExpensive) != 1 {
		t.Errorf("ожидали 1 товар с ценой > 70 000, получили %d", len(veryExpensive))
	}
	if veryExpensive[0].model != "iPhone X" {
		t.Errorf("ожидали модель iPhone X, получили %s", veryExpensive[0].model)
	}

	// Тест с пустой таблицей — создаём новую БД без записей
	emptyDB := openTestDB(t)
	defer emptyDB.Close()
	if err := createProductsTable(emptyDB); err != nil {
		t.Fatal(err)
	}
	empty, err := getProductsByMinPrice(emptyDB, 50000)
	if err != nil {
		t.Fatalf("getProductsByMinPrice для пустой таблицы: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("для пустой таблицы ожидали 0 товаров, получили %d", len(empty))
	}

	// Тест с нулевой ценой
	zeroPrice, err := getProductsByMinPrice(db, 0)
	if err != nil {
		t.Fatalf("getProductsByMinPrice с minPrice=0: %v", err)
	}
	if len(zeroPrice) != 3 {
		t.Errorf("с minPrice=0 ожидали все товары (3), получили %d", len(zeroPrice))
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

	// Несуществующий ID
	_, err = getProductByID(db, 999)
	if err != sql.ErrNoRows {
		t.Errorf("ожидали sql.ErrNoRows для несуществующего ID, получили %v", err)
	}
}

func TestGetProductByID_Error(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}

	// Закрываем соединение, чтобы вызвать ошибку БД
	db.Close()
	_, err := getProductByID(db, 1)
	if err == nil {
		t.Fatal("ожидали ошибку БД после закрытия соединения")
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

func TestUpdateProductPrice_Error(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Таблица не создана
	_, err := updateProductPrice(db, 1, 50000)
	if err == nil {
		t.Fatal("ожидали ошибку обновления в несуществующей таблице")
	}

	// Создаём таблицу, но обновляем несуществующий ID
	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	rows, err := updateProductPrice(db, 999, 50000)
	if err != nil {
		t.Logf("получили ошибку при обновлении несуществующего ID: %v (это допустимо)", err)
	}
	if rows != 0 {
		t.Errorf("ожидали 0 обновлённых строк для несуществующего ID, получили %d", rows)
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

func TestDeleteProduct_Error(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Таблица не создана
	_, err := deleteProduct(db, 1)
	if err == nil {
		t.Fatal("ожидали ошибку удаления из несуществующей таблицы")
	}

	// Создаём таблицу, но удаляем несуществующий ID
	if err := createProductsTable(db); err != nil {
		t.Fatal(err)
	}
	rows, err := deleteProduct(db, 999)
	if err != nil {
		t.Logf("получили ошибку при удалении несуществующего ID: %v (это допустимо)", err)
	}
	if rows != 0 {
		t.Errorf("ожидали 0 удалённых строк для несуществующего ID, получили %d", rows)
	}
}

// Тест для функции main()
func TestMain(t *testing.T) {
	// Сохраняем оригинальные os.Stdout и os.Args
	oldStdout := os.Stdout
	oldArgs := os.Args

	// Перехватываем вывод
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Имитируем аргументы командной строки
	os.Args = []string{"program"}

	var wg sync.WaitGroup
	wg.Add(1)

	// Запускаем main в отдельной горутине, чтобы перехватить panic
	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Logf("main() завершился с panic: %v", r)
			}
			wg.Done()
			done <- true
		}()
		main()
	}()

	// Ждём завершения горутины
	wg.Wait()

	// Закрываем запись, читаем вывод
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Восстанавливаем состояние
	os.Stdout = oldStdout
	os.Args = oldArgs
	<-done

	// Проверяем ключевые фразы в выводе
	expected := []string{
		"Таблица products готова к использованию",
		"Добавлен товар с ID",
		"Получение всех товаров",
		"Товары с ценой > 70 000", // Неразрывный пробел после 70
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("в выводе main() отсутствует ожидаемая строка: %s", exp)
		}
	}
}
