package pkg

import (
	"testing"

	"github.com/mattbonnell/gq/internal"
	"github.com/stretchr/testify/require"
)

func TestMessageFromSQL_StringPayload(t *testing.T) {
	p := []byte("random payload")
	s := internal.Message{}
	s.ID = 9
	s.Payload = p
	m, err := FromSQL(&s)
	require.NoError(t, err)
	s.Payload = []byte("something else")
	require.Equal(t, p, m.Payload)
	require.Equal(t, s.ID, m.ID)
}

func TestMessageFromSQL_EmptyPayload(t *testing.T) {
	s := internal.Message{}
	s.Payload = make([]byte, 0)
	m, err := FromSQL(&s)
	require.NoError(t, err)
	require.Equal(t, []byte(s.Payload), m.Payload)
}
