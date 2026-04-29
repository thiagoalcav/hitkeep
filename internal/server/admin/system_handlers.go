package admin

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/config"
	"hitkeep/internal/database"
)

func (h *handler) handleGetSystem() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.ctx.Config

		uptime := time.Since(h.ctx.StartedAt).String()
		info := api.SystemInfo{
			Version:         cfg.Version,
			RuntimeMode:     "oss",
			Uptime:          uptime,
			PublicURL:       cfg.PublicURL,
			EnabledFeatures: systemFeatureStatuses(cfg, h.ctx.Mailer != nil),
			ConfigFlags:     map[string]any{},
		}

		if buildInfo, ok := debug.ReadBuildInfo(); ok {
			info.Build = buildInfo.Main.Version
			for _, setting := range buildInfo.Settings {
				if setting.Key == "vcs.revision" {
					info.Build = setting.Value[:8]
				}
			}
		}

		writeJSON(w, http.StatusOK, info)
	}
}

func systemFeatureStatuses(cfg *config.Config, mailerConfigured bool) []api.SystemFeatureStatus {
	backupDetail := ""
	if cfg.BackupPath != "" {
		backupDetail = "local"
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(cfg.BackupPath)), "s3://") {
			backupDetail = "s3"
		}
	}

	mailDetail := ""
	if mailerConfigured {
		mailDetail = strings.TrimSpace(cfg.MailDriver)
		if mailDetail == "" {
			mailDetail = "smtp"
		}
	}

	cloudDetail := ""
	if cfg.CloudHosted {
		cloudDetail = strings.TrimSpace(cfg.CloudPlanName)
		if cloudDetail == "" {
			cloudDetail = strings.TrimSpace(cfg.CloudPlanCode)
		}
	}

	billingEnabled := cfg.CloudHosted && strings.TrimSpace(cfg.StripeSecretKey) != ""

	return []api.SystemFeatureStatus{
		{Key: "mcp", Enabled: cfg.MCPEnabled, Detail: enabledDetail(cfg.MCPEnabled, cfg.MCPPath)},
		{Key: "mcp_docs", Enabled: cfg.MCPEnabled && cfg.MCPDocsEnabled, Detail: enabledDetail(cfg.MCPEnabled && cfg.MCPDocsEnabled, cfg.MCPDocsURL)},
		{Key: "automatic_backups", Enabled: cfg.BackupPath != "", Detail: backupDetail},
		{Key: "spam_auto_update", Enabled: cfg.SpamFilterAutoUpdate, Detail: enabledDetail(cfg.SpamFilterAutoUpdate, formatFeatureInterval(cfg.SpamFilterUpdateIntervalMin))},
		{Key: "mail_delivery", Enabled: mailerConfigured, Detail: mailDetail},
		{Key: "managed_cloud", Enabled: cfg.CloudHosted, Detail: cloudDetail},
		{Key: "cloud_signup", Enabled: cfg.CloudHosted && cfg.CloudSignupEnabled},
		{Key: "billing", Enabled: billingEnabled, Detail: enabledDetail(billingEnabled, "stripe")},
	}
}

func enabledDetail(enabled bool, detail string) string {
	if !enabled {
		return ""
	}
	return strings.TrimSpace(detail)
}

func formatFeatureInterval(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes%1440 == 0 {
		return fmt.Sprintf("%dd", minutes/1440)
	}
	if minutes%60 == 0 {
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (h *handler) handleGetHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := api.SystemHealth{
			Status:   "healthy",
			Database: "ok",
			Workers:  "ok",
			IsLeader: true,
		}

		if h.ctx.Cluster != nil {
			health.IsLeader = h.ctx.Cluster.IsLeader()
		}

		if h.ctx.Store != nil {
			if err := h.ctx.Store.DB().Ping(); err != nil {
				health.Database = fmt.Sprintf("error: %v", err)
				health.Status = "degraded"
			}
		}

		writeJSON(w, http.StatusOK, health)
	}
}

func (h *handler) handleGetStorage() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		cfg := h.ctx.Config

		storage := api.SystemStorage{
			SharedDBPath: cfg.DBPath,
			DataPath:     cfg.DataPath,
			BackupPath:   cfg.BackupPath,
		}

		if fi, err := os.Stat(cfg.DBPath); err == nil {
			storage.SharedDBBytes = fi.Size()
		}

		tenants, err := h.ctx.Store.GetTenantList(ctx)
		if err == nil {
			storage.TenantDBCount = len(tenants)
			for i, t := range tenants {
				tenantPath := fmt.Sprintf("%s/tenants/%s/hitkeep.db", cfg.DataPath, t.TenantID.String())
				tenants[i].Path = tenantPath
				if fi, err := os.Stat(tenantPath); err == nil {
					tenants[i].Bytes = fi.Size()
				}
			}
			storage.TenantDBs = tenants
		}

		spamCachePath := cfg.SpamFilterPath
		if spamCachePath == "" {
			spamCachePath = cfg.DataPath + "/spam-filter.json"
		}
		storage.SpamCachePath = spamCachePath

		writeJSON(w, http.StatusOK, storage)
	}
}

func (h *handler) handleGetIngestStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		stats := api.SystemIngestStats{}

		since := time.Now().UTC().Add(-24 * time.Hour)

		if hits, err := h.ctx.Store.GetRecentHitsCount(ctx, since); err == nil {
			stats.RecentHits = hits
		}
		if events, err := h.ctx.Store.GetRecentEventsCount(ctx, since); err == nil {
			stats.RecentEvents = events
		}

		if h.ctx.SystemCounters != nil {
			stats.RecentRejections = int(h.ctx.SystemCounters.Rejections.Load())
			stats.RecentSpam = int(h.ctx.SystemCounters.Spam.Load())
		}

		secs := time.Since(since).Seconds()
		if secs > 0 {
			stats.HitsPerSecond = float64(stats.RecentHits) / secs
		}

		writeJSON(w, http.StatusOK, stats)
	}
}

func (h *handler) handleGetBackups() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := api.SystemBackupStatus{}
		if h.ctx.BackupStatus != nil {
			status = h.ctx.BackupStatus.Status()
		}

		writeJSON(w, http.StatusOK, status)
	}
}

func (h *handler) handleGetSpamFilter() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := api.SystemSpamStatus{
			AutoUpdate: h.ctx.Config.SpamFilterAutoUpdate,
		}

		spamPath := h.ctx.Config.SpamFilterPath
		if spamPath == "" {
			spamPath = h.ctx.Config.DataPath + "/spam-filter.json"
		}
		status.DBPath = spamPath

		if h.ctx.SpamFilter != nil {
			status.RuleCount = h.ctx.SpamFilter.RuleCount()
			status.LastRefresh = h.ctx.SpamFilter.LastRefresh()
		}

		writeJSON(w, http.StatusOK, status)
	}
}

func (h *handler) handleGetCaches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := api.SystemCacheStatus{
			PermissionsCache: api.SystemCacheEntry{
				Size:    0,
				MaxSize: 8192,
				TTL:     "30s",
			},
			APIClientCache: api.SystemCacheEntry{
				Size:    0,
				MaxSize: 4096,
				TTL:     "5m",
			},
			RateLimiterCache: api.SystemCacheEntry{
				Size:    0,
				MaxSize: 10000,
				TTL:     "3m",
			},
			Status: "healthy",
		}

		if h.ctx.Store != nil {
			permsSize := h.ctx.Store.CacheInstanceRoleSize()
			apiSize := h.ctx.Store.CacheAPIClientAuthSize()
			status.PermissionsCache.Size = permsSize
			status.APIClientCache.Size = apiSize

			if float64(permsSize) > float64(8192)*0.9 || float64(apiSize) > float64(4096)*0.9 {
				status.Status = "pressure"
			}
		}

		if h.ctx.ApiLimiter != nil {
			status.RateLimiterCache.Size = h.ctx.ApiLimiter.Len()
		}

		writeJSON(w, http.StatusOK, status)
	}
}

func (h *handler) handleGetMail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cfg := h.ctx.Config
		status := api.SystemMailStatus{
			Configured:  cfg.MailHost != "" || cfg.MailDriver != "",
			Driver:      cfg.MailDriver,
			Host:        cfg.MailHost,
			Port:        cfg.MailPort,
			Encryption:  cfg.MailEncryption,
			FromAddress: cfg.MailFromAddress,
			FromName:    cfg.MailFromName,
			PasswordSet: cfg.MailPassword != "",
		}

		if cfg.MailUsername != "" {
			if len(cfg.MailUsername) > 4 {
				status.Username = cfg.MailUsername[:4] + "****"
			} else {
				status.Username = cfg.MailUsername[:1] + "****"
			}
		}

		if h.ctx.MailTestTracker != nil {
			status.LastTestAt, status.LastTestOK = h.ctx.MailTestTracker.Status()
		}

		writeJSON(w, http.StatusOK, status)
	}
}

func (h *handler) handleListAudit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()

		filter := database.InstanceAuditFilter{
			Action:     strings.TrimSpace(q.Get("action")),
			TargetType: strings.TrimSpace(q.Get("target_type")),
			Outcome:    strings.TrimSpace(q.Get("outcome")),
			Query:      strings.TrimSpace(q.Get("query")),
		}

		if actorIDStr := strings.TrimSpace(q.Get("actor_id")); actorIDStr != "" {
			if actorID, err := uuid.Parse(actorIDStr); err == nil {
				filter.ActorID = actorID
			}
		}
		if fromStr := strings.TrimSpace(q.Get("from")); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				filter.From = t
			}
		}
		if toStr := strings.TrimSpace(q.Get("to")); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				filter.To = t
			}
		}
		if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
			if limit, err := strconv.Atoi(limitStr); err == nil {
				filter.Limit = limit
			}
		}
		if offsetStr := strings.TrimSpace(q.Get("offset")); offsetStr != "" {
			if offset, err := strconv.Atoi(offsetStr); err == nil {
				filter.Offset = offset
			}
		}

		entries, total, err := h.ctx.Store.ListInstanceAuditEntries(ctx, filter)
		if err != nil {
			slog.Error("Failed to list instance audit entries", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		limit := filter.Limit
		if limit <= 0 || limit > 200 {
			limit = 100
		}
		offset := filter.Offset
		if offset < 0 {
			offset = 0
		}

		resp := api.InstanceAuditListResponse{
			Entries: entries,
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+len(entries) < total,
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func (h *handler) handleExportAudit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		q := r.URL.Query()

		filter := database.InstanceAuditFilter{
			Action:     strings.TrimSpace(q.Get("action")),
			ActorID:    uuid.Nil,
			TargetType: strings.TrimSpace(q.Get("target_type")),
			Outcome:    strings.TrimSpace(q.Get("outcome")),
			Query:      strings.TrimSpace(q.Get("query")),
		}

		if actorIDStr := strings.TrimSpace(q.Get("actor_id")); actorIDStr != "" {
			if actorID, err := uuid.Parse(actorIDStr); err == nil {
				filter.ActorID = actorID
			}
		}
		if fromStr := strings.TrimSpace(q.Get("from")); fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				filter.From = t
			}
		}
		if toStr := strings.TrimSpace(q.Get("to")); toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				filter.To = t
			}
		}

		entries, err := h.ctx.Store.ExportInstanceAuditEntries(ctx, filter)
		if err != nil {
			slog.Error("Failed to export instance audit entries", "error", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		format := strings.TrimSpace(q.Get("format"))
		if format == "csv" {
			w.Header().Set("Content-Type", "text/csv")
			w.Header().Set("Content-Disposition", "attachment; filename=instance-audit-export.csv")

			writer := csv.NewWriter(w)
			if err := writer.Write([]string{"id", "created_at", "actor_id", "actor_email", "actor_role", "action", "target_type", "target_id", "target_label", "outcome", "ip_address", "details"}); err != nil {
				slog.Error("Failed to write instance audit CSV header", "error", err)
				return
			}

			for _, entry := range entries {
				actorID := ""
				if entry.ActorID != nil {
					actorID = entry.ActorID.String()
				}
				if err := writer.Write([]string{
					entry.ID.String(),
					entry.CreatedAt.Format(time.RFC3339),
					actorID,
					entry.ActorEmailSnapshot,
					entry.ActorRoleSnapshot,
					entry.Action,
					entry.TargetType,
					entry.TargetID,
					entry.TargetLabel,
					entry.Outcome,
					entry.IPAddress,
					entry.Details,
				}); err != nil {
					slog.Error("Failed to write instance audit CSV row", "error", err, "audit_id", entry.ID)
					return
				}
			}
			writer.Flush()
			if err := writer.Error(); err != nil {
				slog.Error("Failed to flush instance audit CSV", "error", err)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=instance-audit-export.json")
		if err := json.NewEncoder(w).Encode(entries); err != nil {
			slog.Error("Failed to encode instance audit export", "error", err)
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Failed to encode response", "error", err)
	}
}
