package internal

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal/drivers/mysql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestCreate_mysql(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "failed to create mock")

	ExpectSchema(mock, mysql.Schema)

	err = CreateSchema(sqlx.NewDb(db, "mysql"))
	require.NoError(t, err)
}

func ExpectSchema(mock sqlmock.Sqlmock, schema []string) {
	mock.ExpectBegin()
	for _, stmt := range schema {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()
}
