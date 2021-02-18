package sql

import (
	"database/sql"
	"time"
)

type Message struct {
	ID               int64
	Created          time.Time
	Payload          sql.RawBytes `db:"payload"`
	Retries          int32        `db:"retries"`
	RetryBackoffEnds time.Time    `db:"retry_backoff_ends"`
}
