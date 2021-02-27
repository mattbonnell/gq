package gq

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const (
	messageCacheSize       = 50
	pushBatchSize          = 1
	defaultPushPeriod      = "50ms"
	defaultMaxRetryPeriods = 1
)

// ProducerOptions represents the options which can be used to tailor producer behaviour
type ProducerOptions struct {
	// PushPeriod is duration of the period messages should be pushed at (default: 50ms).
	// This can be tuned to achieve the desired throughput/latency tradeoff
	PushPeriod string
	// MaxRetryPeriods is the maximum number of push periods to retry a batch of messages for before discarding them (default: 1)
	MaxRetryPeriods int
}

func defaultOptions() ProducerOptions {
	return ProducerOptions{
		PushPeriod:      defaultPushPeriod,
		MaxRetryPeriods: defaultMaxRetryPeriods,
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
			if len(cache) == 0 {
				continue
			}
			ctx, cancel := context.WithTimeout(ctx, period*time.Duration(p.opts.MaxRetryPeriods))
			if err := backoff.Retry(func() error { return p.pushMessages(cache) }, backoff.WithContext(backoff.NewExponentialBackOff(), ctx)); err != nil {
				log.Err(err).Msg("error pushing messages")
			}
			cancel() // release ctx resources if timeout hasn't expired
			cache = clear(cache)
		}
	}
}

func clear(cache [][]byte) [][]byte {
	for i := range cache {
		cache[i] = []byte{} // allow elements to be garbage-collected
	}
	return cache[:0]
}

func (p Producer) pushMessages(messages [][]byte) error {
	log.Debug().Msgf("pushing %d messages onto queue", len(messages))
	valuesListBuilder := strings.Builder{}
	valuesListBuilder.Grow(len(messages) * len([]byte("(?), ")))
	args := make([]interface{}, len(messages))
	for i := range messages {
		if i == 0 {
			valuesListBuilder.WriteString("(?)")
		} else {
			valuesListBuilder.WriteString(", (?)")
		}
		args[i] = messages[i]
	}
	query := fmt.Sprintf("INSERT INTO message (payload) VALUES %s", valuesListBuilder.String())
	query = p.db.Rebind(query)
	if _, err := p.db.Exec(query, args...); err != nil {
		return fmt.Errorf("error INSERTING messages: %s", err)
	}
	log.Debug().Msg("successfully pushed messages onto queue")
	return nil
}
