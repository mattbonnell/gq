package gq

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/rs/zerolog/log"
)

const (
	processErrorWaitSeconds          = 1
	concurrentProcessingGoroutines   = 10
	pullBatchSize                    = 50
	consumerIdUnassigned             = 0
	processingMaxRetries             = 3
	retryInitialBackoffPeriodSeconds = 2
)

// Consumer represents a gq consumer
type Consumer struct {
	db          *sqlx.DB
	processFunc func(message []byte) error
}

func newConsumer(ctx context.Context, db *sqlx.DB, processFunc func(message []byte) error) (*Consumer, error) {
	log.Debug().Msg("pinging database")
	if err := db.Ping(); err != nil {
		e := fmt.Errorf("error pinging database: %s", err)
		log.Debug().Err(e)
		return nil, e
	}
	c := &Consumer{db: db, processFunc: processFunc}
	go c.startPullingMessages(ctx)
	return c, nil
}

func (c *Consumer) startPullingMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Debug().Err(ctx.Err()).Msg("stopping message processing: context closed")
			return
		default:
			if err := backoff.Retry(
				func() error { return c.pullMessage(ctx, time.Now().UTC()) },
				backoff.NewConstantBackOff(time.Second*processErrorWaitSeconds)); err != nil {
				log.Err(err).Msg("permanent error pulling messages")
				return
			}
		}
	}
}

func (c *Consumer) pullMessage(ctx context.Context, now time.Time) error {
	log.Debug().Msg("pulling new message")
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		e := fmt.Errorf("error beginning message pull transaction: %s", err)
		log.Debug().Err(e)
		return e
	}
	defer tx.Rollback()
	query := tx.Rebind("SELECT id, payload, retries FROM message WHERE ready_at <= ? ORDER BY ready_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED")
	// use Query instead of QueryRow so that we can close the Rows after processing the message, thus allowing us to scan the payload into an sql.RawBytes
	// and avoid having to copy it
	rows, err := tx.QueryxContext(ctx, query, now)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debug().Err(err).Msg("no messages to pull")
			return err
		}
		e := fmt.Errorf("error pulling message: %s", err)
		log.Debug().Err(e).Msg("error")
		return e
	}
	defer rows.Close()
	var m internal.Message
	rows.Next()
	if err := rows.Scan(&m.ID, &m.Payload, &m.Retries); err != nil {
		e := fmt.Errorf("error scanning message: %s", err)
		log.Debug().Err(e).Msg("error")
		return e
	}
	if err := rows.Err(); err != nil {
		log.Debug().Err(err).Msg("error from query result")
		return err
	}
	log.Debug().Msgf("pulled message %d", m.ID)
	log.Debug().Msgf("processing message %d", m.ID)
	if err := c.processFunc([]byte(m.Payload)); err != nil {
		log.Debug().Err(err).Msg("error processing message")
		if m.Retries < processingMaxRetries {
			query := c.db.Rebind("UPDATE message WHERE id = ? SET retries = ?, ready_at = ?")
			numRetries := m.Retries + 1
			backoffPeriodSeconds := retryInitialBackoffPeriodSeconds * numRetries
			readyAt := time.Now().UTC().Add(time.Second * time.Duration(backoffPeriodSeconds))
			res, err := tx.ExecContext(ctx, query, m.ID, numRetries, readyAt)
			if err != nil {
				e := fmt.Errorf("error setting next ready_at: %s", err)
				log.Debug().Err(e).Msg("error")
				return e
			}
			n, err := res.RowsAffected()
			if err != nil || n != 1 {
				e := fmt.Errorf("error setting next ready_at: %s", err)
				log.Debug().Err(e).Msg("error")
				return e
			}
			return nil
		}
	}
	rows.Close()
	log.Debug().Msgf("successfully processed message %d", m.ID)
	log.Debug().Msgf("deleting message %d from queue", m.ID)
	query = tx.Rebind("DELETE FROM message WHERE id = ?")
	res, err := tx.ExecContext(ctx, query, m.ID)
	if err != nil {
		e := fmt.Errorf("error deleting message from queue: %s", err)
		log.Debug().Err(e).Msg("error")
		return e
	}
	n, err := res.RowsAffected()
	if err != nil || n != 1 {
		e := fmt.Errorf("error deleting message from queue: %s", err)
		log.Debug().Err(e).Msg("error")
		return e
	}
	if err := tx.Commit(); err != nil {
		e := fmt.Errorf("error committing transaction: %s", err)
		log.Debug().Err(e).Msg("error")
		return e
	}
	log.Debug().Msgf("deleted message %d from queue", m.ID)
	return nil
}
