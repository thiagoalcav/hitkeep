package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddr        string
	DBPath          string
	BindAddr        string
	JoinAddr        string
	NodeName        string
	LogLevel        string
	NSQTCPAddress   string
	NSQHTTPAddress  string
	JWTSecret       string
	PublicURL       string
	Healthcheck     bool
	IngestRateLimit float64
	IngestBurst     int
	ApiRateLimit    float64
	ApiBurst        int
	AuthRateLimit   float64
	AuthBurst       int
}

func Load() *Config {
	var conf Config

	// Public / Cluster Settings
	flag.StringVar(&conf.HTTPAddr, "http", ":8080", "HTTP listen address")
	flag.StringVar(&conf.BindAddr, "bind", "0.0.0.0:7946", "Address for cluster gossip")
	flag.StringVar(&conf.JoinAddr, "join", "", "Address of a peer to join")
	flag.StringVar(&conf.PublicURL, "public-url", "http://localhost:8080", "Public URL (used for JWT issuer)")

	// Data / Logging
	flag.StringVar(&conf.DBPath, "db", "hitkeep.db", "Database file path")
	flag.StringVar(&conf.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&conf.JWTSecret, "jwt-secret", "", "Secret key for JWT signing")

	// NSQ
	flag.StringVar(&conf.NSQTCPAddress, "nsq-tcp-address", "127.0.0.1:4150", "Internal NSQ TCP address")
	flag.StringVar(&conf.NSQHTTPAddress, "nsq-http-address", "127.0.0.1:4151", "Internal NSQ HTTP address")

	// Healthcheck
	flag.BoolVar(&conf.Healthcheck, "healthcheck", false, "Run as healthcheck client")

	// Rate Limits
	flag.Float64Var(&conf.IngestRateLimit, "ingest-rate", 20.0, "Ingest endpoint rate limit (req/sec/ip)")
	flag.IntVar(&conf.IngestBurst, "ingest-burst", 40, "Ingest endpoint burst size")
	flag.Float64Var(&conf.ApiRateLimit, "api-rate", 10.0, "General API rate limit (req/sec/ip)")
	flag.IntVar(&conf.ApiBurst, "api-burst", 20, "General API burst size")
	flag.Float64Var(&conf.AuthRateLimit, "auth-rate", 2.0, "Auth endpoint rate limit (req/sec/ip)")
	flag.IntVar(&conf.AuthBurst, "auth-burst", 5, "Auth endpoint burst size")

	hostname, _ := os.Hostname()
	flag.StringVar(&conf.NodeName, "name", fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()), "Unique node name")
	flag.Parse()

	if conf.JWTSecret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err == nil {
			conf.JWTSecret = hex.EncodeToString(bytes)
		} else {
			conf.JWTSecret = fmt.Sprintf("fallback-secret-%d", time.Now().UnixNano())
		}
	}

	return &conf
}
