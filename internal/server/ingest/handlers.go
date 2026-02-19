package ingest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/server/shared"
)

type handler struct {
	ctx *shared.Context
}

var (
	forwardedHostPattern = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*)$`)
	leaderForwardClient  = &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

func Register(mux *http.ServeMux, ctx *shared.Context) {
	h := &handler{ctx: ctx}
	mux.HandleFunc("POST /ingest", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.IngestLimiter,
	}, h.handleIngest()))
	mux.HandleFunc("OPTIONS /ingest", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.IngestLimiter,
	}, h.handleIngest()))
	mux.HandleFunc("POST /ingest/event", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.IngestLimiter,
	}, h.handleIngestEvent()))
	mux.HandleFunc("OPTIONS /ingest/event", ctx.Handler(shared.HandlerConfig{
		RateLimiter: ctx.IngestLimiter,
	}, h.handleIngestEvent()))
}

func (h *handler) handleIngest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}

		if h.ctx.Cluster.IsLeader() {
			h.handleIngestLeader(w, r)
		} else {
			h.handleIngestFollower(w, r)
		}
	}
}

func (h *handler) handleIngestLeader(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		http.Error(w, "Origin header is required", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(origin)
	if err != nil {
		http.Error(w, "Invalid Origin header", http.StatusBadRequest)
		return
	}
	domain := strings.TrimPrefix(parsedURL.Hostname(), "www.")

	site, err := h.ctx.Store.FindSiteByDomain(r.Context(), domain)
	if err != nil {
		slog.Error("Failed to find site", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if site == nil {
		slog.Warn("Dropped hit for unknown site")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	userIP := shared.GetRealIP(r, h.ctx.Config.GetTrustedProxyNetworks())
	if h.ctx.IPFilter != nil && h.ctx.IPFilter.IsBlocked(site.ID, userIP) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	type ingestPayload struct {
		Path      string    `json:"path"`
		Referrer  *string   `json:"referrer"`
		UserAgent *string   `json:"ua"`
		VPWidth   *int      `json:"vp_w"`
		VPHeight  *int      `json:"vp_h"`
		SCWidth   *int      `json:"sc_w"`
		SCHeight  *int      `json:"sc_h"`
		Language  *string   `json:"lang"`
		UTMSource *string   `json:"u_src"`
		UTMMedium *string   `json:"u_med"`
		UTMCamp   *string   `json:"u_cmp"`
		UTMTerm   *string   `json:"u_trm"`
		UTMCont   *string   `json:"u_cnt"`
		IsUnique  bool      `json:"unique"`
		SessionID uuid.UUID `json:"session_id"`
		PageID    uuid.UUID `json:"page_id"`
	}

	var payload ingestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	extractor := NewCountryCodeExtractor(h.ctx.Config.GetTrustedProxyNetworks())
	countryCode := extractor.ExtractFromRequest(r, payload.Language)

	var countryCodePtr *string
	if countryCode != "" {
		countryCodePtr = &countryCode
	}

	hit := api.Hit{
		SiteID:         site.ID,
		SessionID:      payload.SessionID,
		PageID:         payload.PageID,
		Timestamp:      time.Now().UTC(),
		Path:           payload.Path,
		Referrer:       payload.Referrer,
		UserAgent:      payload.UserAgent,
		ViewportWidth:  payload.VPWidth,
		ViewportHeight: payload.VPHeight,
		ScreenWidth:    payload.SCWidth,
		ScreenHeight:   payload.SCHeight,
		Language:       payload.Language,
		CountryCode:    countryCodePtr,
		UTMSource:      payload.UTMSource,
		UTMMedium:      payload.UTMMedium,
		UTMCampaign:    payload.UTMCamp,
		UTMTerm:        payload.UTMTerm,
		UTMContent:     payload.UTMCont,
		IsUnique:       &payload.IsUnique,
	}

	body, _ := json.Marshal(hit)
	if err := h.ctx.Producer.Publish("hits", body); err != nil {
		slog.Error("Failed to publish hit to NSQ", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *handler) forwardToLeader(w http.ResponseWriter, r *http.Request, targetPath string) {
	forwardURL, err := buildForwardURL(h.ctx.Cluster.GetLeaderAddr(), h.ctx.Config.HTTPAddr, targetPath)
	if err != nil {
		http.Error(w, "No leader available", http.StatusServiceUnavailable)
		return
	}
	bodyBytes := new(bytes.Buffer)
	if _, err := bodyBytes.ReadFrom(r.Body); err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, forwardURL.String(), bodyBytes)
	if err != nil {
		http.Error(w, "Failed to create forward request", http.StatusInternalServerError)
		return
	}

	proxyReq.Header = r.Header.Clone()
	proxyReq.Header.Set("Content-Type", "application/json")
	appendForwardedFor(proxyReq.Header, r.RemoteAddr)

	//nolint:gosec // proxyReq target is validated via buildForwardURL and constrained to cluster leader ingest endpoints.
	resp, err := leaderForwardClient.Do(proxyReq)
	if err != nil {
		slog.Error("Follower failed to forward request", "error", err)
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
}

func (h *handler) handleIngestFollower(w http.ResponseWriter, r *http.Request) {
	h.forwardToLeader(w, r, "/ingest")
}

func (h *handler) handleIngestEvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusOK)
			return
		}

		if h.ctx.Cluster.IsLeader() {
			h.handleIngestEventLeader(w, r)
		} else {
			h.handleIngestEventFollower(w, r)
		}
	}
}

func (h *handler) handleIngestEventLeader(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		http.Error(w, "Origin header is required", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(origin)
	if err != nil {
		http.Error(w, "Invalid Origin header", http.StatusBadRequest)
		return
	}
	domain := strings.TrimPrefix(parsedURL.Hostname(), "www.")

	site, err := h.ctx.Store.FindSiteByDomain(r.Context(), domain)
	if err != nil {
		slog.Error("Failed to find site", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if site == nil {
		slog.Warn("Dropped event for unknown site")
		w.WriteHeader(http.StatusAccepted)
		return
	}

	userIP := shared.GetRealIP(r, h.ctx.Config.GetTrustedProxyNetworks())
	if h.ctx.IPFilter != nil && h.ctx.IPFilter.IsBlocked(site.ID, userIP) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	type eventPayload struct {
		Name       string         `json:"n"`
		Properties map[string]any `json:"p"`
		SessionID  uuid.UUID      `json:"sid"`
	}

	var payload eventPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	event := api.Event{
		SiteID:     site.ID,
		SessionID:  payload.SessionID,
		Name:       payload.Name,
		Properties: payload.Properties,
		Timestamp:  time.Now().UTC(),
	}

	body, _ := json.Marshal(event)
	if err := h.ctx.Producer.Publish("events", body); err != nil {
		slog.Error("Failed to publish event to NSQ", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *handler) handleIngestEventFollower(w http.ResponseWriter, r *http.Request) {
	h.forwardToLeader(w, r, "/ingest/event")
}

func normalizeLeaderHost(addr string) string {
	if addr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(addr)
	if err == nil {
		return host
	}

	return addr
}

func buildForwardURL(leaderAddr, httpAddr, targetPath string) (*url.URL, error) {
	if targetPath != "/ingest" && targetPath != "/ingest/event" {
		return nil, fmt.Errorf("invalid forward target")
	}

	leaderHost := normalizeLeaderHost(leaderAddr)
	if !isValidForwardHost(leaderHost) {
		return nil, fmt.Errorf("invalid leader address")
	}

	_, port, err := net.SplitHostPort(httpAddr)
	if err != nil || port == "" {
		port = "8080"
	}

	return &url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(leaderHost, port),
		Path:   targetPath,
	}, nil
}

func isValidForwardHost(host string) bool {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, `/\?#`) {
		return false
	}
	if net.ParseIP(trimmed) != nil {
		return true
	}
	return forwardedHostPattern.MatchString(trimmed)
}

func appendForwardedFor(headers http.Header, remoteAddr string) {
	ip := shared.RemoteIPFromAddr(remoteAddr)
	if ip == "" {
		return
	}

	existing := strings.TrimSpace(headers.Get("X-Forwarded-For"))
	if existing == "" {
		headers.Set("X-Forwarded-For", ip)
		return
	}

	headers.Set("X-Forwarded-For", existing+", "+ip)
}
