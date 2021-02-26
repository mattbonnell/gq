package main

import (
	"context"
	"runtime"
	"sync"

	"github.com/brianvoe/gofakeit/v6"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	databaseDriver = "mysql"
	databaseDSN    = "root:password@tcp(localhost:3306)/gq"
	numMessages    = 100
)

func main() {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	db := sqlx.MustConnect(databaseDriver, databaseDSN)
	client, err := gq.NewClient(db)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new client")
	}
	producer, err := client.NewProducer(context.TODO(), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new producer")
	}

	wg := sync.WaitGroup{}
	wg.Add(numMessages)
	for i := 0; i < runtime.NumCPU(); i++ {
		_, err = client.NewConsumer(context.TODO(), func(consumerIndex int) func(m gq.Message) error {
			return func(m gq.Message) error {
				defer wg.Done()
				log.Debug().Msgf("[consumer %d] processing message %d with payload %s", consumerIndex, m.ID, m.Payload)
				return nil

			}
		}(i))
	}
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new consumer")
	}

	m := make([]gq.Message, numMessages)
	for i := range m {
		m[i].Payload = []byte(gofakeit.LoremIpsumSentence(10))
		producer.Push(m[i])
	}
	wg.Wait()

}
