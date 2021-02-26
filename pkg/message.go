package gq

import (
	"database/sql"

	"github.com/mattbonnell/gq/internal"
)

const (
	MessageStatusQueued uint8 = iota
	MessageStatusConsumed
	MessageStatusProcessed
	MessageStatusFailed
)

type Message struct {
	ID      int64
	Payload []byte
	retries int32
}

func FromSQL(sqlMessage internal.Message) Message {
	return Message{ID: sqlMessage.ID, Payload: []byte(sqlMessage.Payload)}
}

func (m Message) ToSQL() internal.Message {
	return internal.Message{ID: m.ID, Payload: sql.RawBytes(m.Payload)}
}
