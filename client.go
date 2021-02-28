package gq

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// Client represents a client of the message queue. It can be used to spawn any number of consumers or producers.
type Client struct {
	db *sqlx.DB
}

// NewClient creates a new Client, generating the database schema if it doesn't exist
func NewClient(db *sql.DB, driverName string) (*Client, error) {
	log.Debug().Msg("creating new client")
	c := Client{db: sqlx.NewDb(db, driverName)}
	if err := internal.CreateSchema(c.db); err != nil {
		err = fmt.Errorf("error creating schema: %s", err)
		log.Debug().Msg(err.Error())
		return nil, err
	}
	log.Debug().Msg("client created")
	return &c, nil
}

// NewConsumer creates a new gq Consumer. It begins pulling messages immediately, and passes each one to the supplied process function
func (c Client) NewConsumer(ctx context.Context, process ProcessFunc, opts *ConsumerOptions) (*Consumer, error) {
	return newConsumer(ctx, c.db, process, opts)
}

// NewProducer creates a new gq Producer
func (c Client) NewProducer(ctx context.Context, opts *ProducerOptions) (*Producer, error) {
	return newProducer(ctx, c.db, opts)
}
