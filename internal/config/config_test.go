package config

import (
	"net"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		env        map[string]string
		check      func(*Config) bool
		errMessage string
	}{
		{
			name: "Defaults",
			args: []string{},
			env:  map[string]string{},
			check: func(c *Config) bool {
				return c.HTTPAddr == ":8080" &&
					c.MailDriver == "smtp" &&
					c.IngestBurst == 40 &&
					len(c.JWTSecret) >= 32
			},
			errMessage: "Defaults failed",
		},
		{
			name: "Environment Variables Override Defaults",
			args: []string{},
			env: map[string]string{
				"HITKEEP_HTTP_ADDR":         ":9000",
				"HITKEEP_MAIL_PORT":         "25",
				"HITKEEP_INGEST_RATE_LIMIT": "100.5",
				"HITKEEP_MAIL_DRIVER":       "log",
			},
			check: func(c *Config) bool {
				return c.HTTPAddr == ":9000" &&
					c.MailPort == 25 &&
					c.IngestRateLimit == 100.5 &&
					c.MailDriver == "log"
			},
			errMessage: "Environment variables did not override defaults",
		},
		{
			name: "Flags Override Environment Variables",
			args: []string{"-http", ":9999", "-mail-port", "1025"},
			env: map[string]string{
				"HITKEEP_HTTP_ADDR": ":8080",
				"HITKEEP_MAIL_PORT": "587",
			},
			check: func(c *Config) bool {
				return c.HTTPAddr == ":9999" && c.MailPort == 1025
			},
			errMessage: "Flags did not override environment variables",
		},
		{
			name: "Boolean Flags and Env",
			args: []string{"-healthcheck"},
			env: map[string]string{
				"HITKEEP_MAIL_INSECURE_SKIP_VERIFY": "true",
			},
			check: func(c *Config) bool {
				return c.Healthcheck == true && c.MailInsecureSkipVerify == true
			},
			errMessage: "Boolean logic failed",
		},
		{
			name: "JWT Secret Generation",
			args: []string{},
			env:  map[string]string{},
			check: func(c *Config) bool {
				return c.JWTSecret != ""
			},
			errMessage: "JWT Secret was not generated",
		},
		{
			name: "JWT Secret Supplied",
			args: []string{},
			env: map[string]string{
				"HITKEEP_JWT_SECRET": "super-secret-key",
			},
			check: func(c *Config) bool {
				return c.JWTSecret == "super-secret-key"
			},
			errMessage: "Supplied JWT secret was ignored",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Mock Env Lookup
			mockEnv := func(key, fallback string) string {
				if val, ok := tc.env[key]; ok {
					return val
				}
				return fallback
			}

			// Run Logic
			conf := load(tc.args, mockEnv)

			if !tc.check(conf) {
				t.Errorf("%s: %s", tc.name, tc.errMessage)
			}
		})
	}
}

func TestTrustedProxiesDefaultIsWildcard(t *testing.T) {
	conf := load([]string{}, func(key, fallback string) string {
		return fallback
	})

	if conf.TrustedProxies != "*" {
		t.Fatalf("expected default trusted proxies to be '*', got %q", conf.TrustedProxies)
	}
	if len(conf.GetTrustedProxyNetworks()) == 0 {
		t.Fatalf("expected trust-all proxy networks to be loaded by default")
	}
	if !conf.IsTrustedProxy(net.ParseIP("203.0.113.10")) {
		t.Fatalf("expected default trusted proxies to trust public ipv4")
	}
}

func TestParseTrustedProxiesWildcard(t *testing.T) {
	networks := parseTrustedProxies("*")
	if len(networks) == 0 {
		t.Fatalf("expected wildcard to parse into trust-all proxy networks")
	}

	if !isIPInNetworksForTest(net.ParseIP("198.51.100.20"), networks) {
		t.Fatalf("expected wildcard trusted proxies to include public ipv4")
	}
	if !isIPInNetworksForTest(net.ParseIP("2001:db8::1"), networks) {
		t.Fatalf("expected wildcard trusted proxies to include ipv6")
	}
}

func isIPInNetworksForTest(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
