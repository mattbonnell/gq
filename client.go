package gq

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/rs/zerolog/log"
)

type Client struct {
	db *sqlx.DB
}

func NewClient(db *sqlx.DB) (*Client, error) {
	log.Debug().Msg("creating new client")
	c := Client{db: db}
	if err := internal.CreateSchema(c.db); err != nil {
		err = fmt.Errorf("error creating schema: %s", err)
		log.Debug().Msg(err.Error())
		return nil, err
	}
	log.Debug().Msg("client created")
	return &c, nil
}

func (c Client) NewConsumer(ctx context.Context, process func(m Message) error) (*Consumer, error) {
	return newConsumer(ctx, c.db, process)
}

func (c Client) NewProducer(ctx context.Context, opts *ProducerOptions) (*Producer, error) {
	return newProducer(ctx, c.db, opts)
}
