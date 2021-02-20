package internal

import (
	"database/sql"
	"time"
)

type Message struct {
	ID        int64
	CreatedAt time.Time    `db:"created_at"`
	Payload   sql.RawBytes `db:"payload"`
	Retries   int32        `db:"retries"`
	ReadyAt   time.Time    `db:"ready_at"`
}
