package benchmark

import (
	"context"

	"github.com/jmoiron/sqlx"
	gq "github.com/mattbonnell/gq/pkg"
	"github.com/rs/zerolog/log"
)

const (
	databaseDriver = "mysql"
	databaseDSN    = "root:password@tcp(3306)/gq"
)

func main() {
	db := sqlx.MustConnect(databaseDriver, databaseDSN)
	client, err := gq.NewClient(db)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new client")
	}
	producer, err := client.NewProducer(context.TODO(), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new producer")
	}
	consumer, err := client.NewConsumer(context.TODO(), func(m *gq.Message) error {
		log.Debug().Msgf("consumed message %d with payload %s", m.ID, m.Payload)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("error creating new consumer")
	}

	messages := make([]gq.Message, 0, 100)

}
