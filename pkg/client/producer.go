package client

import (
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const (
	messageCacheSize = 50
	//TODO implement push batches
	pushBatchSize = 1
)

type Producer struct {
	db       *sqlx.DB
	msgChan  chan *Message
	msgCache []*Message
}

func newProducer(db *sqlx.DB) (*Producer, error) {
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("couldn't reach database: %s", err)
	}
	p := &Producer{db: db, msgChan: make(chan *Message), msgCache: make([]*Message, 0, messageCacheSize)}
	go p.startPushingMessages()
	return p, nil
}

func (p *Producer) Push(m *Message) {
	p.msgCache = append(p.msgCache, m)
	if len(p.msgCache) == pushBatchSize {
		p.drainMessageCache()
	}
}

func (p *Producer) drainMessageCache() {
	for _, m := range p.msgCache {
		p.msgChan <- m
	}
}

func (p *Producer) startPushingMessages() {
	for {
		msgCache := make([]*Message, 0, messageCacheSize)
		select {
		case m := <-p.msgChan:
			msgCache = append(msgCache, m)
			if len(msgCache) == messageCacheSize {
				break
			}

		}
		if err := backoff.Retry(func() error { return p.pushMessages(msgCache) }, backoff.NewExponentialBackOff()); err != nil {
			log.Err(err).Msg("error pushing messages")
			return
		}
	}
}

func (p *Producer) pushMessages(m []*Message) error {
	payloads := make([]*[]byte, 0, len(m))
	for _, message := range m {
		payloads = append(payloads, &message.Payload)
	}
	query, args, err := sqlx.In("INSERT INTO message (payload) VALUES(?)", payloads)
	if err != nil {
		return fmt.Errorf("error formulating INSERT query: %s", err)
	}
	query = p.db.Rebind(query)
	_, err = p.db.Exec(query, args)
	if err != nil {
		return fmt.Errorf("error INSERTING messages: %s", err)
	}
	return nil
}

func (p *Producer) pushMessage(m *Message) error {
	stmt := p.db.Rebind("INSERT INTO message (payload) VALUES (?)")
	res, err := p.db.Exec(stmt, &m.Payload)
	if err != nil {
		return fmt.Errorf("error INSERTing message: %s", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("error reading LastInsertId: %s", err)
	}
	m.ID = id
	return nil
}
