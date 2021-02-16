package client

import (
	"errors"

	"github.com/mattbonnell/gq/internal/sql"
)

type Message interface {
	ID() uint64
	Payload() []byte
}

type message struct {
	id      uint64
	payload []byte
}

func (m message) ID() uint64 {
	return m.id
}

func (m message) Payload() []byte {
	return m.payload
}

func FromSQL(m *sql.Message) (Message, error) {
	p := []byte(m.Payload)
	msg := message{payload: make([]byte, len(p))}
	if n := copy(msg.payload, p); n < len(p) {
		return nil, errors.New("failed to copy the message's full payload")
	}
	msg.id = m.ID
	return &msg, nil
}
