package service

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"strings"
)

type AESGCMSecretCipher struct{ Key []byte }

func NewAESGCMSecretCipher(value string) AESGCMSecretCipher {
	value = strings.TrimSpace(value)
	if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
		return AESGCMSecretCipher{Key: decoded}
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil && len(decoded) == 32 {
		return AESGCMSecretCipher{Key: decoded}
	}
	return AESGCMSecretCipher{}
}

func (c AESGCMSecretCipher) Encrypt(plaintext string) (string, error) {
	gcm, err := c.aead()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c AESGCMSecretCipher) Decrypt(ciphertext string) (string, error) {
	gcm, err := c.aead()
	if err != nil {
		return "", err
	}
	sealed, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil || len(sealed) < gcm.NonceSize() {
		return "", errors.New("invalid encrypted SSH credential")
	}
	plaintext, err := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("invalid encrypted SSH credential")
	}
	return string(plaintext), nil
}

func (c AESGCMSecretCipher) aead() (cipher.AEAD, error) {
	if len(c.Key) != 32 {
		return nil, errors.New("SLAN_NODE_SSH_CREDENTIAL_KEY must be 32-byte hex or base64")
	}
	block, err := aes.NewCipher(c.Key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
