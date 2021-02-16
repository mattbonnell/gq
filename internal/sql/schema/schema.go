package schema

import (
	"fmt"

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

func CreateSchema(db *sqlx.DB) {
	log.Debug().Msg("creating schema")
	for _, stmt := range getSchema(db) {
		db.MustExec(stmt)
	}
	log.Debug().Msg("schema created")
}
