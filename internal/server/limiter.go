package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter manages rate limiters for individual IPs
type IPRateLimiter struct {
	ips   map[string]*visitor
	mu    sync.RWMutex
	rate  rate.Limit
	burst int
	stop  chan struct{}
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter creates a limiter and starts a background cleanup routine.
// r: requests per second
// b: burst size (allow short spikes of this many requests)
func NewIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	l := &IPRateLimiter{
		ips:   make(map[string]*visitor),
		rate:  r,
		burst: b,
		stop:  make(chan struct{}),
	}

	go l.cleanupLoop()

	return l
}

// GetLimiter returns the rate limiter for the provided IP.
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()

	v, exists := i.ips[ip]
	if !exists {
		limiter := rate.NewLimiter(i.rate, i.burst)
		i.ips[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupLoop removes old entries every minute to prevent memory leaks.
func (i *IPRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-i.stop:
			return
		case <-ticker.C:
			i.mu.Lock()
			for ip, v := range i.ips {
				// If IP hasn't been seen in 3 minutes, delete it to free memory
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(i.ips, ip)
				}
			}
			i.mu.Unlock()
		}
	}
}

func (i *IPRateLimiter) Stop() {
	close(i.stop)
}

// getRealIP extracts the real client IP using trusted proxy configuration.
func getRealIP(r *http.Request, trustedProxies []*net.IPNet) string {
	directIP := remoteIPFromAddr(r.RemoteAddr)
	parsedDirectIP := net.ParseIP(directIP)
	if parsedDirectIP == nil {
		if directIP != "" {
			return directIP
		}
		return r.RemoteAddr
	}

	if !isTrustedProxy(parsedDirectIP, trustedProxies) {
		return directIP
	}

	if ip := parseIPHeader(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if ip := parseIPHeader(r.Header.Get("Fastly-Client-IP")); ip != "" {
		return ip
	}
	if ip := parseIPHeader(r.Header.Get("CloudFront-Viewer-Address")); ip != "" {
		return ip
	}

	if ip := parseIPHeader(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(trustedProxies) == 0 {
			for _, part := range parts {
				if ip := parseIPHeader(strings.TrimSpace(part)); ip != "" {
					return ip
				}
			}
		} else {
			for i := len(parts) - 1; i >= 0; i-- {
				ip := strings.TrimSpace(parts[i])
				parsedIP := net.ParseIP(ip)
				if parsedIP == nil {
					continue
				}
				if !isIPInNetworks(parsedIP, trustedProxies) {
					return ip
				}
			}
		}
	}

	return directIP
}

func remoteIPFromAddr(addr string) string {
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func parseIPHeader(value string) string {
	if value == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		if isValidIP(host) {
			return host
		}
		return ""
	}
	if isValidIP(value) {
		return value
	}
	return ""
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func isTrustedProxy(ip net.IP, trustedProxies []*net.IPNet) bool {
	if len(trustedProxies) == 0 {
		return true
	}
	return isIPInNetworks(ip, trustedProxies)
}

// isIPInNetworks checks if an IP belongs to any of the provided networks.
func isIPInNetworks(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
