package gq

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const (
	messageCacheSize  = 50
	pushBatchSize     = 1
	defaultMaxRetries = 3
	defaultPushPeriod = "5ms"
)

// ProducerOptions represents the options which can be used to tailor producer behaviour
type ProducerOptions struct {
	// MaxRetries is the maximum number of times a message push will be retried (default: 3)
	MaxRetries int
	// PushPeriod is duration of the period messages should be pushed at (default: 5ms).
	// This can be tuned to achieve the desired throughput/latency tradeoff
	PushPeriod string
}

func defaultOptions() ProducerOptions {
	return ProducerOptions{
		MaxRetries: defaultMaxRetries,
		PushPeriod: defaultPushPeriod,
	}
}

// Producer represents a message queue producer
type Producer struct {
	db      *sqlx.DB
	msgChan chan []byte
	opts    ProducerOptions
}

func newProducer(ctx context.Context, db *sqlx.DB, opts *ProducerOptions) (*Producer, error) {
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("couldn't reach database: %s", err)
	}
	p := Producer{db: db, msgChan: make(chan []byte)}
	if opts != nil {
		p.opts = *opts
	} else {
		p.opts = defaultOptions()
	}
	d, err := time.ParseDuration(p.opts.PushPeriod)
	if err != nil {
		return nil, fmt.Errorf("error parsing push period: %s", err)
	}
	go p.startPushingMessages(ctx, d)
	return &p, nil
}

// Push pushes a message onto the queue
func (p *Producer) Push(message []byte) {
	p.msgChan <- message
}

func (p Producer) startPushingMessages(ctx context.Context, period time.Duration) {
	cache := make([][]byte, 0, messageCacheSize)
	ticker := time.NewTicker(period)
	for {
		select {
		case <-ctx.Done():
			log.Debug().Err(ctx.Err()).Msg("stopping message pushing: context closed")
			return
		case m := <-p.msgChan:
			cache = append(cache, m)
		case <-ticker.C:
			if err := backoff.Retry(func() error { return p.pushMessages(cache) }, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), uint64(p.opts.MaxRetries))); err != nil {
				log.Err(err).Msg("error pushing messages")
				return
			}
			cache = make([][]byte, 0, cap(cache))
		}
	}
}

func (p Producer) pushMessages(messages [][]byte) error {
	query, args, err := sqlx.In("INSERT INTO message (payload) VALUES (?)", messages)
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

func (p Producer) pushMessage(message []byte) error {
	log.Debug().Msg("pushing message onto queue")
	stmt := p.db.Rebind("INSERT INTO message (payload) VALUES (?)")
	_, err := p.db.Exec(stmt, message)
	if err != nil {
		return fmt.Errorf("error inserting message: %s", err)
	}
	log.Debug().Msg("successfully pushed message onto queue")
	return nil
}
