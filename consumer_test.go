package gq

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/stretchr/testify/require"
)

const arbitraryDriverName = "mysql"

func TestPullMessageShouldSucceed_OneMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	ctx := context.Background()

	now := time.Now().UTC()

	expectedMessage := internal.Message{}
	expectedMessage.ID = 1
	expectedPayload := []byte("message payload")

	c, err := newConsumer(ctx, sqlx.NewDb(db, arbitraryDriverName), func(message []byte) error {
		require.Equal(t, expectedPayload, message)
		return nil
	}, nil)

	mock.ExpectBegin()

	mock.
		ExpectQuery(
			regexp.QuoteMeta(`SELECT id, payload, retries FROM message WHERE ready_at <= ? ORDER BY ready_at ASC LIMIT ? FOR UPDATE SKIP LOCKED`),
		).
		WithArgs(
			now,
			c.opts.MaxBatchSize,
		).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "payload", "retries"}).
				AddRow(expectedMessage.ID, expectedPayload, expectedMessage.Retries),
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

	c.pullMessages(ctx, now)
}
