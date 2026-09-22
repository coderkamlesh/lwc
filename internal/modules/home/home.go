package home

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"example.com/go-lambda-monolith/internal/platform/web"
)

type Module struct {
	renderer *web.Renderer
}

type PageData struct {
	Title  string
	Active string
}

func NewModule(renderer *web.Renderer) *Module {
	return &Module{renderer: renderer}
}

func (m *Module) Routes(r chi.Router) {
	r.Get("/", m.page)
}

func (m *Module) page(w http.ResponseWriter, _ *http.Request) {
	data := PageData{
		Title:  "Modular Go on Lambda",
		Active: "home",
	}
	if err := m.renderer.Render(w, "home", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
