package pkg

import (
	"database/sql"
	"errors"

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

func FromSQL(sqlMessage *internal.Message) (*Message, error) {
	p := []byte(sqlMessage.Payload)
	msg := Message{ID: sqlMessage.ID, Payload: make([]byte, len(p))}
	if n := copy(msg.Payload, p); n < len(p) {
		return nil, errors.New("failed to copy the message's full payload")
	}
	return &msg, nil
}

func (m Message) ToSQL() *internal.Message {
	sqlMessage := &internal.Message{ID: m.ID, Payload: sql.RawBytes(m.Payload)}
	return sqlMessage
}
