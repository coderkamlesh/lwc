package health

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Module struct {
	db *sql.DB
}

func NewModule(db *sql.DB) *Module {
	return &Module{db: db}
}

func (m *Module) Routes(r chi.Router) {
	r.Get("/healthz", m.health)
	r.Get("/readyz", m.ready)
}

func (m *Module) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

func (m *Module) ready(w http.ResponseWriter, r *http.Request) {
	if err := m.db.PingContext(r.Context()); err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ready\n"))
}
