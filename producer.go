package gq

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/rs/zerolog/log"
)

const (
	messageCacheSize = 50
	//TODO implement push batches
	pushBatchSize     = 1
	defaultMaxRetries = 3
)

type ProducerOptions struct {
	MaxRetries int
	BatchSize  int
}

func defaultOptions() ProducerOptions {
	return ProducerOptions{
		MaxRetries: defaultMaxRetries,
		BatchSize:  pushBatchSize,
	}
}

type Producer struct {
	db      *sqlx.DB
	msgChan chan internal.Message
	opts    ProducerOptions
}

func newProducer(ctx context.Context, db *sqlx.DB, opts *ProducerOptions) (*Producer, error) {
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("couldn't reach database: %s", err)
	}
	p := Producer{db: db, msgChan: make(chan internal.Message)}
	if opts != nil {
		p.opts = *opts
	} else {
		p.opts = defaultOptions()
	}
	go p.startPushingMessages(ctx)
	return &p, nil
}

func (p *Producer) Push(message []byte) {
	p.msgChan <- internal.Message{Payload: message}
}

func (p Producer) startPushingMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug().Err(ctx.Err()).Msg("stopping message pushing: context closed")
			return

		case m := <-p.msgChan:
			if err := backoff.Retry(func() error { return p.pushMessage(m) }, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(p.opts.MaxRetries))); err != nil {
				log.Err(err).Msg("error pushing messages")
				return
			}
		}
	}
}

func (p Producer) pushMessages(messages []internal.Message, numMessages int) error {
	payloads := make([]sql.RawBytes, 0, numMessages)
	for i := 0; i < numMessages; i++ {
		payloads = append(payloads, messages[i].Payload)
	}
	query, args, err := sqlx.In("INSERT INTO message (payload) VALUES (?)", payloads)
	if err != nil {
		return fmt.Errorf("error formulating INSERT query: %s", err)
	}
	query = p.db.Rebind(query)
	_, err = p.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error INSERTING messages: %s", err)
	}
	return nil
}

func (p Producer) pushMessage(m internal.Message) error {
	log.Debug().Msg("pushing message onto queue")
	stmt := p.db.Rebind("INSERT INTO message (payload) VALUES (?)")
	res, err := p.db.Exec(stmt, &m.Payload)
	if err != nil {
		return fmt.Errorf("error inserting message: %s", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("error reading LastInsertId: %s", err)
	}
	m.ID = id
	log.Debug().Msg("successfully pushed message onto queue")
	return nil
}
