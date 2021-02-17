package client

import (
	"context"
	go_sql "database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

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
	db              *sqlx.DB
	processFunc     func(m *Message) error
	currentMsgBatch []Message
}

func newConsumer(db *sqlx.DB, process func(m *Message) error) (*Consumer, error) {
	c := &Consumer{db: db, processFunc: process}
	if err := c.register(); err != nil {
		return nil, err
	}
	go func() {
		if err := c.startKeepAlive(); err != nil {
			log.Error().Msgf("keep alive failed: %s", err)
		}
	}()
	return c, nil
}

func (c *Consumer) register() error {
	res, err := c.db.Exec("INSERT INTO consumer () VALUES ()")
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	c.ID = uint64(id)
	return nil
}

func (c *Consumer) startKeepAlive() error {
	for {
		if err := c.keepAlive(); err != nil {
			return err
		}
		time.Sleep(time.Second * keepAliveHeartbeatPeriodSeconds)
	}
}

func (c *Consumer) keepAlive() error {
	res, err := c.db.Exec(c.db.Rebind("UPDATE consumer WHERE id = ? SET updated = ?"), c.ID, time.Now().UTC())
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

func (c *Consumer) pullMessages() error {
	var wg sync.WaitGroup
	wg.Add(concurrentProcessingGoroutines)
	lastHeartBeatThreshold := time.Now().UTC().Add(-time.Second * keepAliveTimeoutSeconds)
	numConsumersSubQuery := `SELECT count(id) FROM consumer WHERE created < ? AND updated >= ?`
	birthIndexSubQuery := `SELECT count(id)) FROM consumer WHERE id != ? AND created < ? AND updated >= ?`
	tx, err := c.db.BeginTxx(context.TODO(), nil)
	if err != nil {
		return fmt.Errorf("error creating tx: %s", err)
	}
	defer tx.Rollback()
	query := tx.Rebind(fmt.Sprintf(`SELECT id, payload FROM message WHERE status = ? AND id %% (%s) = (%s) ORDER BY created ASC LIMIT ?`, numConsumersSubQuery, birthIndexSubQuery))
	m := []Message{}
	err = tx.Select(&m, query, MessageStatusQueued, time.Now().UTC().Add(time.Second), lastHeartBeatThreshold, c.ID, c.Created, lastHeartBeatThreshold, pullBatchSize)
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

	query, args, err := sqlx.In(`UPDATE message WHERE id IN (?) AND consumer_id = ? SET consumer_id = ?`, mIds, consumerIdUnassigned, c.ID)
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

	if int(n) < len(m) {
		log.Debug().Msg("retrieving claimed subset of initial batch")
		m = []Message{}
		query, args, err := sqlx.In(`SELECT id, payload FROM message WHERE id IN (?) AND consumer_id = ?`, mIds, c.ID)
		if err != nil {
			return fmt.Errorf("error formulating claimed messages retrieval query: %s", err)
		}
		query = tx.Rebind(query)
		err = tx.Select(&m, query, args...)
		if err != nil {
			return fmt.Errorf("error retrieving claimed batch subset: %s", err)
		}
	}
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("error committing pull tx: %s", err)
	}

	c.currentMsgBatch = m
	return nil
}
