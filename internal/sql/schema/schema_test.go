package schema

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestCreateSchema_sqlite3(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err, "failed to connect to db")

	err = CreateSchema(db)
	require.NoError(t, err)
}
