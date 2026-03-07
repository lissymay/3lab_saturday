package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

// productResponse — тип для JSON-ответов API (поля экспортируются).
type productResponse struct {
	ID      int    `json:"id"`
	Model   string `json:"model"`
	Company string `json:"company"`
	Price   int    `json:"price"`
}

// createProductRequest — тело запроса на создание товара.
type createProductRequest struct {
	Model   string `json:"model"`
	Company string `json:"company"`
	Price   int    `json:"price"`
}

func productToResponse(p product) productResponse {
	return productResponse{ID: p.id, Model: p.model, Company: p.company, Price: p.price}
}

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

// server хранит БД для HTTP-обработчиков.
type server struct {
	db *sql.DB
}

func (s *server) handleProducts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/products")
	path = strings.Trim(path, "/")

	switch r.Method {
	case http.MethodGet:
		if path == "" {
			s.handleGetAll(w)
			return
		}
		id, err := strconv.Atoi(path)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		s.handleGetByID(w, id)
		return
	case http.MethodPost:
		if path != "" {
			http.Error(w, `{"error":"POST only to /products"}`, http.StatusBadRequest)
			return
		}
		s.handleCreate(w, r)
		return
	case http.MethodDelete:
		if path == "" {
			http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(path)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		s.handleDelete(w, id)
		return
	default:
		http.Error(w, "", http.StatusMethodNotAllowed)
		return
	}
}

func (s *server) handleGetAll(w http.ResponseWriter) {
	products, err := getAllProducts(s.db)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	resp := make([]productResponse, 0, len(products))
	for _, p := range products {
		resp = append(resp, productToResponse(p))
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) handleGetByID(w http.ResponseWriter, id int) {
	prod, err := getProductByID(s.db, id)
	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(productToResponse(prod))
}

func (s *server) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	lastID, _, err := insertProduct(s.db, req.Model, req.Company, req.Price)
	if err != nil {
		http.Error(w, `{"error":"insert failed"}`, http.StatusInternalServerError)
		return
	}
	prod, _ := getProductByID(s.db, int(lastID))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(productToResponse(prod))
}

func (s *server) handleDelete(w http.ResponseWriter, id int) {
	rows, err := deleteProduct(s.db, id)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

const indexHTML = `<!DOCTYPE html>
<html lang="ru">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>Магазин товаров</title>
	<style>
		* { box-sizing: border-box; }
		body { font-family: 'Segoe UI', system-ui, sans-serif; margin: 0; padding: 20px; background: #1a1a2e; color: #eee; min-height: 100vh; }
		.container { max-width: 900px; margin: 0 auto; }
		h1 { margin: 0 0 24px; font-size: 1.75rem; color: #e94560; }
		.tabs { display: flex; gap: 4px; margin-bottom: 20px; border-bottom: 1px solid #333; }
		.tabs button { padding: 12px 20px; border: none; background: transparent; color: #aaa; cursor: pointer; font-size: 1rem; border-bottom: 2px solid transparent; }
		.tabs button:hover { color: #fff; }
		.tabs button.active { color: #e94560; border-bottom-color: #e94560; }
		.panel { display: none; padding: 20px; background: #16213e; border-radius: 8px; }
		.panel.active { display: block; }
		table { width: 100%; border-collapse: collapse; }
		th, td { padding: 10px 12px; text-align: left; border-bottom: 1px solid #333; }
		th { color: #e94560; }
		.btn { padding: 8px 16px; border: none; border-radius: 6px; cursor: pointer; font-size: 0.9rem; }
		.btn-primary { background: #e94560; color: #fff; }
		.btn-primary:hover { background: #c73e54; }
		.btn-danger { background: #c73e54; color: #fff; padding: 6px 12px; font-size: 0.85rem; }
		.btn-danger:hover { background: #a02d3e; }
		input, label { display: block; margin-bottom: 10px; }
		input { width: 100%; max-width: 320px; padding: 10px; border: 1px solid #333; border-radius: 6px; background: #0f3460; color: #eee; font-size: 1rem; }
		label { color: #aaa; margin-bottom: 4px; }
		#findResult { margin-top: 12px; padding: 12px; background: #0f3460; border-radius: 6px; min-height: 40px; }
		.message { padding: 10px; border-radius: 6px; margin-top: 10px; }
		.message.err { background: #5a1a1a; color: #f88; }
		.message.ok { background: #1a3a2e; color: #8f8; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Магазин товаров</h1>
		<div class="tabs">
			<button type="button" class="active" data-tab="list">Список товаров</button>
			<button type="button" data-tab="find">Найти по ID</button>
			<button type="button" data-tab="create">Создать товар</button>
		</div>
		<div id="panel-list" class="panel active">
			<button type="button" class="btn btn-primary" id="refreshList">Обновить список</button>
			<div id="listContainer" style="margin-top: 16px;"></div>
		</div>
		<div id="panel-find" class="panel">
			<label for="findId">ID товара</label>
			<input type="number" id="findId" placeholder="Например: 1" min="1">
			<button type="button" class="btn btn-primary" id="btnFind" style="margin-top: 10px;">Найти</button>
			<div id="findResult"></div>
		</div>
		<div id="panel-create" class="panel">
			<label for="createModel">Модель</label>
			<input type="text" id="createModel" placeholder="Например: iPhone X">
			<label for="createCompany">Компания</label>
			<input type="text" id="createCompany" placeholder="Например: Apple">
			<label for="createPrice">Цена</label>
			<input type="number" id="createPrice" placeholder="Например: 72000" min="0">
			<button type="button" class="btn btn-primary" id="btnCreate" style="margin-top: 12px;">Создать</button>
			<div id="createMessage"></div>
		</div>
	</div>
	<script>
		const API = '/products';
		function qs(s) { return document.querySelector(s); }
		function qsa(s) { return document.querySelectorAll(s); }
		qsa('.tabs button').forEach(btn => {
			btn.addEventListener('click', () => {
				qsa('.tabs button').forEach(b => b.classList.remove('active'));
				qsa('.panel').forEach(p => p.classList.remove('active'));
				btn.classList.add('active');
				qs('#panel-' + btn.dataset.tab).classList.add('active');
				if (btn.dataset.tab === 'list') loadList();
			});
		});
		function loadList() {
			fetch(API).then(r => r.json()).then(data => {
				const el = qs('#listContainer');
				if (!data.length) { el.innerHTML = '<p>Товаров пока нет.</p>'; return; }
				el.innerHTML = '<table><thead><tr><th>ID</th><th>Модель</th><th>Компания</th><th>Цена</th><th></th></tr></thead><tbody></tbody></table>';
				const tbody = el.querySelector('tbody');
				data.forEach(p => {
					const tr = document.createElement('tr');
					tr.innerHTML = '<td>' + p.id + '</td><td>' + p.model + '</td><td>' + p.company + '</td><td>' + p.price + '</td><td><button type="button" class="btn btn-danger btn-delete" data-id="' + p.id + '">Удалить</button></td>';
					tbody.appendChild(tr);
				});
				el.querySelectorAll('.btn-delete').forEach(b => {
					b.addEventListener('click', () => {
						if (!confirm('Удалить товар ID ' + b.dataset.id + '?')) return;
						fetch(API + '/' + b.dataset.id, { method: 'DELETE' }).then(r => {
							if (r.ok) loadList();
							else r.json().then(j => alert(j.error || 'Ошибка'));
						});
					});
				});
			}).catch(e => { qs('#listContainer').innerHTML = '<p class="message err">Ошибка загрузки: ' + e.message + '</p>'; });
		}
		qs('#refreshList').addEventListener('click', loadList);
		qs('#btnFind').addEventListener('click', () => {
			const id = qs('#findId').value.trim();
			if (!id) { qs('#findResult').innerHTML = '<span class="message err">Введите ID</span>'; return; }
			fetch(API + '/' + id).then(r => {
				if (!r.ok) return r.json().then(j => { qs('#findResult').innerHTML = '<span class="message err">' + (j.error || r.status) + '</span>'; });
				return r.json().then(p => {
					qs('#findResult').innerHTML = '<strong>ID:</strong> ' + p.id + ', <strong>Модель:</strong> ' + p.model + ', <strong>Компания:</strong> ' + p.company + ', <strong>Цена:</strong> ' + p.price;
				});
			}).catch(e => { qs('#findResult').innerHTML = '<span class="message err">' + e.message + '</span>'; });
		});
		qs('#btnCreate').addEventListener('click', () => {
			const model = qs('#createModel').value.trim();
			const company = qs('#createCompany').value.trim();
			const price = parseInt(qs('#createPrice').value, 10);
			if (!model || !company || isNaN(price) || price < 0) {
				qs('#createMessage').innerHTML = '<span class="message err">Заполните все поля, цена — число ≥ 0</span>';
				return;
			}
			fetch(API, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ model, company, price }) })
				.then(r => r.json().then(data => {
					if (r.ok) {
						qs('#createMessage').innerHTML = '<span class="message ok">Создан товар ID ' + data.id + '</span>';
						qs('#createModel').value = qs('#createCompany').value = qs('#createPrice').value = '';
					} else qs('#createMessage').innerHTML = '<span class="message err">' + (data.error || 'Ошибка') + '</span>';
				})).catch(e => { qs('#createMessage').innerHTML = '<span class="message err">' + e.message + '</span>'; });
		});
		loadList();
	</script>
</body>
</html>
`

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func runServer(db *sql.DB) {
	srv := &server{db: db}
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/products", srv.handleProducts)
	http.HandleFunc("/products/", srv.handleProducts)
	addr := ":8080"
	fmt.Printf("Сервер запущен на http://localhost%s\n", addr)
	fmt.Println("  GET    /products     — список товаров")
	fmt.Println("  GET    /products/:id — товар по ID")
	fmt.Println("  POST   /products     — создать товар (body: {\"model\",\"company\",\"price\"})")
	fmt.Println("  DELETE /products/:id — удалить товар")
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка сервера: %v\n", err)
		os.Exit(1)
	}
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

	// Запуск HTTP-сервера при флаге -serve
	for _, arg := range os.Args[1:] {
		if arg == "-serve" {
			runServer(db)
			return
		}
	}

	// Демонстрация работы с БД (как раньше)
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
