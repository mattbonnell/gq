package gq

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

const arbitraryDriverName = "mysql"

func TestProcessMessageShouldSucceed_OneMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	ctx := context.Background()

	now := time.Now().UTC()

	expectedMessage := &Message{}
	expectedMessage.ID = 1
	expectedMessage.Payload = []byte("message payload")

	c, err := newConsumer(ctx, sqlx.NewDb(db, arbitraryDriverName), func(m Message) error {
		require.Equal(t, expectedMessage, m)
		return nil
	})

	mock.ExpectBegin()

	mock.
		ExpectQuery(
			regexp.QuoteMeta(`SELECT id, payload, retries FROM message WHERE ready_at <= ? ORDER BY ready_at ASC LIMIT 1 FOR UPDATE SKIP LOCKED`),
		).
		WithArgs(
			now,
		).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "payload", "retries"}).
				AddRow(expectedMessage.ID, expectedMessage.Payload, expectedMessage.retries),
		)

	mock.
		ExpectExec(
			regexp.QuoteMeta(`DELETE FROM message WHERE id = ?`),
		).
		WithArgs(
			expectedMessage.ID,
		).
		WillReturnResult(
			sqlmock.NewResult(0, 1),
		)

	mock.ExpectCommit()

	err = c.processMessage(ctx, now)
	require.NoError(t, err)
}
