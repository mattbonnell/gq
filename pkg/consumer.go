package gq

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

const (
	processErrorWaitSeconds          = 2
	concurrentProcessingGoroutines   = 10
	pullBatchSize                    = 50
	consumerIdUnassigned             = 0
	processingMaxRetries             = 3
	retryInitialBackoffPeriodSeconds = 2
)

type Consumer struct {
	db                       *sqlx.DB
	processFunc              func(m *Message) error
	failedProcessingBackoffs map[int64]*backoff.BackOff
}

func newConsumer(ctx context.Context, db *sqlx.DB, processFunc func(m *Message) error) (*Consumer, error) {
	log.Debug().Msg("pinging database")
	if err := db.Ping(); err != nil {
		e := fmt.Errorf("error pinging database: %s", err)
		log.Debug().Err(e)
		return nil, e
	}
	c := &Consumer{db: db, processFunc: processFunc}
	go c.startProcessingMessages(ctx)
	return c, nil
}

func (c *Consumer) startProcessingMessages(ctx context.Context) {
	select {
	case <-ctx.Done():
		log.Debug().Err(ctx.Err()).Msg("stopping message processing: context closed")
		return
	default:
		if err := backoff.Retry(
			func() error { return c.processMessage(ctx, time.Now().UTC()) },
			backoff.NewConstantBackOff(time.Second*processErrorWaitSeconds)); err != nil {
			log.Err(err).Msg("permanent error pulling messages")
			return
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, now time.Time) error {
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		e := fmt.Errorf("error beginning message pull transaction: %s", err)
		log.Debug().Err(e)
		return e
	}
	defer tx.Rollback()
	var m Message
	query := tx.Rebind("SELECT id, payload, retries FROM message WHERE ready_at <= ? FOR UPDATE SKIP LOCKED ORDER BY ready_at ASC LIMIT 1")
	if err := tx.QueryRowContext(ctx, query, now).Scan(&m.ID, &m.Payload, &m.retries); err != nil {
		if err == sql.ErrNoRows {
			log.Debug().Err(err).Msg("no messages to pull")
			return err
		}
		e := fmt.Errorf("error pulling message: %s", err)
		log.Debug().Err(e)
		return e
	}
	if err := c.processFunc(&m); err != nil {
		log.Debug().Err(err).Msg("error processing message")
		if m.retries < processingMaxRetries {
			query := c.db.Rebind("UPDATE message WHERE id = ? SET retries = ?, ready_at = ?")
			numRetries := m.retries + 1
			backoffPeriodSeconds := retryInitialBackoffPeriodSeconds * numRetries
			readyAt := time.Now().UTC().Add(time.Second * time.Duration(backoffPeriodSeconds))
			res, err := tx.ExecContext(ctx, query, m.ID, numRetries, readyAt)
			if err != nil {
				e := fmt.Errorf("error setting next ready_at: %s", err)
				log.Debug().Err(e)
				return e
			}
			n, err := res.RowsAffected()
			if err != nil || n != 1 {
				e := fmt.Errorf("error setting next ready_at: %s", err)
				log.Debug().Err(e)
				return e
			}
			return nil
		}
	}
	log.Debug().Msgf("successfully processed message %d", m.ID)
	log.Debug().Msgf("deleting message %d from queue", m.ID)
	query = tx.Rebind("DELETE FROM message WHERE id = ?")
	res, err := tx.ExecContext(ctx, query, m.ID)
	if err != nil {
		e := fmt.Errorf("error deleting message from queue: %s", err)
		log.Debug().Err(e)
		return e
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		e := fmt.Errorf("error deleting message from queue: %s", err)
		log.Debug().Err(e)
		return e
	}
	if err := tx.Commit(); err != nil {
		e := fmt.Errorf("error committing transaction: %s", err)
		log.Debug().Err(e)
		return e
	}
	log.Debug().Msgf("deleted message %d from queue", m.ID)
	return nil
}
