package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var files embed.FS

func Up(db *sql.DB) error {
	goose.SetBaseFS(files)
	if err := goose.SetDialect("mysql"); err != nil {
		return err
	}
	return goose.Up(db, ".")
}
