package schema

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/sql/schema/mysql"
	"github.com/mattbonnell/gq/internal/sql/schema/sqlite3"
	"github.com/rs/zerolog/log"
)

func getSchema(db *sqlx.DB) []string {
	switch db.DriverName() {
	case "sqlite3":
		return sqlite3.Schema
	case "mysql":
		return mysql.Schema
	default:
		panic(fmt.Sprintf("driver '%s' not supported", db.DriverName()))
	}
}

func Create(db *sqlx.DB) error {
	log.Debug().Msg("creating schema")
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %s", err)
	}
	defer tx.Rollback()
	for _, stmt := range getSchema(db) {
		_, err := tx.Exec(stmt)
		if err != nil {
			return fmt.Errorf("failed to exec stmt %s: %s", strings.Split(stmt, "(")[0], err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit tx: %s", err)
	}
	log.Debug().Msg("schema created")
	return nil
}
