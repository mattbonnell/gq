package test

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func ExpectSchema(t *testing.T, mock sqlmock.Sqlmock, schema []string) {
	mock.ExpectBegin()
	for _, stmt := range schema {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectCommit()
}
