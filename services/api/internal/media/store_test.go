package media

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalStoreRoundTrip(t *testing.T) {
	store, err := New(context.Background(), Config{LocalDirectory: t.TempDir()})
	require.NoError(t, err)

	const key = "source-logos/source/logo.png"
	require.NoError(t, store.Put(context.Background(), key, "image/png", []byte("logo")))
	body, contentType, err := store.Open(context.Background(), key)
	require.NoError(t, err)
	t.Cleanup(func() { body.Close() })
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	require.Equal(t, "image/png", contentType)
	require.Equal(t, []byte("logo"), data)
	require.NoError(t, store.Delete(context.Background(), key))
	_, _, err = store.Open(context.Background(), key)
	require.Error(t, err)
}

func TestKeyFromURLRejectsTraversal(t *testing.T) {
	_, ok := KeyFromURL(URLPrefix + "../secret")
	require.False(t, ok)
}
