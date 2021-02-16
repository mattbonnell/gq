package schema

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestCreateSchema_sqlite3(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("failed to create schema: %s\n", r)
		}
	}()
	db, err := sqlx.Connect("sqlite3", ":memory:")
	require.NoError(t, err, "failed to connect to db")

	CreateSchema(db)
}
