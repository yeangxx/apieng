package common

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type FusionBaseURLPolicy struct {
	AllowPrivateIP bool
	AllowedDomains []string
	AllowedPorts   []int
}

var fusionLookupIP = net.LookupIP

func ValidateFusionBaseURL(baseURL string, policy FusionBaseURLPolicy) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("base_url is required")
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("base_url must use https")
	}
	if parsed.User != nil {
		return "", fmt.Errorf("base_url must not contain userinfo")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("base_url must not contain query or fragment")
	}

	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", fmt.Errorf("base_url host is required")
	}

	port, err := fusionBaseURLPort(parsed)
	if err != nil {
		return "", err
	}
	if !fusionAllowedPort(port, policy.AllowedPorts) {
		return "", fmt.Errorf("base_url port %d is not allowed", port)
	}

	if len(policy.AllowedDomains) > 0 && net.ParseIP(host) == nil && !isDomainListed(host, policy.AllowedDomains) {
		return "", fmt.Errorf("base_url domain is not allowed: %s", host)
	}

	if err := validateFusionBaseURLHost(host, policy); err != nil {
		return "", err
	}

	normalized := *parsed
	normalized.Scheme = "https"
	normalized.User = nil
	normalized.RawQuery = ""
	normalized.Fragment = ""
	normalized.Host = fusionNormalizedHost(host, parsed.Port())
	normalized.Path = strings.TrimRight(parsed.Path, "/")
	return normalized.String(), nil
}

func fusionBaseURLPort(parsed *url.URL) (int, error) {
	portValue := parsed.Port()
	if portValue == "" {
		return 443, nil
	}
	port, err := strconv.Atoi(portValue)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("base_url port is invalid: %s", portValue)
	}
	return port, nil
}

func fusionAllowedPort(port int, allowedPorts []int) bool {
	if len(allowedPorts) == 0 {
		allowedPorts = []int{443}
	}
	for _, allowed := range allowedPorts {
		if port == allowed {
			return true
		}
	}
	return false
}

func validateFusionBaseURLHost(host string, policy FusionBaseURLPolicy) error {
	if ip := net.ParseIP(host); ip != nil {
		return validateFusionBaseURLIP(host, ip, policy)
	}

	ips, err := fusionLookupIP(host)
	if err != nil {
		return fmt.Errorf("base_url DNS resolution failed for %s: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("base_url DNS resolution returned no addresses for %s", host)
	}
	for _, ip := range ips {
		if err := validateFusionBaseURLIP(host, ip, policy); err != nil {
			return err
		}
	}
	return nil
}

func validateFusionBaseURLIP(host string, ip net.IP, policy FusionBaseURLPolicy) error {
	if ip == nil {
		return fmt.Errorf("base_url host %s resolved to invalid IP", host)
	}
	if isPrivateIP(ip) && !policy.AllowPrivateIP {
		return fmt.Errorf("base_url private IP is not allowed: %s resolves to %s", host, ip.String())
	}
	return nil
}

func fusionNormalizedHost(host, port string) string {
	normalizedHost := host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		normalizedHost = "[" + host + "]"
	}
	if port == "" {
		return normalizedHost
	}
	return net.JoinHostPort(host, port)
}
