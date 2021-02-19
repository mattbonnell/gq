package client

import (
	go_sql "database/sql"
	"errors"

	"github.com/mattbonnell/gq/internal/sql"
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

func FromSQL(sqlMessage *sql.Message) (*Message, error) {
	p := []byte(sqlMessage.Payload)
	msg := Message{Payload: make([]byte, len(p))}
	if n := copy(msg.Payload, p); n < len(p) {
		return nil, errors.New("failed to copy the message's full payload")
	}
	msg.ID = sqlMessage.ID
	return &msg, nil
}

func (m Message) ToSQL() *sql.Message {
	sqlMessage := &sql.Message{Payload: go_sql.RawBytes(m.Payload)}
	sqlMessage.ID = m.ID
	return sqlMessage
}
