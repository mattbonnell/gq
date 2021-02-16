package client

import (
	go_sql "database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/sql"
)

const (
	keepAliveTimeoutSeconds         = 5
	keepAliveHeartbeatPeriodSeconds = 2
)

type Consumer struct {
	sql.Consumer
	db          *sqlx.DB
	birthOrder  int
	processFunc func(m *Message) error
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
	tx, err := c.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	lastHeartBeatThreshold := time.Now().Add(-time.Second * keepAliveTimeoutSeconds)

	// update birth order
	query := tx.Rebind("SELECT count(id) FROM consumer WHERE id != ? AND created < ? AND updated >= ?")

	var n int
	if err := tx.Get(&n, query, c.ID, c.Created, lastHeartBeatThreshold); err != nil && err != go_sql.ErrNoRows {
		return fmt.Errorf("error retrieving consumers: %s", err)
	}
	c.birthOrder = n

	// update own heartbeat
	res, err := tx.Exec(tx.Rebind("UPDATE consumer WHERE id = ? SET updated = ?"), c.ID, time.Now().UTC())
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return errors.New("consumer record wasn't updated")
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error committing transaction: %s", err)
	}
	return nil
}

// TODO: implement this
func (c *Consumer) pullMessages() error {
	return nil
}
