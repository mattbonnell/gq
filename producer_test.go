package gq

import (
	"context"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/stretchr/testify/require"
)

func TestPushMessageShouldSucceed_OneMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	m := internal.Message{Payload: []byte("random payload")}

	mock.
		ExpectExec(
			regexp.QuoteMeta(`INSERT INTO message (payload) VALUES (?)`),
		).
		WithArgs(m.Payload).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	p, err := newProducer(ctx, sqlx.NewDb(db, arbitraryDriverName), &ProducerOptions{PushPeriod: "500ns", MaxRetryPeriods: 0})
	require.NoError(t, err)

	p.Push(m.Payload)
	time.Sleep(time.Millisecond)
	require.NoError(t, mock.ExpectationsWereMet())
}

// TODO: fix this test
func TestPushMessageShouldSucceed_ThreeMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	mock.MatchExpectationsInOrder(false)
	require.NoError(t, err)

	messages := make([][]byte, 3)
	for i := range messages {
		messages[i] = []byte("payload" + strconv.Itoa(i))
	}

	mock.
		ExpectExec(
			regexp.QuoteMeta(`INSERT INTO message (payload) VALUES (?), (?), (?)`),
		).
		WithArgs(messages[0], messages[1], messages[2]).
		WillReturnResult(sqlmock.NewResult(3, 3))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	p, err := newProducer(ctx, sqlx.NewDb(db, arbitraryDriverName), &ProducerOptions{PushPeriod: "1ms", MaxRetryPeriods: 0})
	require.NoError(t, err)

	for _, m := range messages {
		p.Push(m)
	}
	time.Sleep(time.Millisecond * 5)
	require.NoError(t, mock.ExpectationsWereMet())
}
