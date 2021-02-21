package gq

import (
	"context"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestPushMessageShouldSucceed_OneMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	m := Message{Payload: []byte("random payload")}

	mock.
		ExpectExec(
			regexp.QuoteMeta(`INSERT INTO message (payload) VALUES (?)`),
		).
		WithArgs(m.Payload).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	p, err := newProducer(ctx, sqlx.NewDb(db, arbitraryDriverName), WithMaxRetries(0))
	require.NoError(t, err)

	p.Push(&m)

	select {
	case <-ctx.Done():
		if err := mock.ExpectationsWereMet(); err != nil {
			require.FailNowf(t, ctx.Err().Error(), "context timed-out")
		}
	default:
		if err := mock.ExpectationsWereMet(); err == nil {
			cancel()
		}
	}
}

// TODO: fix this test
func TestPushMessageShouldSucceed_ThreeMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	mock.MatchExpectationsInOrder(false)
	require.NoError(t, err)

	messages := make([]Message, 3)
	for i := range messages {
		messages[i].Payload = []byte("payload" + strconv.Itoa(i))
	}

	for i := range messages {
		mock.
			ExpectExec(
				regexp.QuoteMeta(`INSERT INTO message (payload) VALUES (?)`),
			).
			WithArgs(messages[i].Payload).
			WillReturnResult(sqlmock.NewResult(int64(i+1), 1))
		t.Logf("expecting message with payload %s", string(messages[i].Payload))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
	defer cancel()

	p, err := newProducer(ctx, sqlx.NewDb(db, arbitraryDriverName), WithMaxRetries(0))
	require.NoError(t, err)

	for _, m := range messages {
		p.Push(&m)
		time.Sleep(time.Millisecond)
	}

	select {
	case <-ctx.Done():
		if err := mock.ExpectationsWereMet(); err != nil {
			require.FailNowf(t, ctx.Err().Error(), "context timed-out")
		}
	default:
		if err := mock.ExpectationsWereMet(); err == nil {
			cancel()
		}
	}
}
