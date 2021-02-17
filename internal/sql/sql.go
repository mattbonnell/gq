package sql

import (
	"database/sql"
	"time"
)

type AutoIncr struct {
	ID      int64
	Created time.Time
}

type Message struct {
	AutoIncr
	Status     uint8        `db:"status"`
	Payload    sql.RawBytes `db:"payload"`
	ConsumerID int64        `db:"consumer_id"`
}

type Consumer struct {
	AutoIncr
	Updated time.Time
}
