package internal

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
)

func TestCreateSchema(t *testing.T) {
	driverNames := []string{"mysql", "postgres", "pq", "pqx"}
	for _, d := range driverNames {
		t.Run(fmt.Sprintf("driver=%s", d), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err, "failed to create mock")

			ExpectSchema(t, mock, d)

			err = CreateSchema(sqlx.NewDb(db, d))
			require.NoError(t, err)
		})
	}
}

func ExpectSchema(t *testing.T, mock sqlmock.Sqlmock, driverName string) {
	mock.ExpectBegin()
	schema, err := getSchema(driverName)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range schema {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()
}
