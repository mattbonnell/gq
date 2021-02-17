package client

import (
	"context"
	go_sql "database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/sql"
	"github.com/rs/zerolog/log"
)

const (
	keepAliveTimeoutSeconds         = 5
	keepAliveHeartbeatPeriodSeconds = 2
	concurrentProcessingGoroutines  = 10
	pullBatchSize                   = 50
	consumerIdUnassigned            = 0
)

type Consumer struct {
	sql.Consumer
	db      *sqlx.DB
	msgChan chan *Message
}

func (c Consumer) Message() <-chan *Message {
	return c.msgChan
}

func newConsumer(db *sqlx.DB) (*Consumer, error) {
	c := &Consumer{db: db, msgChan: make(chan *Message, pullBatchSize)}
	if err := c.register(); err != nil {
		return nil, err
	}
	go c.startKeepAlive()
	go c.startPullMessages()
	return c, nil
}

func (c *Consumer) register() error {
	res, err := c.db.Exec("INSERT INTO consumer")
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = uint64(id)
	query := c.db.Rebind("SELECT created FROM consumer WHERE id = ?")
	return c.db.Get(&c.Created, query, c.ID)
}

func (c *Consumer) startKeepAlive() {
	for {
		if err := backoff.Retry(func() error { return c.keepAlive(time.Now().UTC()) }, backoff.NewExponentialBackOff()); err != nil {
			log.Err(err).Msg("error keeping consumer alive")
			return
		}
		time.Sleep(time.Second * keepAliveHeartbeatPeriodSeconds)
	}
}

func (c *Consumer) keepAlive(now time.Time) error {
	query := c.db.Rebind("UPDATE consumer WHERE id = ? SET updated = ?")
	res, err := c.db.Exec(query, c.ID, now)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return errors.New("consumer record wasn't updated")
	}
	return nil
}

func (c *Consumer) startPullMessages() {
	for {
		if err := backoff.Retry(func() error { return c.pullMessages(time.Now().UTC()) }, backoff.NewExponentialBackOff()); err != nil {
			log.Err(err).Msg("error pulling messages")
			return
		}
	}
}

func (c *Consumer) pullMessages(now time.Time) error {
	lastHeartBeatThreshold := now.Add(-time.Second * keepAliveTimeoutSeconds)
	numConsumersSubQuery := `SELECT count(id) FROM consumer WHERE created < ? AND updated >= ?`
	birthIndexSubQuery := `SELECT count(id) FROM consumer WHERE id != ? AND created < ? AND updated >= ?`
	tx, err := c.db.BeginTxx(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("error creating tx: %s", err)
	}
	defer tx.Rollback()
	query := tx.Rebind(fmt.Sprintf(`SELECT id FROM message WHERE status = ? AND id %% (%s) = (%s) ORDER BY created ASC LIMIT ?`, numConsumersSubQuery, birthIndexSubQuery))
	m := []Message{}
	err = tx.Select(&m, query, MessageStatusQueued, now.Add(time.Second), lastHeartBeatThreshold, c.ID, c.Created, lastHeartBeatThreshold, pullBatchSize)
	if err != nil {
		if err == go_sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("error making initial message batch pull: %s", err)
	}
	log.Debug().Msgf("pulled %d messages in initial batch", len(m))

	mIds := make([]uint64, len(m))
	for i := range m {
		mIds[i] = m[i].ID
	}

	query, args, err := sqlx.In(`UPDATE message WHERE id IN (?) AND consumer_id = ? SET status = ?, consumer_id = ?`, mIds, consumerIdUnassigned, MessageStatusConsumed, c.ID)
	if err != nil {
		return fmt.Errorf("error formulating message claim statement: %s", err)
	}
	query = tx.Rebind(query)

	res, err := tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("error executing message claim statement: %s", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error retrieving number of messages claimed: %s", err)
	}
	log.Debug().Msgf("claimed %d/%d messages", n, len(m))

	log.Debug().Msg("retrieving claimed subset of initial batch")
	query, args, err = sqlx.In(`SELECT id, payload FROM message WHERE id IN (?) AND consumer_id = ?`, mIds, c.ID)
	if err != nil {
		return fmt.Errorf("error formulating claimed messages retrieval query: %s", err)
	}
	query = tx.Rebind(query)
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return fmt.Errorf("error retrieving claimed batch subset: %s", err)
	}
	defer rows.Close()
	log.Debug().Msg("committing pullMessages tx")

	log.Debug().Msg("delivering messages")
	for rows.Next() {
		var messageRecord sql.Message
		err := rows.StructScan(&messageRecord)
		if err != nil {
			return fmt.Errorf("error scanning message record: %s", err)
		}
		m, err := FromSQL(&messageRecord)
		if err != nil {
			return err
		}
		log.Debug().Msgf("delivering message %d", m.ID)
		c.msgChan <- m
		log.Debug().Msgf("delivered message %d", m.ID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error with message results object: %s", err)
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing pull tx: %s", err)
	}
	return nil
}
