package repository

import (
	"database/sql"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// RunMigrations applies all up migrations found in dir (filesystem path).
// dir is a directory on disk: "db/migrations" for local runs, "/db/migrations"
// inside the container (the Dockerfile copies migrations there and WORKDIR is /).
func RunMigrations(dsn, dir string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
