package gq

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/rs/zerolog/log"
)

const (
	processErrorWaitSeconds          = 1
	concurrentProcessingGoroutines   = 10
	consumerIdUnassigned             = 0
	processingMaxRetries             = 3
	retryInitialBackoffPeriodSeconds = 2
	defaultPullPeriod                = 50 * time.Millisecond
	defaultMaxBatchSize              = 400
)

// ConsumerOptions represents the options which can be used to tailor producer behaviour
type ConsumerOptions struct {
	// PullPeriod is the period messages should be pulled at (default: 50ms).
	// This can be tuned to achieve the desired throughput/latency tradeoff
	PullPeriod time.Duration
	// MaxPullSize is the maximum number of messages to be pulled in one batch (default: 50)
	MaxBatchSize int
	// MaxProcessingRetries is the maximum number of times that a message will be requeued for re-processing after processing fails (default: 3)
	MaxProcessingRetries int
	// Concurrency is the number of concurrent goroutines to pull messages from (default: num cpus)
	Concurrency int
}

func defaultConsumerOpts() ConsumerOptions {
	return ConsumerOptions{
		PullPeriod:   defaultPullPeriod,
		MaxBatchSize: defaultMaxBatchSize,
		Concurrency:  runtime.NumCPU(),
	}
}

// ProcessFunc represents a function which is passed a message to process
type ProcessFunc func(message []byte) error

// Consumer represents a gq consumer
type Consumer struct {
	db      *sqlx.DB
	process ProcessFunc
	opts    ConsumerOptions
}

func newConsumer(ctx context.Context, db *sqlx.DB, process ProcessFunc, opts *ConsumerOptions) (*Consumer, error) {
	log.Debug().Msg("pinging database")
	if err := db.Ping(); err != nil {
		e := fmt.Errorf("error pinging database: %s", err)
		log.Debug().Err(e)
		return nil, e
	}
	c := &Consumer{db: db, process: process}
	if opts != nil {
		c.opts = *opts
	} else {
		c.opts = defaultConsumerOpts()
	}
	for i := 0; i < c.opts.Concurrency; i++ {
		go c.startPullingMessages(ctx)
	}
	return c, nil
}

func (c *Consumer) startPullingMessages(ctx context.Context) {
	ticker := time.NewTicker(c.opts.PullPeriod)
	for {
		select {
		case <-ctx.Done():
			log.Debug().Err(ctx.Err()).Msg("stopping message processing: context closed")
			return
		case <-ticker.C:
			c.pullMessages(ctx, time.Now().UTC())
		}
	}
}

func (c *Consumer) pullMessages(ctx context.Context, now time.Time) {
	log.Debug().Msg("pulling new message")
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		e := fmt.Errorf("error beginning message pull transaction: %s", err)
		log.Debug().Err(e)
		return
	}
	defer tx.Rollback()
	query := tx.Rebind("SELECT id, payload, retries FROM message WHERE ready_at <= ? ORDER BY ready_at ASC LIMIT ? FOR UPDATE SKIP LOCKED")
	// use Query instead of QueryRow so that we can close the Rows after processing the message, thus allowing us to scan the payload into an sql.RawBytes
	// and avoid having to copy it
	rows, err := tx.QueryxContext(ctx, query, now, c.opts.MaxBatchSize)
	if err != nil {
		log.Debug().Err(err).Msg("error pulling messages")
		return
	}
	defer rows.Close()
	var m internal.Message
	errMsgs := make([]internal.Message, 0, c.opts.MaxBatchSize)
	successMsgIds := make([]int64, 0, c.opts.MaxBatchSize)
	for rows.Next() {
		log.Debug().Msgf("pulled message %d", m.ID)
		if err := rows.Scan(&m.ID, &m.Payload, &m.Retries); err != nil {
			log.Debug().Err(err).Msg("error scanning messages")
		}
		log.Debug().Msgf("processing message %d", m.ID)
		if err := c.process([]byte(m.Payload)); err != nil {
			log.Debug().Err(err).Msgf("error processing message %d", m.ID)
			errMsgs = append(errMsgs, m)
		} else {
			log.Debug().Msgf("successfully processed message %d", m.ID)
			successMsgIds = append(successMsgIds, m.ID)
		}
	}
	if err := rows.Err(); err != nil {
		log.Debug().Err(err).Msg("error from query result")
		return
	}
	rows.Close()
	for _, m := range errMsgs {
		if int(m.Retries) < c.opts.MaxProcessingRetries {
			query := c.db.Rebind("UPDATE message WHERE id = ? SET retries = ?, ready_at = ?")
			numRetries := m.Retries + 1
			backoffPeriodSeconds := retryInitialBackoffPeriodSeconds * numRetries
			readyAt := time.Now().UTC().Add(time.Second * time.Duration(backoffPeriodSeconds))
			res, err := tx.ExecContext(ctx, query, m.ID, numRetries, readyAt)
			if err != nil {
				e := fmt.Errorf("error setting next ready_at: %s", err)
				log.Debug().Err(e).Msg("error")
			}
			n, err := res.RowsAffected()
			if err != nil || n != 1 {
				e := fmt.Errorf("error setting next ready_at: %s", err)
				log.Debug().Err(e).Msg("error")
			}
		}
	}
	log.Debug().Msg("deleting successfully processed messages from the queue")
	query, args, err := sqlx.In("DELETE FROM MESSAGE WHERE id in (?)", successMsgIds)
	if err != nil {
		log.Debug().Err(err).Msg("error formulating delete query")
		return
	}
	query = tx.Rebind(query)
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		log.Debug().Err(err).Msg("error deleting messages from queue")
		return
	}
	if err := tx.Commit(); err != nil {
		log.Debug().Msg("error committing tx")
		return
	}
}
