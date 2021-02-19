package pkg

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const (
	messageCacheSize = 50
	//TODO implement push batches
	pushBatchSize = 1
	maxRetries    = 1
)

type Producer struct {
	db      *sqlx.DB
	msgChan chan *Message
}

func newProducer(ctx context.Context, db *sqlx.DB) (*Producer, error) {
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("couldn't reach database: %s", err)
	}
	p := &Producer{db: db, msgChan: make(chan *Message)}
	go p.startPushingMessages(ctx)
	return p, nil
}

func (p *Producer) Push(m *Message) {
	p.msgChan <- m
}

func (p *Producer) startPushingMessages(ctx context.Context) {
	msgCache := make([]*Message, messageCacheSize)
	numCached := 0
	for {
		select {
		case <-ctx.Done():
			log.Debug().Err(ctx.Err()).Msg("stopping message pushing: context closed")
			return

		case m := <-p.msgChan:
			msgCache[numCached] = m
			numCached += 1
			if numCached == pushBatchSize {
				break
			}
		}
		if err := backoff.Retry(func() error { return p.pushMessages(msgCache, numCached) }, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), maxRetries)); err != nil {
			log.Err(err).Msg("error pushing messages")
			return
		}
		numCached = 0
	}
}

func (p *Producer) pushMessages(messages []*Message, numMessages int) error {
	payloads := make([]*[]byte, 0, numMessages)
	for i := 0; i < numMessages; i++ {
		payloads = append(payloads, &messages[i].Payload)
	}
	query, args, err := sqlx.In("INSERT INTO message (payload) VALUES (?)", payloads)
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
