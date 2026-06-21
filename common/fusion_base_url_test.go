package common

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withFusionLookupIP(t *testing.T, lookup func(string) ([]net.IP, error)) {
	originalLookup := fusionLookupIP
	fusionLookupIP = lookup
	t.Cleanup(func() {
		fusionLookupIP = originalLookup
	})
}

func TestValidateFusionBaseURLAllowsPublicHTTPS(t *testing.T) {
	withFusionLookupIP(t, func(host string) ([]net.IP, error) {
		require.Equal(t, "example.com", host)
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	})

	normalized, err := ValidateFusionBaseURL("https://Example.COM/v1/", FusionBaseURLPolicy{
		AllowedPorts: []int{443},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://example.com/v1", normalized)
}

func TestValidateFusionBaseURLRejectsHTTP(t *testing.T) {
	_, err := ValidateFusionBaseURL("http://example.com", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "https")
}

func TestValidateFusionBaseURLRejectsUserinfo(t *testing.T) {
	_, err := ValidateFusionBaseURL("https://user:pass@example.com", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "userinfo")
}

func TestValidateFusionBaseURLRejectsPrivateIP(t *testing.T) {
	_, err := ValidateFusionBaseURL("https://127.0.0.1", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private IP")
}

func TestValidateFusionBaseURLAllowsPrivateIPWhenPolicyAllows(t *testing.T) {
	normalized, err := ValidateFusionBaseURL("https://10.0.0.10/v1", FusionBaseURLPolicy{
		AllowPrivateIP: true,
		AllowedPorts:   []int{443},
	})

	require.NoError(t, err)
	assert.Equal(t, "https://10.0.0.10/v1", normalized)
}

func TestValidateFusionBaseURLRejectsDisallowedPort(t *testing.T) {
	withFusionLookupIP(t, func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	})

	_, err := ValidateFusionBaseURL("https://example.com:8443", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "port 8443")
}

func TestValidateFusionBaseURLRejectsDomainOutsideAllowlist(t *testing.T) {
	withFusionLookupIP(t, func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	})

	_, err := ValidateFusionBaseURL("https://evil.com", FusionBaseURLPolicy{
		AllowedDomains: []string{"example.com"},
		AllowedPorts:   []int{443},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "domain is not allowed")
}

func TestValidateFusionBaseURLRejectsRedirectToPrivateIP(t *testing.T) {
	_, err := ValidateFusionBaseURL("https://127.0.0.1/v1", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private IP")
}

func TestValidateFusionBaseURLPreventsDNSRebinding(t *testing.T) {
	withFusionLookupIP(t, func(host string) ([]net.IP, error) {
		return []net.IP{
			net.ParseIP("93.184.216.34"),
			net.ParseIP("10.0.0.10"),
		}, nil
	})

	_, err := ValidateFusionBaseURL("https://example.com", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private IP")
}

func TestValidateFusionBaseURLRejectsDNSFailure(t *testing.T) {
	withFusionLookupIP(t, func(host string) ([]net.IP, error) {
		return nil, errors.New("lookup failed")
	})

	_, err := ValidateFusionBaseURL("https://example.com", FusionBaseURLPolicy{AllowedPorts: []int{443}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DNS resolution failed")
}
