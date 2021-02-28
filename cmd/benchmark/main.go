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
		_, err = client.NewConsumer(context.TODO(), func(consumerIndex int) gq.ProcessFunc {
			return func(message []byte) error {
				defer wg.Done()
				log.Debug().Msgf("[consumer %d] processing message with payload %s", consumerIndex, message)
				return nil

			}
		}(i), nil)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new consumer")
	}

	m := make([][]byte, numMessages)
	for i := range m {
		m[i] = []byte(gofakeit.LoremIpsumSentence(10))
		producer.Push(m[i])
	}
	wg.Wait()

}
