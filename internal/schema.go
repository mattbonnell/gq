package internal

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/drivers/mysql"
	"github.com/mattbonnell/gq/internal/drivers/postgres"
	"github.com/rs/zerolog/log"
)

func getSchema(driverName string) ([]string, error) {
	switch driverName {
	case "mysql":
		return mysql.Schema, nil
	case "pq":
		fallthrough
	case "pqx":
		fallthrough
	case "postgres":
		return postgres.Schema, nil
	default:
		return nil, fmt.Errorf("driver '%s' not supported", driverName)
	}
}

func CreateSchema(db *sqlx.DB) error {
	log.Debug().Msg("creating schema")
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin tx: %s", err)
	}
	defer tx.Rollback()
	schema, err := getSchema(db.DriverName())
	if err != nil {
		return fmt.Errorf("error retrieving schema: %s", err)
	}
	for _, stmt := range schema {
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
