package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"example.com/go-lambda-monolith/internal/modules/health"
	"example.com/go-lambda-monolith/internal/modules/home"
	"example.com/go-lambda-monolith/internal/modules/todos"
	"example.com/go-lambda-monolith/internal/platform/config"
	"example.com/go-lambda-monolith/internal/platform/database"
	"example.com/go-lambda-monolith/internal/platform/httpserver"
	"example.com/go-lambda-monolith/internal/platform/web"
)

// App owns process-wide dependencies and the application HTTP handler.
// Feature modules are wired here so they stay independently reusable.
type App struct {
	db      *sql.DB
	handler http.Handler
}

func New(ctx context.Context, cfg config.Config) (*App, error) {
	renderer, err := web.NewRenderer()
	if err != nil {
		return nil, fmt.Errorf("create renderer: %w", err)
	}

	db := database.Open(cfg)

	if err := todos.Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run todos migrations: %w", err)
	}

	homeModule := home.NewModule(renderer)
	todosModule := todos.NewModule(db, renderer)
	healthModule := health.NewModule(db)

	handler := httpserver.NewRouter(
		web.StaticFS(),
		cfg.RequestTimeout,
		homeModule.Routes,
		todosModule.Routes,
		healthModule.Routes,
	)

	return &App{db: db, handler: handler}, nil
}

func (a *App) Handler() http.Handler {
	return a.handler
}

func (a *App) Close() error {
	if a.db == nil {
		return nil
	}
	return a.db.Close()
}
