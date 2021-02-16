package schema

import (
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/schema/sqlite3"
)

func getSchema(db *sqlx.DB) []string {
	switch db.DriverName() {
	case "sqlite3":
		return sqlite3.Schema
	}
}

func CreateSchema(db *sqlx.DB) {
}
