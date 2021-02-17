package sql

import (
	"database/sql"
	"time"
)

type AutoIncr struct {
	ID      uint64
	Created time.Time
}

type Message struct {
	AutoIncr
	Status     uint8        `db:"status"`
	Payload    sql.RawBytes `db:"payload"`
	ConsumerID uint64       `db:"consumer_id"`
}

type Consumer struct {
	AutoIncr
	Updated time.Time
}
