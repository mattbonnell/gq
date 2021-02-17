package client

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

const arbitraryDriverName = "sqlite3"

func TestRegisterShouldSucceed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO consumer`)).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT created FROM consumer WHERE id = ?`)).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(time.Now().UTC()))

	c := &Consumer{db: sqlx.NewDb(db, arbitraryDriverName)}

	err = c.register()
	require.NoError(t, err)
	require.Equal(t, uint64(1), c.ID)
}

func TestKeepAliveShouldSucceed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	currTime := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE consumer WHERE id = ? SET updated = ?`)).WithArgs(1, currTime).WillReturnResult(sqlmock.NewResult(0, 1))

	c := &Consumer{db: sqlx.NewDb(db, arbitraryDriverName)}
	c.ID = 1

	err = c.keepAlive(currTime)
	require.NoError(t, err)
}

func TestKeepAliveShouldFail_NoConsumerRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE consumer WHERE id = ? SET updated = ?`)).WithArgs(1, now).WillReturnResult(sqlmock.NewResult(0, 0))

	c := &Consumer{db: sqlx.NewDb(db, arbitraryDriverName)}
	c.ID = 1

	err = c.keepAlive(now)
	require.EqualError(t, err, "consumer record wasn't updated")
}

func TestPullMessagesShouldSucceed_OneMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	lastHeartBeatThreshold := now.Add(-time.Second * keepAliveTimeoutSeconds)

	c := &Consumer{db: sqlx.NewDb(db, arbitraryDriverName), msgChan: make(chan *Message, pullBatchSize)}
	c.ID = 1
	c.Created = now.Add(-time.Minute)
	c.Updated = now.Add(-time.Second)

	expectedMessage := &Message{}
	expectedMessage.ID = 1
	expectedMessage.Payload = []byte("message payload")

	mock.ExpectBegin()

	mock.
		ExpectQuery(
			regexp.QuoteMeta(`SELECT id FROM message WHERE status = ? AND id % (SELECT count(id) FROM consumer WHERE created < ? AND updated >= ?) = (SELECT count(id) FROM consumer WHERE id != ? AND created < ? AND updated >= ?) ORDER BY created ASC LIMIT ?`),
		).
		WithArgs(
			MessageStatusQueued,
			now.Add(time.Second),
			lastHeartBeatThreshold,
			c.ID,
			c.Created,
			lastHeartBeatThreshold,
			pullBatchSize,
		).
		WillReturnRows(
			sqlmock.NewRows([]string{"id"}).
				AddRow(expectedMessage.ID),
		)

	mock.
		ExpectExec(
			regexp.QuoteMeta(`UPDATE message WHERE id IN (?) AND consumer_id = ? SET status = ?, consumer_id = ?`),
		).
		WithArgs(
			expectedMessage.ID,
			consumerIdUnassigned,
			MessageStatusConsumed,
			c.ID,
		).
		WillReturnResult(
			sqlmock.NewResult(0, 1),
		)

	mock.
		ExpectQuery(
			regexp.QuoteMeta(`SELECT id, payload FROM message WHERE id IN (?) AND consumer_id = ?`),
		).
		WithArgs(
			expectedMessage.ID,
			c.ID,
		).
		WillReturnRows(
			sqlmock.NewRows([]string{"id", "payload"}).
				AddRow(expectedMessage.ID, expectedMessage.Payload),
		)

	mock.ExpectCommit()

	err = c.popMessages(now)
	require.NoError(t, err)
	require.Exactly(t, expectedMessage, c.Pop())
}
