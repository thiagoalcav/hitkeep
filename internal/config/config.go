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
	HTTPAddr       string
	DBPath         string
	BindAddr       string
	JoinAddr       string
	NodeName       string
	LogLevel       string
	NSQTCPAddress  string
	NSQHTTPAddress string
	JWTSecret      string
	PublicURL      string
	Healthcheck    bool
}

func Load() *Config {
	var conf Config

	// Public / Cluster Settings
	flag.StringVar(&conf.HTTPAddr, "http", ":8080", "HTTP listen address for the API and Dashboard")
	flag.StringVar(&conf.BindAddr, "bind", "0.0.0.0:7946", "Address for cluster gossip (Memberlist)")
	flag.StringVar(&conf.JoinAddr, "join", "", "Address of a peer to join the cluster")
	flag.StringVar(&conf.PublicURL, "public-url", "http://localhost:8080", "Public URL of the instance (e.g. https://analytics.example.com). Used for JWT claims and Cookie security.")

	// Data / Logging Settings
	flag.StringVar(&conf.DBPath, "db", "hitkeep.db", "Database file path")
	flag.StringVar(&conf.LogLevel, "log-level", "info", "Log level (debug, info, warn, error)")

	// Auth
	flag.StringVar(&conf.JWTSecret, "jwt-secret", "", "Secret key for JWT signing (defaults to random if empty)")

	// Internal Embedded NSQ Settings (Leader Only)
	flag.StringVar(&conf.NSQTCPAddress, "nsq-tcp-address", "127.0.0.1:4150", "Internal address for embedded NSQ TCP listener")
	flag.StringVar(&conf.NSQHTTPAddress, "nsq-http-address", "127.0.0.1:4151", "Internal address for embedded NSQ HTTP listener")

	// Healthcheck Mode
	flag.BoolVar(&conf.Healthcheck, "healthcheck", false, "Run as a healthcheck client and exit")

	hostname, _ := os.Hostname()
	flag.StringVar(&conf.NodeName, "name", fmt.Sprintf("%s-%d", hostname, time.Now().UnixNano()), "Unique node name in cluster")
	flag.Parse()

	if conf.JWTSecret == "" {
		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err == nil {
			conf.JWTSecret = hex.EncodeToString(bytes)
		} else {
			// shouldnt really happen
			conf.JWTSecret = fmt.Sprintf("fallback-secret-%d", time.Now().UnixNano())
		}
	}

	return &conf
}
