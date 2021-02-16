package client

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func TestNew_sqlite3(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	_, err = New(db)
	require.NoError(t, err)
}

func TestNew_sqlite3_ClosedDb(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	err = db.Close()
	require.NoError(t, err)

	_, err = New(db)
	require.Error(t, err)
}
