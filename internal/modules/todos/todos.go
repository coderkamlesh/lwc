package todos

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"example.com/go-lambda-monolith/internal/platform/web"
)

type Module struct {
	db       *sql.DB
	renderer *web.Renderer
}

type Todo struct {
	ID        int64
	Title     string
	Completed bool
	CreatedAt string
}

type PageData struct {
	Title  string
	Active string
	Todos  []Todo
}

func NewModule(db *sql.DB, renderer *web.Renderer) *Module {
	return &Module{db: db, renderer: renderer}
}

func (m *Module) Routes(r chi.Router) {
	r.Route("/todos", func(r chi.Router) {
		r.Get("/", m.list)
		r.Post("/", m.create)
		r.Post("/{id}/toggle", m.toggle)
	})
}

func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS todos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			completed INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)
	`)
	return err
}

func (m *Module) list(w http.ResponseWriter, r *http.Request) {
	todos, err := m.findAll(r.Context())
	if err != nil {
		http.Error(w, "could not load todos", http.StatusInternalServerError)
		return
	}

	data := PageData{
		Title:  "Todos | Modular Go",
		Active: "todos",
		Todos:  todos,
	}
	if err := m.renderPage(w, r, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (m *Module) create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" || len(title) > 200 {
		http.Error(w, "title must contain between 1 and 200 characters", http.StatusBadRequest)
		return
	}

	_, err := m.db.ExecContext(
		r.Context(),
		"INSERT INTO todos (title, completed, created_at) VALUES (?, 0, ?)",
		title,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		http.Error(w, "could not create todo", http.StatusInternalServerError)
		return
	}

	if !isHTMXRequest(r) {
		http.Redirect(w, r, "/todos/", http.StatusSeeOther)
		return
	}

	m.renderList(w, r)
}

func (m *Module) toggle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id < 1 {
		http.Error(w, "invalid todo id", http.StatusBadRequest)
		return
	}

	result, err := m.db.ExecContext(
		r.Context(),
		"UPDATE todos SET completed = CASE completed WHEN 0 THEN 1 ELSE 0 END WHERE id = ?",
		id,
	)
	if err != nil {
		http.Error(w, "could not update todo", http.StatusInternalServerError)
		return
	}

	if affected, err := result.RowsAffected(); err != nil || affected == 0 {
		http.NotFound(w, r)
		return
	}

	if !isHTMXRequest(r) {
		http.Redirect(w, r, "/todos/", http.StatusSeeOther)
		return
	}

	m.renderList(w, r)
}

func (m *Module) renderPage(w http.ResponseWriter, r *http.Request, data PageData) error {
	if isHTMXRequest(r) {
		return m.renderer.RenderPartial(w, "todos", "todo-list", data)
	}
	return m.renderer.Render(w, "todos", data)
}

func (m *Module) renderList(w http.ResponseWriter, r *http.Request) {
	todos, err := m.findAll(r.Context())
	if err != nil {
		http.Error(w, "could not load todos", http.StatusInternalServerError)
		return
	}

	data := PageData{Title: "Todos | Modular Go", Active: "todos", Todos: todos}
	if err := m.renderer.RenderPartial(w, "todos", "todo-list", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (m *Module) findAll(ctx context.Context) ([]Todo, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT id, title, completed, created_at
		FROM todos
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Todo, 0, 16)
	for rows.Next() {
		var (
			item      Todo
			completed int64
		)
		if err := rows.Scan(&item.ID, &item.Title, &completed, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Completed = completed != 0
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
