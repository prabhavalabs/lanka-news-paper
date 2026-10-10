package social

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTokenCipherBindsPageAndState(t *testing.T) {
	cipher, err := newTokenCipher(bytes.Repeat([]byte{7}, 32))
	require.NoError(t, err)
	encrypted, nonce, err := cipher.encrypt([]byte("private-page-token"), "social:page-one")
	require.NoError(t, err)
	plain, err := cipher.decrypt(encrypted, nonce, "social:page-one")
	require.NoError(t, err)
	require.Equal(t, "private-page-token", string(plain))
	_, err = cipher.decrypt(encrypted, nonce, "social:page-two")
	require.Error(t, err)
	_, err = cipher.decrypt(encrypted, nonce, "social:oauth:state")
	require.Error(t, err)
	_, err = cipher.decrypt(encrypted, []byte{1}, "social:page-one")
	require.Error(t, err)
	encrypted[0] ^= 1
	_, err = cipher.decrypt(encrypted, nonce, "social:page-one")
	require.Error(t, err)
	_, err = newTokenCipher([]byte("short"))
	require.Error(t, err)
}

func TestTokenCipherUsesRandomNonce(t *testing.T) {
	cipher, err := newTokenCipher(make([]byte, 32))
	require.NoError(t, err)
	a, na, err := cipher.encrypt([]byte("token"), "social:oauth:one")
	require.NoError(t, err)
	b, nb, err := cipher.encrypt([]byte("token"), "social:oauth:one")
	require.NoError(t, err)
	require.NotEqual(t, na, nb)
	require.NotEqual(t, a, b)
}
