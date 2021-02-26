package main

import (
	"context"
	"database/sql"
	"runtime"
	"sync"

	"github.com/brianvoe/gofakeit/v6"
	_ "github.com/go-sql-driver/mysql"
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
	db, err := sql.Open(databaseDriver, databaseDSN)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open DB")
	}
	client, err := gq.NewClient(db, databaseDriver)
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
		_, err = client.NewConsumer(context.TODO(), func(consumerIndex int) func(message []byte) error {
			return func(message []byte) error {
				defer wg.Done()
				log.Debug().Msgf("[consumer %d] processing message with payload %s", consumerIndex, message)
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
		producer.Push(m[i].Payload)
	}
	wg.Wait()

}
