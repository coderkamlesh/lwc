package database

import (
	"database/sql"

	turso "turso.tech/database/tursogo-serverless"

	"example.com/go-lambda-monolith/internal/platform/config"
)

func Open(cfg config.Config) *sql.DB {
	db := sql.OpenDB(turso.NewConnector(cfg.TursoDatabaseURL, cfg.TursoAuthToken))
	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnLifetime)
	return db
}
