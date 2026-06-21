package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const secretEnvelopePrefix = "v1:"

var PersistentCryptoSecretConfigured bool

func HasPersistentCryptoSecret() bool {
	return PersistentCryptoSecretConfigured && strings.TrimSpace(CryptoSecret) != ""
}

func secretEncryptionKey() ([]byte, error) {
	if !HasPersistentCryptoSecret() {
		return nil, errors.New("CRYPTO_SECRET is required for Fusion secret storage")
	}
	sum := sha256.Sum256([]byte(CryptoSecret))
	return sum[:], nil
}

func EncryptSecret(plain string) (string, error) {
	if strings.TrimSpace(plain) == "" {
		return "", errors.New("secret is empty")
	}
	key, err := secretEncryptionKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return secretEnvelopePrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func DecryptSecret(envelope string) (string, error) {
	if !strings.HasPrefix(envelope, secretEnvelopePrefix) {
		return "", errors.New("unsupported secret envelope")
	}
	key, err := secretEncryptionKey()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(envelope, secretEnvelopePrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid secret envelope")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func FingerprintSecret(plain string) string {
	return GenerateHMACWithKey([]byte(CryptoSecret), plain)
}

func MaskSecret(plain string) string {
	if len(plain) <= 4 {
		return strings.Repeat("*", len(plain))
	}
	if len(plain) <= 8 {
		return plain[:2] + "..." + plain[len(plain)-2:]
	}
	return fmt.Sprintf("%s...%s", plain[:3], plain[len(plain)-4:])
}
