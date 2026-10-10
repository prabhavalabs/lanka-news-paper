package social

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

type tokenCipher struct{ aead cipher.AEAD }

func newTokenCipher(key []byte) (*tokenCipher, error) {
	if len(key) != 32 {
		return nil, errors.New("social token key must contain 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &tokenCipher{aead: aead}, nil
}

func (cipher *tokenCipher) encrypt(plain []byte, aad string) ([]byte, []byte, error) {
	nonce := make([]byte, cipher.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return cipher.aead.Seal(nil, nonce, plain, []byte(aad)), nonce, nil
}

func (cipher *tokenCipher) decrypt(encrypted, nonce []byte, aad string) ([]byte, error) {
	if len(nonce) != cipher.aead.NonceSize() {
		return nil, errors.New("invalid social token nonce")
	}
	plain, err := cipher.aead.Open(nil, nonce, encrypted, []byte(aad))
	if err != nil {
		return nil, errors.New("social token could not be decrypted; reconnect the Page")
	}
	return plain, nil
}
