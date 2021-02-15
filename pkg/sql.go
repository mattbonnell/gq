package gq

import (
	"database/sql"
	"strconv"
	"time"
)

var schema = `
CREATE TABLE message (
    	uuid text,
    	status integer,
	consumer_uuid text
);

CREATE TABLE place (
    country text,
    city text NULL,
    telcode integer
)`

type MessageStatus int

const (
	MessageStatusQueued MessageStatus = iota
	MessageStatusConsumed
	MessageStatusProcessed
	MessageStatusFailed
)

func (m MessageStatus) String() string {
	switch m {
	case MessageStatusQueued:
		return "Queued"
	case MessageStatusConsumed:
		return "Consumed"
	case MessageStatusProcessed:
		return "Processed"
	case MessageStatusFailed:
		return "Failed"
	default:
		return "MessageStatus(" + strconv.Itoa(int(m)) + ")"
	}
}

type Message struct {
	UUID       string         `db:"uuid"`
	Status     int            `db:"status"`
	WorkerUUID sql.NullString `db:"worker_uuid"`
}

type Consumer struct {
	UUID      string    `db:"uuid"`
	CreatedAt time.Time `db:"created_at"`
}
