package client

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/sql"
	"github.com/rs/zerolog/log"
)

const (
	emptyQueueConstantBackoffDurationSeconds = 5
	concurrentProcessingGoroutines           = 10
	pullBatchSize                            = 50
	consumerIdUnassigned                     = 0
	processingMaxRetries                     = 3
	retryInitialBackoffPeriodSeconds         = 2
)

type Consumer struct {
	db                       *sqlx.DB
	processFunc              func(m *Message) error
	failedProcessingBackoffs map[int64]*backoff.BackOff
}

func newConsumer(ctx context.Context, db *sqlx.DB, processFunc func(m *Message) error) (*Consumer, error) {
	c := &Consumer{db: db, processFunc: processFunc}
	log.Debug().Msg("pinging database")
	if err := c.db.Ping(); err != nil {
		e := fmt.Errorf("error pinging database: %s", err)
		log.Debug().Err(e)
		return nil, e
	}
	go c.startProcessingMessages(ctx)
	return c, nil
}

func (c *Consumer) startProcessingMessages(ctx context.Context) {
	for {
		if err := backoff.Retry(func() error { return c.processMessage(ctx) }, backoff.NewConstantBackOff(time.Second*emptyQueueConstantBackoffDurationSeconds)); err != nil {
			log.Err(err).Msg("error pulling messages")
			return
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context) error {
	tx, err := c.db.BeginTxx(ctx, nil)
	defer tx.Rollback()
	if err != nil {
		e := fmt.Errorf("error beginning message pull transaction: %s", err)
		log.Debug().Err(e)
		return e
	}
	var sqlMessage sql.Message
	query := tx.Rebind("SELECT id, payload, retries FROM message WHERE retry_backoff_ends <= ? FOR UPDATE SKIP LOCKED ORDER BY created ASC LIMIT 1")
	if err := tx.QueryRowxContext(ctx, query, time.Now().UTC()).StructScan(&sqlMessage); err != nil {
		e := fmt.Errorf("error pulling message: %s", err)
		log.Debug().Err(e)
		return e
	}
	m, err := FromSQL(&sqlMessage)
	if err != nil {
		e := fmt.Errorf("error copying message %s", err)
		log.Debug().Err(e)
		return e
	}
	if err := c.processFunc(m); err != nil {
		log.Debug().Err(err).Msg("error processing message")
		if sqlMessage.Retries < processingMaxRetries {
			query := c.db.Rebind("UPDATE message WHERE id = ? SET retries = ?, retry_backoff_ends = ?")
			numRetries := sqlMessage.Retries + 1
			backoffPeriodSeconds := retryInitialBackoffPeriodSeconds * numRetries
			retryBackoffEnds := time.Now().UTC().Add(time.Second * time.Duration(backoffPeriodSeconds))
			res, err := tx.ExecContext(ctx, query, sqlMessage.ID, numRetries, retryBackoffEnds)
			if err != nil {
				e := fmt.Errorf("error setting next retry backoff: %s", err)
				log.Debug().Err(e)
				return e
			}
			n, err := res.RowsAffected()
			if err != nil || n != 1 {
				e := fmt.Errorf("error setting next retry backoff: %s", err)
				log.Debug().Err(e)
				return e
			}
			return nil
		}
	}
	log.Debug().Msgf("successfully processed message %d", m.ID)
	log.Debug().Msgf("deleting message %d from queue", m.ID)
	query = tx.Rebind("DELETE FROM message WHERE id = ?")
	res, err := tx.ExecContext(ctx, query, sqlMessage.ID)
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
	log.Debug().Msgf("deleted message %s from queue", m.ID)
	return nil
}
