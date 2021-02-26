package internal

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/test"
	"github.com/stretchr/testify/require"
)

func TestCreateSchema(t *testing.T) {
	for _, d := range SupportedDrivers {
		t.Run(fmt.Sprintf("driver=%s", d), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err, "failed to create mock")
			schema, err := GetSchema(d)
			require.NoError(t, err, "failed to get schema")
			test.ExpectSchema(t, mock, schema)

			err = CreateSchema(sqlx.NewDb(db, d))
			require.NoError(t, err)
		})
	}
}
