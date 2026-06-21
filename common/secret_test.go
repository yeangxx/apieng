package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptSecretRoundTrip(t *testing.T) {
	originalSecret := CryptoSecret
	originalConfigured := PersistentCryptoSecretConfigured
	t.Cleanup(func() {
		CryptoSecret = originalSecret
		PersistentCryptoSecretConfigured = originalConfigured
	})
	CryptoSecret = "test-secret-with-enough-entropy"
	PersistentCryptoSecretConfigured = true

	encrypted, err := EncryptSecret("sk-test-secret")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encrypted, "v1:"))
	assert.NotContains(t, encrypted, "sk-test-secret")

	decrypted, err := DecryptSecret(encrypted)
	require.NoError(t, err)
	assert.Equal(t, "sk-test-secret", decrypted)
}

func TestEncryptSecretRequiresCryptoSecret(t *testing.T) {
	originalSecret := CryptoSecret
	originalConfigured := PersistentCryptoSecretConfigured
	t.Cleanup(func() {
		CryptoSecret = originalSecret
		PersistentCryptoSecretConfigured = originalConfigured
	})
	CryptoSecret = ""
	PersistentCryptoSecretConfigured = false

	_, err := EncryptSecret("sk-test-secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CRYPTO_SECRET")
}

func TestEncryptSecretRejectsSessionSecretFallback(t *testing.T) {
	originalSecret := CryptoSecret
	originalSessionSecret := SessionSecret
	originalConfigured := PersistentCryptoSecretConfigured
	t.Cleanup(func() {
		CryptoSecret = originalSecret
		SessionSecret = originalSessionSecret
		PersistentCryptoSecretConfigured = originalConfigured
	})
	SessionSecret = "session-secret-is-not-persistent-enough"
	CryptoSecret = SessionSecret
	PersistentCryptoSecretConfigured = false

	require.False(t, HasPersistentCryptoSecret())
	_, err := EncryptSecret("sk-test-secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CRYPTO_SECRET")
}

func TestFingerprintAndMaskSecret(t *testing.T) {
	originalSecret := CryptoSecret
	originalConfigured := PersistentCryptoSecretConfigured
	t.Cleanup(func() {
		CryptoSecret = originalSecret
		PersistentCryptoSecretConfigured = originalConfigured
	})
	CryptoSecret = "test-secret-with-enough-entropy"
	PersistentCryptoSecretConfigured = true

	fingerprintA := FingerprintSecret("sk-same-secret")
	fingerprintB := FingerprintSecret("sk-same-secret")
	fingerprintC := FingerprintSecret("sk-other-secret")

	assert.Equal(t, fingerprintA, fingerprintB)
	assert.NotEqual(t, fingerprintA, fingerprintC)
	assert.Equal(t, "sk-...cdef", MaskSecret("sk-abcdefghijklmnopqrstuvwxyzabcdef"))
	assert.Equal(t, "****", MaskSecret("test"))
}
