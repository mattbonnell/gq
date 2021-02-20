package pkg

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/drivers/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestNewShouldSucceed_mysql(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	mock.ExpectBegin()
	for _, stmt := range mysql.Schema {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()

	_, err = New(sqlx.NewDb(db, "mysql"))
	require.NoError(t, err)
}

func TestNewShouldFail_mysql_ClosedDb(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	mock.ExpectClose()

	err = db.Close()
	require.NoError(t, err)

	_, err = New(sqlx.NewDb(db, "mysql"))
	require.Error(t, err)
}
