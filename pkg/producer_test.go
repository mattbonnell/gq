package pkg

import (
	"context"
	"database/sql"
	"database/sql/driver"
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
			regexp.QuoteMeta("INSERT INTO message (payload) VALUES (?)"),
		).
		WithArgs(m.Payload).
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)

	p, err := newProducer(ctx, sqlx.NewDb(db, arbitraryDriverName))
	require.NoError(t, err)

	p.Push(&m)

	select {
	case <-ctx.Done():
		require.Failf(t, ctx.Err().Error(), "context timed-out")
		break
	default:
		if err := mock.ExpectationsWereMet(); err == nil {
			break
		}
	}
	cancel()
	return
}

// TODO: fix this test
func TestPushMessageShouldSucceed_TenMessages(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	messages := make([]Message, 10)
	payloads := make([]driver.Value, 10)
	for i := range messages {
		p := sql.RawBytes([]byte("payload" + strconv.Itoa(i)))
		payloads[i] = driver.Value(p)
		messages[i].Payload = p
	}

	mock.
		ExpectExec(
			regexp.QuoteMeta("INSERT INTO message (payload) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"),
		).
		WithArgs(payloads...).
		WillReturnResult(sqlmock.NewResult(10, 10))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	p, err := newProducer(ctx, sqlx.NewDb(db, arbitraryDriverName))
	require.NoError(t, err)

	for _, m := range messages {
		p.Push(&m)
	}

	select {
	case <-ctx.Done():
		require.FailNowf(t, ctx.Err().Error(), "context timed-out")
	default:
		if err := mock.ExpectationsWereMet(); err == nil {
			return
		}
	}
}
