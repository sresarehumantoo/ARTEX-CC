package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func main() {
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	seed(db)

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/admin", handleAdmin)
	http.HandleFunc("/admin/users", handleAdminUsers)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/api/search", handleSearch)
	http.HandleFunc("/files", handleFiles)
	http.HandleFunc("/fetch", handleFetch)
	http.HandleFunc("/api/users/", handleUserByID)
	http.HandleFunc("/health", handleHealth)

	addr := ":8083"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		addr = v
	}
	slog.Info("vulnapp listening", "addr", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func seed(db *sql.DB) {
	stmts := []string{
		`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, password TEXT, role TEXT, email TEXT)`,
		`INSERT INTO users VALUES (1, 'admin', 'admin123', 'admin', 'admin@vulnapp.local')`,
		`INSERT INTO users VALUES (2, 'alice', 'password1', 'user', 'alice@vulnapp.local')`,
		`INSERT INTO users VALUES (3, 'bob', 'hunter2', 'user', 'bob@vulnapp.local')`,
		`CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, description TEXT, price REAL)`,
		`INSERT INTO products VALUES (1, 'Widget A', 'A basic widget', 9.99)`,
		`INSERT INTO products VALUES (2, 'Widget B', 'A premium widget', 19.99)`,
		`INSERT INTO products VALUES (3, 'Secret Widget', 'Internal only - API key: sk-vuln-1234567890', 0.00)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			log.Fatalf("seed: %s: %v", s, err)
		}
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>VulnApp</title></head><body>
<h1>VulnApp</h1>
<ul>
<li><a href="/admin">Admin Panel</a></li>
<li><a href="/api/search?q=widget">Product Search</a></li>
<li><a href="/login">Login</a></li>
</ul>
</body></html>`)
}

// No authentication required: open admin panel.
func handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Admin Panel</title></head><body>
<h1>Admin Panel</h1>
<p>Welcome, administrator. No authentication required.</p>
<ul>
<li><a href="/admin/users">User Management</a></li>
<li><a href="/api/search?q=">Product Search</a></li>
<li><a href="/files?path=data/config.txt">Configuration Files</a></li>
</ul>
</body></html>`)
}

func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, username, role, email FROM users")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, "[")
	first := true
	for rows.Next() {
		var id int
		var username, role, email string
		rows.Scan(&id, &username, &role, &email)
		if !first {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `{"id":%d,"username":%q,"role":%q,"email":%q}`, id, username, role, email)
		first = false
	}
	fmt.Fprint(w, "]")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Login</title></head><body>
<h1>Login</h1>
<form method="POST" action="/login">
<label>Username: <input name="username"></label><br>
<label>Password: <input name="password" type="password"></label><br>
<button type="submit">Login</button>
</form>
</body></html>`)
		return
	}
	username := r.FormValue("username")
	password := r.FormValue("password")
	var id int
	// SQL injection in login (concatenation instead of parameterized query).
	query := fmt.Sprintf("SELECT id FROM users WHERE username='%s' AND password='%s'", username, password)
	err := db.QueryRow(query).Scan(&id)
	if err != nil {
		http.Error(w, "Invalid credentials", 401)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","user_id":%d,"token":"static-session-token-12345"}`, id)
}

// SQL injection: query parameter directly interpolated.
func handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	query := fmt.Sprintf("SELECT id, name, description, price FROM products WHERE name LIKE '%%%s%%' OR description LIKE '%%%s%%'", q, q)
	rows, err := db.Query(query)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), 500)
		return
	}
	defer rows.Close()
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"results":[`)
	first := true
	for rows.Next() {
		var id int
		var name, description string
		var price float64
		rows.Scan(&id, &name, &description, &price)
		if !first {
			fmt.Fprint(w, ",")
		}
		fmt.Fprintf(w, `{"id":%d,"name":%q,"description":%q,"price":%.2f}`, id, name, description, price)
		first = false
	}
	fmt.Fprint(w, "]}")
}

// Directory traversal: path parameter used directly with filepath.Join but no
// sanitization, so ../../etc/passwd works.
func handleFiles(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		http.Error(w, `{"error":"path parameter required"}`, 400)
		return
	}
	base := "/app/data"
	if v := os.Getenv("DATA_DIR"); v != "" {
		base = v
	}
	full := filepath.Join(base, p)
	data, err := os.ReadFile(full)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"file not found: %s"}`, p), 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write(data)
}

// SSRF: fetches arbitrary URLs from the server side.
func handleFetch(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, `{"error":"url parameter required"}`, 400)
		return
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		http.Error(w, `{"error":"url must start with http:// or https://"}`, 400)
		return
	}
	resp, err := http.Get(target)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"fetch failed: %s"}`, err.Error()), 502)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// IDOR: no authorization check, any user ID returns full data including password.
func handleUserByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/users/")
	var id int
	var username, password, role, email string
	err := db.QueryRow("SELECT id, username, password, role, email FROM users WHERE id = ?", idStr).
		Scan(&id, &username, &password, &role, &email)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"id":%d,"username":%q,"password":%q,"role":%q,"email":%q}`,
		id, username, password, role, email)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status":"ok"}`)
}
