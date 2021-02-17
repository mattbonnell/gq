package client

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type Client struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) (*Client, error) {
	log.Debug().Msg("creating new client")
	c := Client{db: db}
	if err := c.db.Ping(); err != nil {
		err = fmt.Errorf("couldn't connect to db: %s", err)
		log.Debug().Msg(err.Error())
		return nil, err
	}
	log.Debug().Msg("client created")
	return &c, nil
}

func (c Client) NewConsumer() (*Consumer, error) {
	return newConsumer(c.db)
}
