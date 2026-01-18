package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ApiBurst               int
	ApiRateLimit           float64
	AuthBurst              int
	AuthRateLimit          float64
	ArchivePath            string
	BindAddr               string
	DBPath                 string
	Healthcheck            bool
	HTTPAddr               string
	IngestBurst            int
	IngestRateLimit        float64
	JoinAddr               string
	JWTSecret              string
	LogLevel               string
	MailDriver             string
	MailEncryption         string
	MailInsecureSkipVerify bool
	MailFromAddress        string
	MailFromName           string
	MailHost               string
	MailPassword           string
	MailPort               int
	MailUsername           string
	NodeName               string
	NSQHTTPAddress         string
	NSQTCPAddress          string
	PublicURL              string
	Version                string
	DataRetentionDays      int
	TrustedProxies         string
	trustedProxyNets       []*net.IPNet
}

// GetTrustedProxyNetworks returns the parsed trusted proxy networks.
func (c *Config) GetTrustedProxyNetworks() []*net.IPNet {
	return c.trustedProxyNets
}

// IsTrustedProxy checks if an IP is in the trusted proxy list.
func (c *Config) IsTrustedProxy(ip net.IP) bool {
	if len(c.trustedProxyNets) == 0 {
		return true
	}

	for _, network := range c.trustedProxyNets {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func Load() *Config {
	return load(os.Args[1:], func(key, fallback string) string {
		if val := os.Getenv(key); val != "" {
			return val
		}
		return fallback
	})
}

// load internal
func load(args []string, getEnv func(string, string) string) *Config {
	var conf Config

	fs := flag.NewFlagSet("hitkeep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	getInt := func(key string, fallback int) int {
		val := getEnv(key, "")
		if val != "" {
			if i, err := strconv.Atoi(val); err == nil {
				return i
			}
			slog.Warn("Invalid integer in env var, using default", "key", key, "val", val, "default", fallback)
		}
		return fallback
	}

	getFloat := func(key string, fallback float64) float64 {
		val := getEnv(key, "")
		if val != "" {
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f
			}
			slog.Warn("Invalid float in env var, using default", "key", key, "val", val, "default", fallback)
		}
		return fallback
	}

	getBool := func(key string, fallback bool) bool {
		val := getEnv(key, "")
		if val != "" {
			if b, err := strconv.ParseBool(val); err == nil {
				return b
			}
			slog.Warn("Invalid boolean in env var, using default", "key", key, "val", val, "default", fallback)
		}
		return fallback
	}

	defDB := getEnv("HITKEEP_DB_PATH", "hitkeep.db")
	defArchive := getEnv("HITKEEP_ARCHIVE_PATH", "archive")
	defHTTP := getEnv("HITKEEP_HTTP_ADDR", ":8080")
	defBind := getEnv("HITKEEP_BIND_ADDR", "0.0.0.0:7946")
	defJoin := getEnv("HITKEEP_JOIN_ADDR", "")
	defPublicURL := getEnv("HITKEEP_PUBLIC_URL", "http://localhost:8080")
	defLogLevel := getEnv("HITKEEP_LOG_LEVEL", "info")
	defJWT := getEnv("HITKEEP_JWT_SECRET", "")

	defNSQTCP := getEnv("HITKEEP_NSQ_TCP_ADDRESS", "127.0.0.1:4150")
	defNSQHTTP := getEnv("HITKEEP_NSQ_HTTP_ADDRESS", "127.0.0.1:4151")

	defIngestRate := getFloat("HITKEEP_INGEST_RATE_LIMIT", 20.0)
	defIngestBurst := getInt("HITKEEP_INGEST_BURST", 40)
	defApiRate := getFloat("HITKEEP_API_RATE_LIMIT", 10.0)
	defApiBurst := getInt("HITKEEP_API_BURST", 20)
	defAuthRate := getFloat("HITKEEP_AUTH_RATE_LIMIT", 2.0)
	defAuthBurst := getInt("HITKEEP_AUTH_BURST", 5)

	defMailDriver := getEnv("HITKEEP_MAIL_DRIVER", "smtp")
	defMailEnc := getEnv("HITKEEP_MAIL_ENCRYPTION", "tls")
	defMailSkipVerify := getBool("HITKEEP_MAIL_INSECURE_SKIP_VERIFY", false)
	defMailHost := getEnv("HITKEEP_MAIL_HOST", "")
	defMailPort := getInt("HITKEEP_MAIL_PORT", 587)
	defMailUser := getEnv("HITKEEP_MAIL_USERNAME", "")
	defMailPass := getEnv("HITKEEP_MAIL_PASSWORD", "")
	defMailFrom := getEnv("HITKEEP_MAIL_FROM_ADDRESS", "hitkeep@localhost")
	defMailName := getEnv("HITKEEP_MAIL_FROM_NAME", "HitKeep")

	defRetention := getInt("HITKEEP_DATA_RETENTION_DAYS", 365)
	defTrustedProxies := getEnv("HITKEEP_TRUSTED_PROXIES", "")

	hostname, _ := os.Hostname()
	defNodeName := getEnv("HITKEEP_NODE_NAME", fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()))

	fs.StringVar(&conf.HTTPAddr, "http", defHTTP, "HTTP listen address")
	fs.StringVar(&conf.BindAddr, "bind", defBind, "Address for cluster gossip")
	fs.StringVar(&conf.JoinAddr, "join", defJoin, "Address of a peer to join")
	fs.StringVar(&conf.PublicURL, "public-url", defPublicURL, "Public URL")

	fs.StringVar(&conf.DBPath, "db", defDB, "Database file path")
	fs.StringVar(&conf.ArchivePath, "archive-path", defArchive, "Data archive path")
	fs.StringVar(&conf.LogLevel, "log-level", defLogLevel, "Log level")
	fs.StringVar(&conf.JWTSecret, "jwt-secret", defJWT, "Secret key for JWT")

	fs.StringVar(&conf.NSQTCPAddress, "nsq-tcp-address", defNSQTCP, "Internal NSQ TCP")
	fs.StringVar(&conf.NSQHTTPAddress, "nsq-http-address", defNSQHTTP, "Internal NSQ HTTP")

	fs.BoolVar(&conf.Healthcheck, "healthcheck", false, "Run as healthcheck client")

	fs.Float64Var(&conf.IngestRateLimit, "ingest-rate", defIngestRate, "Ingest rate limit")
	fs.IntVar(&conf.IngestBurst, "ingest-burst", defIngestBurst, "Ingest burst")
	fs.Float64Var(&conf.ApiRateLimit, "api-rate", defApiRate, "API rate limit")
	fs.IntVar(&conf.ApiBurst, "api-burst", defApiBurst, "API burst")
	fs.Float64Var(&conf.AuthRateLimit, "auth-rate", defAuthRate, "Auth rate limit")
	fs.IntVar(&conf.AuthBurst, "auth-burst", defAuthBurst, "Auth burst")

	fs.StringVar(&conf.MailDriver, "mail-driver", defMailDriver, "Mail driver")
	fs.StringVar(&conf.MailEncryption, "mail-encryption", defMailEnc, "Mail encryption")
	fs.BoolVar(&conf.MailInsecureSkipVerify, "mail-insecure-skip-verify", defMailSkipVerify, "Disable Cert validation")
	fs.StringVar(&conf.MailHost, "mail-host", defMailHost, "SMTP Host")
	fs.IntVar(&conf.MailPort, "mail-port", defMailPort, "SMTP Port")
	fs.StringVar(&conf.MailUsername, "mail-username", defMailUser, "SMTP Username")
	fs.StringVar(&conf.MailPassword, "mail-password", defMailPass, "SMTP Password")
	fs.StringVar(&conf.MailFromAddress, "mail-from-address", defMailFrom, "From Email")
	fs.StringVar(&conf.MailFromName, "mail-from-name", defMailName, "From Name")

	fs.IntVar(&conf.DataRetentionDays, "retention-days", defRetention, "Default data retention in days")

	fs.StringVar(&conf.TrustedProxies, "trusted-proxies", defTrustedProxies, "Trusted proxy CIDRs (comma-separated)")

	fs.StringVar(&conf.NodeName, "name", defNodeName, "Unique node name")

	_ = fs.Parse(args)

	if conf.JWTSecret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err == nil {
			conf.JWTSecret = hex.EncodeToString(bytes)
		} else {
			conf.JWTSecret = fmt.Sprintf("fallback-secret-%d", time.Now().UnixNano())
		}
	}

	// Parse trusted proxies
	if conf.TrustedProxies != "" {
		conf.trustedProxyNets = parseTrustedProxies(conf.TrustedProxies)
		if len(conf.trustedProxyNets) > 0 {
			slog.Info("Loaded trusted proxy networks", "count", len(conf.trustedProxyNets))
		}
	}

	return &conf
}

// parseTrustedProxies parses a comma-separated list of CIDR ranges.
func parseTrustedProxies(cidrs string) []*net.IPNet {
	if cidrs == "" {
		return nil
	}

	parts := strings.Split(cidrs, ",")
	networks := make([]*net.IPNet, 0, len(parts))

	for _, cidr := range parts {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			slog.Warn("Invalid trusted proxy CIDR, skipping", "cidr", cidr, "error", err)
			continue
		}

		networks = append(networks, network)
	}

	return networks
}
