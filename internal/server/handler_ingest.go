package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func (s *Server) handleIngest() http.HandlerFunc {
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

		if s.cluster.IsLeader() || !s.cluster.HasPeers() {
			s.handleIngestLeader(w, r)
		} else {
			s.handleIngestFollower(w, r)
		}
	}
}

func (s *Server) handleIngestLeader(w http.ResponseWriter, r *http.Request) {
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

	site, err := s.store.FindSiteByDomain(r.Context(), domain)
	if err != nil {
		slog.Error("Failed to find site", "error", err, "domain", domain)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if site == nil {
		slog.Warn("Dropped hit for unknown site", "domain", domain)
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
		IsUnique  bool      `json:"unique"`
		SessionID uuid.UUID `json:"session_id"`
		PageID    uuid.UUID `json:"page_id"`
	}

	var payload ingestPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
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
		IsUnique:       &payload.IsUnique,
	}

	body, _ := json.Marshal(hit)
	if err := s.producer.Publish("hits", body); err != nil {
		slog.Error("Failed to publish hit to NSQ", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleIngestFollower(w http.ResponseWriter, r *http.Request) {
	leaderIP := s.cluster.GetLeaderAddr()
	if leaderIP == "" {
		http.Error(w, "No leader available", http.StatusServiceUnavailable)
		return
	}

	_, port, err := net.SplitHostPort(s.conf.HTTPAddr)
	if err != nil {
		port = "8080"
	}

	forwardURL := fmt.Sprintf("http://%s:%s/ingest", leaderIP, port)
	bodyBytes := new(bytes.Buffer)
	if _, err := bodyBytes.ReadFrom(r.Body); err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	proxyReq, err := http.NewRequest(r.Method, forwardURL, bodyBytes)
	if err != nil {
		http.Error(w, "Failed to create forward request", http.StatusInternalServerError)
		return
	}

	proxyReq.Header.Set("Origin", r.Header.Get("Origin"))
	proxyReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		slog.Error("Follower failed to forward hit", "error", err, "target", forwardURL)
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
}
