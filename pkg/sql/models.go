package sql

import (
	"database/sql"
	"time"

	"github.com/jmoiron/sqlx"
)

type AutoIncr struct {
	ID      uint64
	Created time.Time
	Updated time.Time
}

type Message struct {
	AutoIncr
	Status     uint8        `db:"status"`
	Payload    sql.RawBytes `db:"payload"`
	ConsumerID uint64       `db:"consumer_id"`
}

type Consumer struct {
	AutoIncr
}

func CreateTables(db *sqlx.DB, tableSchemas []string) {
	for _, table := range tableSchemas {
		db.MustExec(table)
	}
}
