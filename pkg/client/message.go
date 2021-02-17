package client

import (
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
	ID      uint64
	Payload []byte
}

func FromSQL(m *sql.Message) (*Message, error) {
	p := []byte(m.Payload)
	msg := Message{Payload: make([]byte, len(p))}
	if n := copy(msg.Payload, p); n < len(p) {
		return nil, errors.New("failed to copy the message's full payload")
	}
	msg.ID = m.ID
	return &msg, nil
}
