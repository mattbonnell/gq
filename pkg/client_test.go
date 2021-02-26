package gq

import (
	"fmt"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/mattbonnell/gq/internal"
	"github.com/mattbonnell/gq/test"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	for _, d := range internal.SupportedDrivers {
		t.Run(fmt.Sprintf("driver=%s", d), func(t *testing.T) {
			t.Run("new client should succeed", func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)

				schema, err := internal.GetSchema(d)
				require.NoError(t, err, "failed to get schema")
				test.ExpectSchema(t, mock, schema)

				_, err = NewClient(sqlx.NewDb(db, d))
				require.NoError(t, err)
			})
			t.Run("new client should fail, closed db", func(t *testing.T) {

				db, mock, err := sqlmock.New()
				require.NoError(t, err)

				mock.ExpectClose()

				err = db.Close()
				require.NoError(t, err)

				_, err = NewClient(sqlx.NewDb(db, d))
				require.Error(t, err)
			})
		})

	}
}
