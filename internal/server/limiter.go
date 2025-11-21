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

// getRealIP extracts the real IP, supporting X-Forwarded-For for proxies.
func getRealIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
