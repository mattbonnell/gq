package gq

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const (
	messageBufferSize      = 100
	pushBatchSize          = 1
	defaultPushPeriod      = time.Millisecond * 50
	defaultMaxRetryPeriods = 1
	maxBatchQuerySize      = (1 << 16) - 1
)

// ProducerOptions represents the options which can be used to tailor producer behaviour
type ProducerOptions struct {
	// PushPeriod is the period with which messages should be pushed (default: 50ms).
	// This can be tuned to achieve the desired throughput/latency tradeoff
	PushPeriod time.Duration
	// MaxRetryPeriods is the maximum number of push periods to retry a batch of messages for before discarding them (default: 1)
	MaxRetryPeriods int
	// Concurrency is the number of concurrent goroutines to push messages from (default: num cpus)
	Concurrency int
}

func defaultProducerOpts() ProducerOptions {
	return ProducerOptions{
		PushPeriod:      defaultPushPeriod,
		MaxRetryPeriods: defaultMaxRetryPeriods,
		Concurrency:     runtime.NumCPU(),
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
	p := &Producer{db: db, msgChan: make(chan []byte)}
	if opts != nil {
		p.opts = *opts
	} else {
		p.opts = defaultProducerOpts()
	}
	for i := 0; i < p.opts.Concurrency; i++ {
		go p.startPushingMessages(ctx)
	}
	return p, nil
}

// Push pushes a message onto the queue
func (p *Producer) Push(message []byte) {
	p.msgChan <- message
}

func (p Producer) startPushingMessages(ctx context.Context) {
	buf := make([][]byte, 0, messageBufferSize)
	ticker := time.NewTicker(p.opts.PushPeriod)
	retryTimeout := p.opts.PushPeriod * time.Duration(p.opts.MaxRetryPeriods)
	for {
		select {
		case <-ctx.Done():
			log.Debug().Err(ctx.Err()).Msg("stopping message pushing: context closed")
			return
		case m := <-p.msgChan:
			buf = append(buf, m)
			if len(buf) == maxBatchQuerySize {
				p.pushMessagesWithRetryTimeout(ctx, buf, retryTimeout)
				buf = clear(buf)
			}
		case <-ticker.C:
			if len(buf) == 0 {
				continue
			}
			p.pushMessagesWithRetryTimeout(ctx, buf, retryTimeout)
			buf = clear(buf)
		}
	}
}

func clear(buffer [][]byte) [][]byte {
	for i := range buffer {
		buffer[i] = []byte{} // allow elements to be garbage-collected
	}
	return buffer[:0]
}

func (p Producer) pushMessagesWithRetryTimeout(ctx context.Context, messages [][]byte, retryTimeout time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, retryTimeout)
	if err := backoff.Retry(func() error { return p.pushMessages(messages) }, backoff.WithContext(backoff.NewExponentialBackOff(), ctx)); err != nil {
		log.Err(err).Msg("error pushing messages")
	}
	cancel() // release ctx resources if timeout hasn't expired

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
