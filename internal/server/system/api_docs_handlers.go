package system

import (
	"encoding/json"
	"net/http"
	"strings"

	"hitkeep/internal/exportfmt"
)

func (h *handler) handleGetAPIDocVersions() http.HandlerFunc {
	type versionInfo struct {
		Version    string `json:"version"`
		OpenAPIURL string `json:"openapi_url"`
		Latest     bool   `json:"latest"`
	}
	type response struct {
		Latest   string        `json:"latest"`
		Versions []versionInfo `json:"versions"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		resp := response{
			Latest: "v1",
			Versions: []versionInfo{
				{
					Version:    "v1",
					OpenAPIURL: "/api/docs/v1/openapi.json",
					Latest:     true,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (h *handler) handleGetAPIDocV1() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		publicURL := strings.TrimSpace(h.ctx.Config.PublicURL)
		if publicURL == "" {
			publicURL = "http://localhost:8080"
		}

		spec := openAPISpecV1(publicURL)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	}
}

func openAPISpecV1(publicURL string) map[string]any {
	return map[string]any{
		"openapi":           "3.1.0",
		"jsonSchemaDialect": "https://spec.openapis.org/oas/3.1/dialect/base",
		"info": map[string]any{
			"title":       "HitKeep REST API",
			"version":     "v1",
			"description": "Complete HTTP API for HitKeep (session, API key, and public share-token endpoints).",
		},
		"servers": []map[string]string{{"url": publicURL}},
		"tags": []map[string]string{
			{"name": "System", "description": "Service health and API documentation endpoints."},
			{"name": "Ingest", "description": "Public tracking and event ingestion endpoints."},
			{"name": "Auth", "description": "Authentication, password, and sign-in flows."},
			{"name": "User", "description": "Authenticated user profile, preferences, and security endpoints."},
			{"name": "Permissions", "description": "Authenticated permission context endpoints."},
			{"name": "Admin", "description": "Instance-level admin and membership management endpoints."},
			{"name": "Sites", "description": "Site lifecycle, stats, hits, and retention endpoints."},
			{"name": "Goals", "description": "Goal and goal-timeseries endpoints."},
			{"name": "Funnels", "description": "Funnel CRUD and analytics endpoints."},
			{"name": "Share", "description": "Share-link management and public shared analytics endpoints."},
			{"name": "Takeout", "description": "Data export endpoints for user and site data."},
			{"name": "Reports", "description": "Report subscription endpoints for digest and per-site scheduled analytics emails."},
			{"name": "Teams", "description": "Tenant team membership and active-team context endpoints."},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"cookieAuth": map[string]any{
					"type":        "apiKey",
					"in":          "cookie",
					"name":        "hk_token",
					"description": "Session cookie authentication.",
				},
				"bearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "APIClientToken",
					"description":  "API client token in Authorization header.",
				},
				"apiKeyAuth": map[string]any{
					"type":        "apiKey",
					"in":          "header",
					"name":        "X-API-Key",
					"description": "API client token in X-API-Key header.",
				},
			},
			"parameters": map[string]any{
				"siteID":        map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"goalID":        map[string]any{"name": "goalID", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"funnelID":      map[string]any{"name": "funnelID", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"shareID":       map[string]any{"name": "shareID", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"ruleID":        map[string]any{"name": "ruleID", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"userID":        map[string]any{"name": "userId", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"teamID":        map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"adminUserID":   map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"apiClientID":   map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"passkeyID":     map[string]any{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				"token":         map[string]any{"name": "token", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
				"domain":        map[string]any{"name": "domain", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
				"from":          map[string]any{"name": "from", "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}},
				"to":            map[string]any{"name": "to", "in": "query", "schema": map[string]any{"type": "string", "format": "date-time"}},
				"limit":         map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}},
				"offset":        map[string]any{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 0}},
				"query":         map[string]any{"name": "q", "in": "query", "schema": map[string]any{"type": "string"}},
				"sort":          map[string]any{"name": "sort", "in": "query", "schema": map[string]any{"type": "string"}},
				"order":         map[string]any{"name": "order", "in": "query", "schema": map[string]any{"type": "string", "enum": []string{"asc", "desc"}}},
				"filter":        map[string]any{"name": "filter", "in": "query", "description": "Filter in form type:value (repeatable).", "schema": map[string]any{"type": "string"}},
				"filterType":    map[string]any{"name": "filter_type", "in": "query", "schema": map[string]any{"type": "string"}},
				"filterValue":   map[string]any{"name": "filter_value", "in": "query", "schema": map[string]any{"type": "string"}},
				"goalIDQuery":   map[string]any{"name": "goal_id", "in": "query", "schema": map[string]any{"type": "string", "format": "uuid"}},
				"funnelIDQuery": map[string]any{"name": "funnel_id", "in": "query", "schema": map[string]any{"type": "string", "format": "uuid"}},
				"format": map[string]any{
					"name":        "format",
					"in":          "query",
					"description": "Export format. Supported values: xlsx, csv, parquet, json, ndjson. Defaults: xlsx for takeout endpoints, csv for hits export endpoints.",
					"schema":      map[string]any{"type": "string", "enum": exportfmt.SupportedFormats()},
				},
				"avatarSize":   map[string]any{"name": "s", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 32, "maximum": 256}},
				"reportSiteID": map[string]any{"name": "site_id", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}, "description": "Site UUID."},
			},
			"schemas": map[string]any{
				"Error": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"message": map[string]any{"type": "string"},
					},
				},
				"Status": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":  map[string]any{"type": "string"},
						"message": map[string]any{"type": "string"},
					},
				},
				"AdminDeleteUserBlockedResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":  map[string]any{"type": "string"},
						"code":    map[string]any{"type": "string"},
						"message": map[string]any{"type": "string"},
						"teams":   map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Team"}},
					},
					"required": []string{"status", "code", "message", "teams"},
				},
				"SiteTransferResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":              map[string]any{"type": "string"},
						"site_id":             map[string]any{"type": "string", "format": "uuid"},
						"source_team_id":      map[string]any{"type": "string", "format": "uuid"},
						"destination_team_id": map[string]any{"type": "string", "format": "uuid"},
					},
				},
				"Site": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":                  map[string]any{"type": "string", "format": "uuid"},
						"user_id":             map[string]any{"type": "string", "format": "uuid"},
						"domain":              map[string]any{"type": "string"},
						"data_retention_days": map[string]any{"type": "integer"},
						"created_at":          map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"ShareLink": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "format": "uuid"},
						"site_id":    map[string]any{"type": "string", "format": "uuid"},
						"token_hint": map[string]any{"type": "string"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"Hit": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":              map[string]any{"type": "string", "format": "uuid"},
						"site_id":         map[string]any{"type": "string", "format": "uuid"},
						"session_id":      map[string]any{"type": "string", "format": "uuid"},
						"page_id":         map[string]any{"type": "string", "format": "uuid"},
						"timestamp":       map[string]any{"type": "string", "format": "date-time"},
						"path":            map[string]any{"type": "string"},
						"referrer":        map[string]any{"type": "string"},
						"user_agent":      map[string]any{"type": "string"},
						"viewport_width":  map[string]any{"type": "integer"},
						"viewport_height": map[string]any{"type": "integer"},
						"screen_width":    map[string]any{"type": "integer"},
						"screen_height":   map[string]any{"type": "integer"},
						"language":        map[string]any{"type": "string"},
						"country_code":    map[string]any{"type": "string"},
						"utm_source":      map[string]any{"type": "string"},
						"utm_medium":      map[string]any{"type": "string"},
						"utm_campaign":    map[string]any{"type": "string"},
						"utm_term":        map[string]any{"type": "string"},
						"utm_content":     map[string]any{"type": "string"},
						"is_unique":       map[string]any{"type": "boolean"},
					},
				},
				"PaginatedHits": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"data":  map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Hit"}},
						"total": map[string]any{"type": "integer"},
					},
				},
				"MetricStat": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name":  map[string]any{"type": "string"},
						"value": map[string]any{"type": "integer"},
					},
				},
				"ChartDataPoint": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"time":      map[string]any{"type": "string", "format": "date-time"},
						"pageviews": map[string]any{"type": "integer"},
						"visitors":  map[string]any{"type": "integer"},
					},
				},
				"GoalStats": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"goal_id":         map[string]any{"type": "string", "format": "uuid"},
						"name":            map[string]any{"type": "string"},
						"conversions":     map[string]any{"type": "integer"},
						"conversion_rate": map[string]any{"type": "number"},
					},
				},
				"SiteStats": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"live_visitors":        map[string]any{"type": "integer"},
						"total_pageviews":      map[string]any{"type": "integer"},
						"unique_sessions":      map[string]any{"type": "integer"},
						"bounce_rate":          map[string]any{"type": "number"},
						"avg_session_duration": map[string]any{"type": "number"},
						"pages_per_session":    map[string]any{"type": "number"},
						"chart_data":           map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ChartDataPoint"}},
						"top_pages":            map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_referrers":        map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_devices":          map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_countries":        map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_utm_campaigns":    map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_utm_contents":     map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_utm_mediums":      map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_utm_sources":      map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"top_utm_terms":        map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/MetricStat"}},
						"utm_campaign_hits":    map[string]any{"type": "integer"},
						"utm_content_hits":     map[string]any{"type": "integer"},
						"utm_medium_hits":      map[string]any{"type": "integer"},
						"utm_source_hits":      map[string]any{"type": "integer"},
						"utm_term_hits":        map[string]any{"type": "integer"},
						"goals":                map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/GoalStats"}},
					},
				},
				"Goal": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "format": "uuid"},
						"site_id":    map[string]any{"type": "string", "format": "uuid"},
						"name":       map[string]any{"type": "string"},
						"type":       map[string]any{"type": "string", "enum": []string{"event", "path"}},
						"value":      map[string]any{"type": "string"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"GoalSeriesPoint": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"time":        map[string]any{"type": "string", "format": "date-time"},
						"conversions": map[string]any{"type": "integer"},
					},
				},
				"FunnelStep": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"type":  map[string]any{"type": "string", "enum": []string{"event", "path"}},
						"value": map[string]any{"type": "string"},
					},
				},
				"Funnel": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "format": "uuid"},
						"site_id":    map[string]any{"type": "string", "format": "uuid"},
						"name":       map[string]any{"type": "string"},
						"steps":      map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/FunnelStep"}},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"FunnelSeriesPoint": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"time":        map[string]any{"type": "string", "format": "date-time"},
						"entries":     map[string]any{"type": "integer"},
						"completions": map[string]any{"type": "integer"},
					},
				},
				"FunnelStepStats": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"step_index":      map[string]any{"type": "integer"},
						"name":            map[string]any{"type": "string"},
						"visitors":        map[string]any{"type": "integer"},
						"dropoff":         map[string]any{"type": "integer"},
						"conversion_rate": map[string]any{"type": "number"},
					},
				},
				"FunnelStats": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"funnel_id":               map[string]any{"type": "string", "format": "uuid"},
						"name":                    map[string]any{"type": "string"},
						"steps":                   map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/FunnelStepStats"}},
						"total_entries":           map[string]any{"type": "integer"},
						"total_completions":       map[string]any{"type": "integer"},
						"overall_conversion_rate": map[string]any{"type": "number"},
					},
				},
				"SiteMember": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":       map[string]any{"type": "string", "format": "uuid"},
						"user_id":  map[string]any{"type": "string", "format": "uuid"},
						"email":    map[string]any{"type": "string", "format": "email"},
						"role":     map[string]any{"type": "string"},
						"added_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"Team": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "format": "uuid"},
						"name":       map[string]any{"type": "string"},
						"logo_url":   map[string]any{"type": "string"},
						"role":       map[string]any{"type": "string"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"TeamMember": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":       map[string]any{"type": "string", "format": "uuid"},
						"user_id":  map[string]any{"type": "string", "format": "uuid"},
						"email":    map[string]any{"type": "string", "format": "email"},
						"role":     map[string]any{"type": "string"},
						"added_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"TeamInvite": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":              map[string]any{"type": "string", "format": "uuid"},
						"team_id":         map[string]any{"type": "string", "format": "uuid"},
						"email":           map[string]any{"type": "string", "format": "email"},
						"role":            map[string]any{"type": "string"},
						"invited_user_id": map[string]any{"type": "string", "format": "uuid"},
						"status":          map[string]any{"type": "string"},
						"created_by":      map[string]any{"type": "string", "format": "uuid"},
						"created_at":      map[string]any{"type": "string", "format": "date-time"},
						"expires_at":      map[string]any{"type": "string", "format": "date-time"},
						"accepted_at":     map[string]any{"type": "string", "format": "date-time"},
						"revoked_at":      map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"TeamAuditEntry": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":             map[string]any{"type": "string", "format": "uuid"},
						"team_id":        map[string]any{"type": "string", "format": "uuid"},
						"action":         map[string]any{"type": "string"},
						"details":        map[string]any{"type": "string"},
						"actor_user_id":  map[string]any{"type": "string", "format": "uuid"},
						"actor_email":    map[string]any{"type": "string", "format": "email"},
						"target_user_id": map[string]any{"type": "string", "format": "uuid"},
						"target_email":   map[string]any{"type": "string", "format": "email"},
						"created_at":     map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"TeamAuditListResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"entries": map[string]any{
							"type":  "array",
							"items": map[string]any{"$ref": "#/components/schemas/TeamAuditEntry"},
						},
						"total":    map[string]any{"type": "integer"},
						"limit":    map[string]any{"type": "integer"},
						"offset":   map[string]any{"type": "integer"},
						"has_more": map[string]any{"type": "boolean"},
						"action":   map[string]any{"type": "string"},
					},
				},
				"TeamListResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"active_team_id": map[string]any{"type": "string", "format": "uuid"},
						"recent_team_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string", "format": "uuid"},
						},
						"teams": map[string]any{
							"type":  "array",
							"items": map[string]any{"$ref": "#/components/schemas/Team"},
						},
					},
				},
				"TeamActiveResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":         map[string]any{"type": "string"},
						"active_team_id": map[string]any{"type": "string", "format": "uuid"},
						"recent_team_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string", "format": "uuid"},
						},
					},
				},
				"TeamCreateResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"team": map[string]any{"$ref": "#/components/schemas/Team"},
					},
				},
				"TeamLeaveResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":         map[string]any{"type": "string"},
						"active_team_id": map[string]any{"type": "string", "format": "uuid"},
						"recent_team_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string", "format": "uuid"},
						},
					},
				},
				"TeamArchiveResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":         map[string]any{"type": "string"},
						"active_team_id": map[string]any{"type": "string", "format": "uuid"},
						"recent_team_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string", "format": "uuid"},
						},
					},
				},
				"AdminDeleteTeamResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":  map[string]any{"type": "string"},
						"team_id": map[string]any{"type": "string", "format": "uuid"},
						"name":    map[string]any{"type": "string"},
					},
				},
				"IPExclusion": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":          map[string]any{"type": "string", "format": "uuid"},
						"site_id":     map[string]any{"type": "string", "format": "uuid"},
						"cidr":        map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
						"created_at":  map[string]any{"type": "string", "format": "date-time"},
						"created_by":  map[string]any{"type": "string", "format": "uuid"},
					},
				},
				"IPExclusionCreateRequest": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"cidr":        map[string]any{"type": "string", "description": "IP or CIDR value. Plain IP values are normalized to /32 (IPv4) or /128 (IPv6)."},
						"description": map[string]any{"type": "string", "maxLength": 255},
					},
					"required": []string{"cidr"},
				},
				"UserProfile": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":           map[string]any{"type": "string", "format": "uuid"},
						"email":        map[string]any{"type": "string", "format": "email"},
						"given_name":   map[string]any{"type": "string"},
						"last_name":    map[string]any{"type": "string"},
						"display_name": map[string]any{"type": "string"},
						"avatar_url":   map[string]any{"type": "string"},
					},
				},
				"UserPreferences": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"default_locale": map[string]any{"type": "string"},
					},
				},
				"UserPasskey": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "format": "uuid"},
						"name":       map[string]any{"type": "string"},
						"created_at": map[string]any{"type": "string", "format": "date-time"},
						"updated_at": map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"UserSecurityStatus": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"totp_enabled": map[string]any{"type": "boolean"},
						"totp_pending": map[string]any{"type": "boolean"},
						"passkeys":     map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/UserPasskey"}},
					},
				},
				"UserTOTPSetup": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"secret":      map[string]any{"type": "string"},
						"otpauth_url": map[string]any{"type": "string"},
						"expires_at":  map[string]any{"type": "string", "format": "date-time"},
					},
				},
				"PermissionContext": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"instance_role": map[string]any{"type": "string"},
						"permissions":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
				},
				"APIClientSiteRole": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"site_id": map[string]any{"type": "string", "format": "uuid"},
						"role":    map[string]any{"type": "string"},
					},
				},
				"APIClient": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":            map[string]any{"type": "string", "format": "uuid"},
						"user_id":       map[string]any{"type": "string", "format": "uuid"},
						"name":          map[string]any{"type": "string"},
						"description":   map[string]any{"type": "string"},
						"instance_role": map[string]any{"type": "string"},
						"expires_at":    map[string]any{"type": "string", "format": "date-time"},
						"last_used_at":  map[string]any{"type": "string", "format": "date-time"},
						"revoked_at":    map[string]any{"type": "string", "format": "date-time"},
						"created_at":    map[string]any{"type": "string", "format": "date-time"},
						"updated_at":    map[string]any{"type": "string", "format": "date-time"},
						"site_roles":    map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/APIClientSiteRole"}},
					},
				},
				"APIClientCreateResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"client": map[string]any{"$ref": "#/components/schemas/APIClient"},
						"token":  map[string]any{"type": "string"},
					},
				},
				"OpenAPIVersionList": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"latest": map[string]any{"type": "string"},
						"versions": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"version":     map[string]any{"type": "string"},
									"openapi_url": map[string]any{"type": "string"},
									"latest":      map[string]any{"type": "boolean"},
								},
							},
						},
					},
				},
				"DigestSubscription": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"daily":   map[string]any{"type": "boolean"},
						"weekly":  map[string]any{"type": "boolean"},
						"monthly": map[string]any{"type": "boolean"},
					},
				},
				"SiteReportSubscription": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"site_id": map[string]any{"type": "string", "format": "uuid"},
						"domain":  map[string]any{"type": "string"},
						"daily":   map[string]any{"type": "boolean"},
						"weekly":  map[string]any{"type": "boolean"},
						"monthly": map[string]any{"type": "boolean"},
					},
				},
				"ReportSubscriptions": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"digest": map[string]any{"$ref": "#/components/schemas/DigestSubscription"},
						"sites": map[string]any{
							"type":  "array",
							"items": map[string]any{"$ref": "#/components/schemas/SiteReportSubscription"},
						},
					},
				},
				"LoginResponse": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"status":          map[string]any{"type": "string"},
						"challenge_token": map[string]any{"type": "string"},
						"factors":         map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"passkey":         map[string]any{"type": "object", "additionalProperties": true},
					},
				},
			},
		},
		"paths": map[string]any{
			"/healthz": map[string]any{
				"get": op([]string{"System"}, "Health check", "Liveness endpoint.", nil, nil, nil, map[string]any{"200": desc("OK")}),
			},
			"/readyz": map[string]any{
				"get": op([]string{"System"}, "Readiness check", "Readiness endpoint (leader and DB readiness).", nil, nil, nil, map[string]any{"200": desc("Ready"), "503": errResp("Not ready")}),
			},
			"/api/status": map[string]any{
				"get": op([]string{"System"}, "Instance status", "Setup and version status.", nil, nil, nil, map[string]any{
					"200": jsonSchemaResp("Status payload", map[string]any{"type": "object", "properties": map[string]any{"needs_setup": map[string]any{"type": "boolean"}, "version": map[string]any{"type": "string"}}}),
				}),
			},
			"/api/docs/versions": map[string]any{
				"get": op([]string{"System"}, "List API doc versions", "Returns available OpenAPI document versions.", nil, nil, nil, map[string]any{"200": jsonRefResp("Version list", "#/components/schemas/OpenAPIVersionList")}),
			},
			"/api/docs/v1/openapi.json": map[string]any{
				"get": op([]string{"System"}, "OpenAPI v1 document", "Returns the full OpenAPI 3.1 specification for v1.", nil, nil, nil, map[string]any{"200": map[string]any{"description": "OpenAPI 3.1 JSON"}}),
			},

			"/ingest": map[string]any{
				"options": op([]string{"Ingest"}, "Preflight ingest", "CORS preflight for pageview ingest.", nil, nil, nil, map[string]any{"200": desc("Preflight response")}),
				"post": op([]string{"Ingest"}, "Ingest pageview", "Ingests a pageview hit from the browser tracker.", nil, nil,
					map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{
						"path": map[string]any{"type": "string"}, "referrer": map[string]any{"type": "string"}, "ua": map[string]any{"type": "string"},
						"vp_w": map[string]any{"type": "integer"}, "vp_h": map[string]any{"type": "integer"}, "sc_w": map[string]any{"type": "integer"}, "sc_h": map[string]any{"type": "integer"},
						"lang": map[string]any{"type": "string"}, "u_src": map[string]any{"type": "string"}, "u_med": map[string]any{"type": "string"}, "u_cmp": map[string]any{"type": "string"}, "u_trm": map[string]any{"type": "string"}, "u_cnt": map[string]any{"type": "string"},
						"unique": map[string]any{"type": "boolean"}, "session_id": map[string]any{"type": "string", "format": "uuid"}, "page_id": map[string]any{"type": "string", "format": "uuid"},
					}, "required": []string{"path", "session_id", "page_id"}}}}},
					map[string]any{"202": desc("Accepted"), "400": errResp("Invalid request")}),
			},
			"/ingest/event": map[string]any{
				"options": op([]string{"Ingest"}, "Preflight event ingest", "CORS preflight for custom event ingest.", nil, nil, nil, map[string]any{"200": desc("Preflight response")}),
				"post": op([]string{"Ingest"}, "Ingest custom event", "Ingests a custom event from the browser tracker.", nil, nil,
					map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{
						"n": map[string]any{"type": "string"}, "p": map[string]any{"type": "object", "additionalProperties": true}, "sid": map[string]any{"type": "string", "format": "uuid"},
					}, "required": []string{"n", "sid"}}}}},
					map[string]any{"202": desc("Accepted"), "400": errResp("Invalid request")}),
			},

			"/api/initial-user": map[string]any{
				"post": op([]string{"Auth"}, "Create initial admin", "Bootstraps first user account during setup.", nil, nil,
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"email":      map[string]any{"type": "string", "format": "email"},
							"password":   map[string]any{"type": "string", "minLength": 8},
							"given_name": map[string]any{"type": "string"},
							"last_name":  map[string]any{"type": "string"},
						},
						"required": []string{"email", "password"},
					}),
					map[string]any{"201": jsonSchemaResp("Token created", map[string]any{"type": "object", "properties": map[string]any{"token": map[string]any{"type": "string"}}}), "403": errResp("Setup already complete")}),
			},
			"/api/login": map[string]any{
				"post": op([]string{"Auth"}, "Login", "Authenticates user credentials and issues session cookie.", nil, nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"email": map[string]any{"type": "string", "format": "email"}, "password": map[string]any{"type": "string"}, "remember_me": map[string]any{"type": "boolean"}}, "required": []string{"email", "password"}}),
					map[string]any{"200": jsonRefResp("Login response", "#/components/schemas/LoginResponse"), "401": errResp("Invalid credentials")}),
			},
			"/api/logout": map[string]any{
				"post": op([]string{"Auth"}, "Logout", "Clears session and remember-me cookies.", secCookie(), nil, nil, map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/auth/forgot-password": map[string]any{
				"post": op([]string{"Auth"}, "Request password reset", "Sends password reset email if account exists.", nil, nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"email": map[string]any{"type": "string", "format": "email"}}, "required": []string{"email"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/auth/reset-password": map[string]any{
				"post": op([]string{"Auth"}, "Complete password reset", "Resets password using reset token.", nil, nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"token": map[string]any{"type": "string"}, "password": map[string]any{"type": "string", "minLength": 8}}, "required": []string{"token", "password"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status"), "400": errResp("Invalid or expired link")}),
			},
			"/api/auth/accept-invite": map[string]any{
				"post": op([]string{"Auth"}, "Accept invite", "Sets password for invited user using invite token.", nil, nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"token": map[string]any{"type": "string"}, "password": map[string]any{"type": "string", "minLength": 8}}, "required": []string{"token", "password"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/auth/passkey/login/start": map[string]any{
				"post": op([]string{"Auth"}, "Start passkey login", "Creates passkey login challenge.", nil, nil, nil,
					map[string]any{"200": jsonSchemaResp("Passkey challenge", map[string]any{"type": "object", "properties": map[string]any{"challenge_token": map[string]any{"type": "string"}, "publicKey": map[string]any{"type": "object", "additionalProperties": true}}})}),
			},
			"/api/auth/passkey/login/finish": map[string]any{
				"post": op([]string{"Auth"}, "Finish passkey login", "Verifies passkey assertion and issues session.", nil, nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"challenge_token": map[string]any{"type": "string"}, "credential_id": map[string]any{"type": "string"}, "client_data_json": map[string]any{"type": "string"}, "authenticator_data": map[string]any{"type": "string"}, "signature": map[string]any{"type": "string"}, "remember_me": map[string]any{"type": "boolean"}}, "required": []string{"challenge_token", "credential_id", "client_data_json", "authenticator_data", "signature"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/auth/mfa/totp/verify": map[string]any{
				"post": op([]string{"Auth"}, "Verify MFA TOTP", "Verifies TOTP code for pending MFA challenge.", nil, nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"challenge_token": map[string]any{"type": "string", "format": "uuid"}, "code": map[string]any{"type": "string"}}, "required": []string{"challenge_token", "code"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/user/password": map[string]any{
				"post": op([]string{"Auth"}, "Change password", "Changes password for authenticated user.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"current_password": map[string]any{"type": "string"}, "new_password": map[string]any{"type": "string", "minLength": 8}}, "required": []string{"current_password", "new_password"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status"), "403": errResp("Current password is incorrect")}),
			},

			"/api/user/profile": map[string]any{
				"get": op([]string{"User"}, "Get profile", "Returns authenticated user profile.", secCookie(), nil, nil, map[string]any{"200": jsonRefResp("User profile", "#/components/schemas/UserProfile")}),
				"put": op([]string{"User"}, "Update profile", "Updates authenticated user profile details.", secCookie(), nil,
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"email":      map[string]any{"type": "string", "format": "email"},
							"given_name": map[string]any{"type": "string"},
							"last_name":  map[string]any{"type": "string"},
						},
						"required": []string{"email"},
					}),
					map[string]any{
						"200": jsonRefResp("Updated profile", "#/components/schemas/UserProfile"),
						"400": errResp("Invalid request"),
						"404": errResp("User not found"),
						"409": errResp("Email already exists"),
					}),
			},
			"/api/user/avatar": map[string]any{
				"get": op([]string{"User"}, "Get avatar", "Proxies authenticated user's avatar image.", secCookie(), []any{paramRef("#/components/parameters/avatarSize")}, nil, map[string]any{"200": desc("Avatar image")}),
			},
			"/api/user/current-ip": map[string]any{
				"get": op([]string{"User"}, "Get current IP", "Returns the resolved client IP and single-host CIDR for quick exclusion setup.", secCookie(), nil, nil, map[string]any{
					"200": jsonSchemaResp("Current IP", map[string]any{
						"type": "object",
						"properties": map[string]any{
							"ip":   map[string]any{"type": "string"},
							"cidr": map[string]any{"type": "string"},
						},
						"required": []string{"ip", "cidr"},
					}),
				}),
			},
			"/api/user/preferences": map[string]any{
				"get": op([]string{"User"}, "Get user preferences", "Returns authenticated user preferences.", secCookie(), nil, nil, map[string]any{"200": jsonRefResp("Preferences", "#/components/schemas/UserPreferences")}),
				"put": op([]string{"User"}, "Update user preferences", "Updates authenticated user preferences.", secCookie(), nil,
					jsonBody(map[string]any{"$ref": "#/components/schemas/UserPreferences"}),
					map[string]any{"200": jsonRefResp("Preferences", "#/components/schemas/UserPreferences")}),
			},
			"/api/user/teams": map[string]any{
				"post": op([]string{"Teams"}, "Create team", "Creates a new team and returns the created team payload.", secCookie(), nil,
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":     map[string]any{"type": "string"},
							"logo_url": map[string]any{"type": "string"},
						},
						"required": []string{"name"},
					}),
					map[string]any{
						"201": jsonRefResp("Created team", "#/components/schemas/TeamCreateResponse"),
						"400": errResp("Invalid request"),
						"403": errResp("Team limit reached"),
					}),
				"get": op([]string{"Teams"}, "List teams", "Returns all teams for the authenticated user and the current active team.", secCookie(), nil, nil,
					map[string]any{"200": jsonRefResp("Team list", "#/components/schemas/TeamListResponse")}),
			},
			"/api/user/teams/active": map[string]any{
				"put": op([]string{"Teams"}, "Set active team", "Sets the current active team context for the authenticated user.", secCookie(), nil,
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"team_id": map[string]any{"type": "string", "format": "uuid"},
						},
						"required": []string{"team_id"},
					}),
					map[string]any{
						"200": jsonRefResp("Active team response", "#/components/schemas/TeamActiveResponse"),
						"403": errResp("Access denied"),
					}),
			},
			"/api/user/teams/{id}": map[string]any{
				"patch": op([]string{"Teams"}, "Update team", "Updates team settings. This is the canonical update route.", secCookie(), []any{paramRef("#/components/parameters/teamID")},
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":     map[string]any{"type": "string"},
							"logo_url": map[string]any{"type": "string"},
						},
						"required": []string{"name"},
					}),
					map[string]any{
						"200": jsonRefResp("Status", "#/components/schemas/Status"),
						"400": errResp("Invalid request"),
						"403": errResp("Access denied"),
					}),
				"put": op([]string{"Teams"}, "Update team (deprecated)", "Deprecated compatibility alias for team updates. Use PATCH /api/user/teams/{id}.", secCookie(), []any{paramRef("#/components/parameters/teamID")},
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":     map[string]any{"type": "string"},
							"logo_url": map[string]any{"type": "string"},
						},
						"required": []string{"name"},
					}),
					map[string]any{
						"200": jsonRefResp("Status", "#/components/schemas/Status"),
						"400": errResp("Invalid request"),
						"403": errResp("Access denied"),
					}),
			},
			"/api/user/teams/{id}/transfer-ownership": map[string]any{
				"post": op([]string{"Teams"}, "Transfer team ownership", "Transfers ownership from the current owner to another existing team member.", secCookie(), []any{paramRef("#/components/parameters/teamID")},
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"target_user_id": map[string]any{"type": "string", "format": "uuid"},
						},
						"required": []string{"target_user_id"},
					}),
					map[string]any{
						"200": jsonRefResp("Status", "#/components/schemas/Status"),
						"400": errResp("Invalid target user"),
						"403": errResp("Only owners can transfer ownership"),
						"409": errResp("Transfer conflict"),
					}),
			},
			"/api/user/teams/{id}/archive": map[string]any{
				"post": op([]string{"Teams"}, "Archive team", "Archives a non-default team after all sites have been transferred or removed.", secCookie(), []any{paramRef("#/components/parameters/teamID")}, nil,
					map[string]any{
						"200": jsonRefResp("Archive team response", "#/components/schemas/TeamArchiveResponse"),
						"400": errResp("Cannot archive the default team or a team that still owns sites"),
						"403": errResp("Only owners can archive teams"),
					}),
			},
			"/api/user/teams/{id}/members": map[string]any{
				"get": op([]string{"Teams"}, "List team members", "Lists members for the specified team.", secCookie(), []any{paramRef("#/components/parameters/teamID")}, nil,
					map[string]any{"200": jsonSchemaResp("Team members", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/TeamMember"}})}),
				"post": op([]string{"Teams"}, "Invite team member", "Creates a pending invite for a user or updates the role of an existing member.", secCookie(), []any{paramRef("#/components/parameters/teamID")},
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"email": map[string]any{"type": "string", "format": "email"},
							"role":  map[string]any{"type": "string", "enum": []string{"owner", "admin", "member"}},
						},
						"required": []string{"email", "role"},
					}),
					map[string]any{
						"200": jsonSchemaResp("Invite or update response", map[string]any{
							"type": "object",
							"properties": map[string]any{
								"status":    map[string]any{"type": "string"},
								"is_invite": map[string]any{"type": "boolean"},
								"invite":    map[string]any{"$ref": "#/components/schemas/TeamInvite"},
							},
						}),
						"403": errResp("Access denied"),
						"409": errResp("Invite already pending"),
					}),
			},
			"/api/user/teams/{id}/invites": map[string]any{
				"get": op([]string{"Teams"}, "List team invites", "Lists pending invites for the specified team.", secCookie(), []any{paramRef("#/components/parameters/teamID")}, nil,
					map[string]any{
						"200": jsonSchemaResp("Team invites", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/TeamInvite"}}),
						"403": errResp("Access denied"),
					}),
			},
			"/api/user/teams/{id}/audit": map[string]any{
				"get": op([]string{"Teams"}, "List team audit log", "Lists recent audit events for team management actions.", secCookie(), []any{
					paramRef("#/components/parameters/teamID"),
					map[string]any{
						"name":        "action",
						"in":          "query",
						"description": "Optional exact audit action to filter by.",
						"schema":      map[string]any{"type": "string"},
					},
					map[string]any{
						"name":        "limit",
						"in":          "query",
						"description": "Maximum number of audit rows to return (default 25, max 200).",
						"schema":      map[string]any{"type": "integer", "minimum": 1, "maximum": 200},
					},
					map[string]any{
						"name":        "offset",
						"in":          "query",
						"description": "Zero-based audit row offset for pagination.",
						"schema":      map[string]any{"type": "integer", "minimum": 0},
					},
				}, nil,
					map[string]any{
						"200": jsonRefResp("Team audit entries", "#/components/schemas/TeamAuditListResponse"),
						"400": errResp("Invalid query parameters"),
						"403": errResp("Access denied"),
					}),
			},
			"/api/user/teams/{id}/invites/{inviteId}/resend": map[string]any{
				"post": op([]string{"Teams"}, "Resend team invite", "Refreshes and resends a pending invite.", secCookie(), []any{
					paramRef("#/components/parameters/teamID"),
					map[string]any{"name": "inviteId", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				}, nil,
					map[string]any{
						"200": jsonSchemaResp("Invite resend response", map[string]any{
							"type": "object",
							"properties": map[string]any{
								"status": map[string]any{"type": "string"},
								"invite": map[string]any{"$ref": "#/components/schemas/TeamInvite"},
							},
						}),
						"403": errResp("Access denied"),
						"404": errResp("Invite not found"),
					}),
			},
			"/api/user/teams/{id}/invites/{inviteId}": map[string]any{
				"delete": op([]string{"Teams"}, "Revoke team invite", "Revokes a pending invite.", secCookie(), []any{
					paramRef("#/components/parameters/teamID"),
					map[string]any{"name": "inviteId", "in": "path", "required": true, "schema": map[string]any{"type": "string", "format": "uuid"}},
				}, nil,
					map[string]any{
						"200": jsonRefResp("Status", "#/components/schemas/Status"),
						"403": errResp("Access denied"),
						"404": errResp("Invite not found"),
					}),
			},
			"/api/user/teams/{id}/members/{userId}": map[string]any{
				"delete": op([]string{"Teams"}, "Remove team member", "Removes a member from the specified team.", secCookie(), []any{paramRef("#/components/parameters/teamID"), paramRef("#/components/parameters/userID")}, nil,
					map[string]any{
						"200": jsonRefResp("Status", "#/components/schemas/Status"),
						"400": errResp("Cannot remove last owner"),
						"403": errResp("Access denied"),
						"404": errResp("Team member not found"),
					}),
			},
			"/api/user/teams/{id}/leave": map[string]any{
				"delete": op([]string{"Teams"}, "Leave team", "Removes the authenticated user from the specified team and returns the new active team.", secCookie(), []any{paramRef("#/components/parameters/teamID")}, nil,
					map[string]any{
						"200": jsonRefResp("Leave team response", "#/components/schemas/TeamLeaveResponse"),
						"400": errResp("Cannot leave your only team or last owner"),
						"403": errResp("Access denied"),
					}),
			},
			"/api/user/security": map[string]any{
				"get": op([]string{"User"}, "Get user security status", "Returns TOTP/passkey status.", secCookie(), nil, nil, map[string]any{"200": jsonRefResp("Security status", "#/components/schemas/UserSecurityStatus")}),
			},
			"/api/user/security/totp/setup/start": map[string]any{
				"post": op([]string{"User"}, "Start TOTP setup", "Starts TOTP enrollment and returns secret + OTPAuth URI.", secCookie(), nil, nil, map[string]any{"200": jsonRefResp("TOTP setup", "#/components/schemas/UserTOTPSetup")}),
			},
			"/api/user/security/totp/setup/verify": map[string]any{
				"post": op([]string{"User"}, "Verify TOTP setup", "Verifies TOTP code and enables TOTP.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"code": map[string]any{"type": "string"}}, "required": []string{"code"}}),
					map[string]any{"200": jsonRefResp("Security status", "#/components/schemas/UserSecurityStatus")}),
			},
			"/api/user/security/totp/disable": map[string]any{
				"post": op([]string{"User"}, "Disable TOTP", "Disables TOTP after current code verification.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"code": map[string]any{"type": "string"}}, "required": []string{"code"}}),
					map[string]any{"200": jsonRefResp("Security status", "#/components/schemas/UserSecurityStatus")}),
			},
			"/api/user/security/passkeys/register/start": map[string]any{
				"post": op([]string{"User"}, "Start passkey registration", "Creates passkey registration challenge and options.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}),
					map[string]any{"200": jsonSchemaResp("Passkey creation options", map[string]any{"type": "object", "properties": map[string]any{"publicKey": map[string]any{"type": "object", "additionalProperties": true}}})}),
			},
			"/api/user/security/passkeys/register/finish": map[string]any{
				"post": op([]string{"User"}, "Finish passkey registration", "Verifies passkey attestation and stores credential.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "credential_id": map[string]any{"type": "string"}, "client_data_json": map[string]any{"type": "string"}, "public_key": map[string]any{"type": "string"}, "transports": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}}, "required": []string{"credential_id", "client_data_json", "public_key"}}),
					map[string]any{"200": jsonRefResp("Security status", "#/components/schemas/UserSecurityStatus")}),
			},
			"/api/user/security/passkeys/{id}": map[string]any{
				"delete": op([]string{"User"}, "Delete passkey", "Deletes a registered passkey credential.", secCookie(), []any{paramRef("#/components/parameters/passkeyID")}, nil,
					map[string]any{"200": jsonRefResp("Security status", "#/components/schemas/UserSecurityStatus")}),
			},
			"/api/user/api-clients": map[string]any{
				"get": op([]string{"User"}, "List API clients", "Lists API clients for authenticated user.", secCookie(), nil, nil, map[string]any{"200": jsonSchemaResp("API clients", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/APIClient"}})}),
				"post": op([]string{"User"}, "Create API client", "Creates delegated API client and returns one-time token.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{
						"name":          map[string]any{"type": "string"},
						"description":   map[string]any{"type": "string"},
						"instance_role": map[string]any{"type": "string"},
						"expires_at":    map[string]any{"type": "string", "format": "date-time"},
						"site_roles":    map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"site_id": map[string]any{"type": "string", "format": "uuid"}, "role": map[string]any{"type": "string"}}}},
					}, "required": []string{"name"}}),
					map[string]any{"201": jsonRefResp("Created API client", "#/components/schemas/APIClientCreateResponse")}),
			},
			"/api/user/api-clients/{id}": map[string]any{
				"put": op([]string{"User"}, "Update API client", "Updates delegated API client.", secCookie(), []any{paramRef("#/components/parameters/apiClientID")},
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{
						"name":          map[string]any{"type": "string"},
						"description":   map[string]any{"type": "string"},
						"instance_role": map[string]any{"type": "string"},
						"expires_at":    map[string]any{"type": "string", "format": "date-time"},
						"revoked":       map[string]any{"type": "boolean"},
						"site_roles":    map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/APIClientSiteRole"}},
					}, "required": []string{"name", "instance_role"}}),
					map[string]any{"200": jsonRefResp("Updated API client", "#/components/schemas/APIClient")}),
				"delete": op([]string{"User"}, "Delete API client", "Deletes delegated API client.", secCookie(), []any{paramRef("#/components/parameters/apiClientID")}, nil, map[string]any{"204": desc("Deleted")}),
			},

			"/api/user/permissions": map[string]any{
				"get": op([]string{"Permissions"}, "Get permission context", "Returns authenticated user's instance permissions.", secCookie(), nil, nil, map[string]any{"200": jsonRefResp("Permission context", "#/components/schemas/PermissionContext")}),
			},

			"/api/admin/users": map[string]any{
				"get": op([]string{"Admin"}, "List users", "Lists users for admin management.", secCookie(), nil, nil, map[string]any{"200": jsonSchemaResp("User list", map[string]any{"type": "array", "items": map[string]any{"type": "object", "additionalProperties": true}})}),
			},
			"/api/admin/users/{id}/role": map[string]any{
				"post": op([]string{"Admin"}, "Update user role", "Updates instance role for target user.", secCookie(), []any{paramRef("#/components/parameters/adminUserID")},
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"role": map[string]any{"type": "string"}}, "required": []string{"role"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/admin/users/{id}": map[string]any{
				"delete": op([]string{"Admin"}, "Delete user", "Deletes user account (cannot delete self). Deletion is blocked if the target user is the sole owner of any team.", secCookie(), []any{paramRef("#/components/parameters/adminUserID")}, nil, map[string]any{
					"200": jsonRefResp("Status", "#/components/schemas/Status"),
					"409": jsonRefResp("Delete user blocked response", "#/components/schemas/AdminDeleteUserBlockedResponse"),
				}),
			},
			"/api/admin/sites": map[string]any{
				"get": op([]string{"Admin"}, "List all sites", "Lists all sites for admin management.", secCookie(), nil, nil, map[string]any{"200": jsonSchemaResp("Site list", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Site"}})}),
			},
			"/api/admin/sites/{id}": map[string]any{
				"delete": op([]string{"Admin"}, "Delete site (admin)", "Deletes site by admin endpoint.", secCookie(), []any{paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/admin/teams/{id}": map[string]any{
				"delete": op([]string{"Admin"}, "Delete archived team", "Permanently deletes an archived non-default team and removes its per-tenant analytics database directory.", secCookie(), []any{paramRef("#/components/parameters/teamID")}, nil, map[string]any{
					"200": jsonRefResp("Delete archived team response", "#/components/schemas/AdminDeleteTeamResponse"),
					"400": errResp("Archive the team first, and ensure it has no sites"),
					"404": errResp("Team not found"),
				}),
			},
			"/api/admin/exclusions": map[string]any{
				"get": op([]string{"Admin"}, "List global exclusions", "Lists instance-level IP/CIDR exclusions used by ingest filtering.", secCookie(), nil, nil,
					map[string]any{"200": jsonSchemaResp("Global exclusions", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/IPExclusion"}})}),
				"post": op([]string{"Admin"}, "Create global exclusion", "Creates instance-level IP/CIDR exclusion for all sites.", secCookie(), nil,
					jsonBody(map[string]any{"$ref": "#/components/schemas/IPExclusionCreateRequest"}),
					map[string]any{"201": jsonRefResp("Created exclusion", "#/components/schemas/IPExclusion"), "400": errResp("Invalid IP/CIDR")}),
			},
			"/api/admin/exclusions/{ruleID}": map[string]any{
				"delete": op([]string{"Admin"}, "Delete global exclusion", "Deletes instance-level IP/CIDR exclusion rule.", secCookie(), []any{paramRef("#/components/parameters/ruleID")}, nil,
					map[string]any{"204": desc("Deleted"), "404": errResp("Not found")}),
			},
			"/api/sites/{id}/members": map[string]any{
				"get": op([]string{"Admin"}, "List site members", "Lists site members and roles.", secCookie(), []any{paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonSchemaResp("Members", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/SiteMember"}})}),
				"post": op([]string{"Admin"}, "Add site member", "Adds member to site and optionally sends invite.", secCookie(), []any{paramRef("#/components/parameters/siteID")},
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"email": map[string]any{"type": "string", "format": "email"}, "role": map[string]any{"type": "string"}}, "required": []string{"email", "role"}}),
					map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/sites/{id}/members/{userId}": map[string]any{
				"delete": op([]string{"Admin"}, "Remove site member", "Removes a user from site membership.", secCookie(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/userID")}, nil, map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},

			"/api/sites": map[string]any{
				"get": op([]string{"Sites"}, "List accessible sites", "Lists sites visible to caller (session or API key scope).", secAnyAuth(), nil, nil, map[string]any{"200": jsonSchemaResp("Sites", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Site"}})}),
				"post": op([]string{"Sites"}, "Create site", "Creates new tracked site.", secCookie(), nil,
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"domain": map[string]any{"type": "string"}}, "required": []string{"domain"}}),
					map[string]any{"200": jsonRefResp("Site", "#/components/schemas/Site"), "409": errResp("Domain exists")}),
			},
			"/api/sites/{id}": map[string]any{
				"delete": op([]string{"Sites"}, "Delete site", "Deletes a site and associated analytics data.", secCookie(), []any{paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonRefResp("Status", "#/components/schemas/Status")}),
			},
			"/api/sites/{id}/stats": map[string]any{
				"get": op([]string{"Sites"}, "Get site stats", "Aggregated site metrics and charts.", secAnyAuth(), []any{
					paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"),
					paramRef("#/components/parameters/filter"), paramRef("#/components/parameters/filterType"), paramRef("#/components/parameters/filterValue"),
					paramRef("#/components/parameters/goalIDQuery"), paramRef("#/components/parameters/funnelIDQuery"),
				}, nil, map[string]any{"200": jsonRefResp("Site stats", "#/components/schemas/SiteStats")}),
			},
			"/api/sites/{id}/hits": map[string]any{
				"get": op([]string{"Sites"}, "Get site hits", "Returns paginated raw hits.", secAnyAuth(), []any{
					paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"),
					paramRef("#/components/parameters/limit"), paramRef("#/components/parameters/offset"), paramRef("#/components/parameters/query"),
					paramRef("#/components/parameters/sort"), paramRef("#/components/parameters/order"),
					paramRef("#/components/parameters/filter"), paramRef("#/components/parameters/filterType"), paramRef("#/components/parameters/filterValue"),
				}, nil, map[string]any{"200": jsonRefResp("Paginated hits", "#/components/schemas/PaginatedHits")}),
			},
			"/api/sites/{id}/hits/export": map[string]any{
				"get": op([]string{"Sites"}, "Export site hits", "Exports filtered site hits in csv/xlsx/parquet/json/ndjson.", secAnyAuth(), []any{
					paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"),
					paramRef("#/components/parameters/query"), paramRef("#/components/parameters/filter"), paramRef("#/components/parameters/filterType"), paramRef("#/components/parameters/filterValue"), paramRef("#/components/parameters/format"),
				}, nil, map[string]any{"200": desc("Export file stream")}),
			},
			"/api/favicon/{domain}": map[string]any{
				"get": op([]string{"Sites"}, "Get favicon", "Proxies favicon by domain.", nil, []any{paramRef("#/components/parameters/domain")}, nil, map[string]any{"200": desc("Favicon image")}),
			},
			"/api/sites/{id}/retention": map[string]any{
				"put": op([]string{"Sites"}, "Update retention policy", "Updates per-site retention days.", secCookie(), []any{paramRef("#/components/parameters/siteID")},
					jsonBody(map[string]any{"type": "object", "properties": map[string]any{"days": map[string]any{"type": "integer", "minimum": 0}}, "required": []string{"days"}}),
					map[string]any{"200": desc("Updated")}),
			},
			"/api/sites/{id}/transfer-team": map[string]any{
				"post": op([]string{"Sites"}, "Transfer site to another team", "Moves a site into another team the caller can administer and migrates analytics data to the destination tenant store.", secCookie(), []any{paramRef("#/components/parameters/siteID")},
					jsonBody(map[string]any{
						"type": "object",
						"properties": map[string]any{
							"team_id": map[string]any{"type": "string", "format": "uuid"},
						},
						"required": []string{"team_id"},
					}),
					map[string]any{
						"200": jsonRefResp("Site transferred", "#/components/schemas/SiteTransferResponse"),
						"403": errResp("Access denied"),
					}),
			},
			"/api/sites/{id}/exclusions": map[string]any{
				"get": op([]string{"Sites"}, "List site exclusions", "Lists per-site IP/CIDR exclusions used by ingest filtering.", secCookie(), []any{paramRef("#/components/parameters/siteID")}, nil,
					map[string]any{"200": jsonSchemaResp("Site exclusions", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/IPExclusion"}})}),
				"post": op([]string{"Sites"}, "Create site exclusion", "Creates per-site IP/CIDR exclusion rule.", secCookie(), []any{paramRef("#/components/parameters/siteID")},
					jsonBody(map[string]any{"$ref": "#/components/schemas/IPExclusionCreateRequest"}),
					map[string]any{"201": jsonRefResp("Created exclusion", "#/components/schemas/IPExclusion"), "400": errResp("Invalid IP/CIDR")}),
			},
			"/api/sites/{id}/exclusions/{ruleID}": map[string]any{
				"delete": op([]string{"Sites"}, "Delete site exclusion", "Deletes a per-site IP/CIDR exclusion rule.", secCookie(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/ruleID")}, nil,
					map[string]any{"204": desc("Deleted"), "404": errResp("Not found")}),
			},

			"/api/sites/{id}/goals": map[string]any{
				"get": op([]string{"Goals"}, "List goals", "Lists goals for a site.", secAnyAuth(), []any{paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonSchemaResp("Goals", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Goal"}})}),
				"post": op([]string{"Goals"}, "Create goal", "Creates a conversion goal.", secCookie(), []any{paramRef("#/components/parameters/siteID")},
					jsonBody(map[string]any{"$ref": "#/components/schemas/Goal"}),
					map[string]any{"201": desc("Created")}),
			},
			"/api/sites/{id}/goals/{goalID}": map[string]any{
				"delete": op([]string{"Goals"}, "Delete goal", "Deletes goal from site.", secCookie(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/goalID")}, nil, map[string]any{"200": desc("Deleted")}),
			},
			"/api/sites/{id}/goals/timeseries": map[string]any{
				"get": op([]string{"Goals"}, "Goal timeseries", "Returns goal conversion timeseries.", secAnyAuth(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/goalIDQuery")}, nil,
					map[string]any{"200": jsonSchemaResp("Goal timeseries", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/GoalSeriesPoint"}})}),
			},

			"/api/sites/{id}/funnels": map[string]any{
				"get": op([]string{"Funnels"}, "List funnels", "Lists funnels for a site.", secAnyAuth(), []any{paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonSchemaResp("Funnels", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Funnel"}})}),
				"post": op([]string{"Funnels"}, "Create funnel", "Creates a multi-step funnel.", secCookie(), []any{paramRef("#/components/parameters/siteID")},
					jsonBody(map[string]any{"$ref": "#/components/schemas/Funnel"}),
					map[string]any{"201": desc("Created")}),
			},
			"/api/sites/{id}/funnels/{funnelID}": map[string]any{
				"delete": op([]string{"Funnels"}, "Delete funnel", "Deletes funnel from site.", secCookie(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/funnelID")}, nil, map[string]any{"200": desc("Deleted")}),
			},
			"/api/sites/{id}/funnels/timeseries": map[string]any{
				"get": op([]string{"Funnels"}, "Funnel timeseries", "Returns funnel entry/completion timeseries.", secAnyAuth(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/funnelIDQuery")}, nil,
					map[string]any{"200": jsonSchemaResp("Funnel timeseries", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/FunnelSeriesPoint"}})}),
			},
			"/api/sites/{id}/funnels/{funnelID}/stats": map[string]any{
				"get": op([]string{"Funnels"}, "Funnel stats", "Returns full funnel step stats.", secAnyAuth(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/funnelID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to")}, nil,
					map[string]any{"200": jsonRefResp("Funnel stats", "#/components/schemas/FunnelStats")}),
			},

			"/api/user/takeout": map[string]any{
				"get": op([]string{"Takeout"}, "User takeout", "Exports all user data across sites as xlsx/csv/parquet/json/ndjson.", secCookie(), []any{paramRef("#/components/parameters/format")}, nil, map[string]any{"200": desc("Export file stream")}),
			},
			"/api/sites/{id}/takeout": map[string]any{
				"get": op([]string{"Takeout"}, "Site takeout", "Exports site data as xlsx/csv/parquet/json/ndjson.", secCookie(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/format")}, nil, map[string]any{"200": desc("Export file stream")}),
			},

			"/api/user/report-subscriptions": map[string]any{
				"get": op([]string{"Reports"}, "Get report subscriptions", "Returns all report subscription preferences for the authenticated user, including per-site and digest settings.", secCookie(), nil, nil,
					map[string]any{"200": jsonRefResp("Report subscriptions", "#/components/schemas/ReportSubscriptions")}),
			},
			"/api/user/report-subscriptions/digest": map[string]any{
				"put": op([]string{"Reports"}, "Update digest subscription", "Updates the consolidated digest subscription frequencies for the authenticated user.", secCookie(), nil,
					jsonBody(map[string]any{"$ref": "#/components/schemas/DigestSubscription"}),
					map[string]any{"204": desc("Updated"), "400": errResp("Invalid request")}),
			},
			"/api/user/report-subscriptions/sites/{site_id}": map[string]any{
				"put": op([]string{"Reports"}, "Update site report subscription", "Updates per-site report subscription frequencies for the authenticated user.", secCookie(), []any{paramRef("#/components/parameters/reportSiteID")},
					jsonBody(map[string]any{"$ref": "#/components/schemas/DigestSubscription"}),
					map[string]any{"204": desc("Updated"), "400": errResp("Invalid site ID or request")}),
			},

			"/api/sites/{id}/share": map[string]any{
				"get": op([]string{"Share"}, "List share links", "Lists share links for site.", secCookie(), []any{paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonSchemaResp("Share links", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/ShareLink"}})}),
				"post": op([]string{"Share"}, "Create share link", "Creates new read-only share token URL.", secCookie(), []any{paramRef("#/components/parameters/siteID")}, nil,
					map[string]any{"200": jsonSchemaResp("Share link created", map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "string", "format": "uuid"}, "url": map[string]any{"type": "string"}, "token": map[string]any{"type": "string"}, "token_hint": map[string]any{"type": "string"}, "created_at": map[string]any{"type": "string", "format": "date-time"}}})}),
			},
			"/api/sites/{id}/share/{shareID}": map[string]any{
				"delete": op([]string{"Share"}, "Delete share link", "Revokes a share link.", secCookie(), []any{paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/shareID")}, nil, map[string]any{"204": desc("Deleted")}),
			},
			"/api/share/{token}/site": map[string]any{
				"get": op([]string{"Share"}, "Get shared site", "Gets site metadata from share token.", nil, []any{paramRef("#/components/parameters/token")}, nil, map[string]any{"200": jsonRefResp("Site", "#/components/schemas/Site")}),
			},
			"/api/share/{token}/sites/{id}/stats": map[string]any{
				"get": op([]string{"Share"}, "Shared site stats", "Returns stats through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/filter"), paramRef("#/components/parameters/filterType"), paramRef("#/components/parameters/filterValue"), paramRef("#/components/parameters/goalIDQuery"), paramRef("#/components/parameters/funnelIDQuery")}, nil, map[string]any{"200": jsonRefResp("Site stats", "#/components/schemas/SiteStats")}),
			},
			"/api/share/{token}/sites/{id}/hits": map[string]any{
				"get": op([]string{"Share"}, "Shared hits", "Returns paginated raw hits through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/limit"), paramRef("#/components/parameters/offset"), paramRef("#/components/parameters/query"), paramRef("#/components/parameters/sort"), paramRef("#/components/parameters/order"), paramRef("#/components/parameters/filter"), paramRef("#/components/parameters/filterType"), paramRef("#/components/parameters/filterValue")}, nil, map[string]any{"200": jsonRefResp("Paginated hits", "#/components/schemas/PaginatedHits")}),
			},
			"/api/share/{token}/sites/{id}/hits/export": map[string]any{
				"get": op([]string{"Share"}, "Export shared hits", "Exports hits through share token in csv/xlsx/parquet/json/ndjson.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/query"), paramRef("#/components/parameters/filter"), paramRef("#/components/parameters/filterType"), paramRef("#/components/parameters/filterValue"), paramRef("#/components/parameters/format")}, nil, map[string]any{"200": desc("Export file stream")}),
			},
			"/api/share/{token}/sites/{id}/goals": map[string]any{
				"get": op([]string{"Share"}, "Shared goals", "Lists goals through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonSchemaResp("Goals", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Goal"}})}),
			},
			"/api/share/{token}/sites/{id}/goals/timeseries": map[string]any{
				"get": op([]string{"Share"}, "Shared goal timeseries", "Returns goal timeseries through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/goalIDQuery")}, nil, map[string]any{"200": jsonSchemaResp("Goal timeseries", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/GoalSeriesPoint"}})}),
			},
			"/api/share/{token}/sites/{id}/funnels": map[string]any{
				"get": op([]string{"Share"}, "Shared funnels", "Lists funnels through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID")}, nil, map[string]any{"200": jsonSchemaResp("Funnels", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/Funnel"}})}),
			},
			"/api/share/{token}/sites/{id}/funnels/timeseries": map[string]any{
				"get": op([]string{"Share"}, "Shared funnel timeseries", "Returns funnel timeseries through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to"), paramRef("#/components/parameters/funnelIDQuery")}, nil, map[string]any{"200": jsonSchemaResp("Funnel timeseries", map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/FunnelSeriesPoint"}})}),
			},
			"/api/share/{token}/sites/{id}/funnels/{funnelID}/stats": map[string]any{
				"get": op([]string{"Share"}, "Shared funnel stats", "Returns funnel stats through share token.", nil, []any{paramRef("#/components/parameters/token"), paramRef("#/components/parameters/siteID"), paramRef("#/components/parameters/funnelID"), paramRef("#/components/parameters/from"), paramRef("#/components/parameters/to")}, nil, map[string]any{"200": jsonRefResp("Funnel stats", "#/components/schemas/FunnelStats")}),
			},
		},
	}
}

func op(tags []string, summary string, description string, security []any, parameters []any, requestBody any, responses map[string]any) map[string]any {
	out := map[string]any{
		"tags":        tags,
		"summary":     summary,
		"description": description,
		"responses":   responses,
	}
	if len(security) > 0 {
		out["security"] = security
	}
	if len(parameters) > 0 {
		out["parameters"] = parameters
	}
	if requestBody != nil {
		out["requestBody"] = requestBody
	}
	return out
}

func paramRef(ref string) map[string]any {
	return map[string]any{"$ref": ref}
}

func secCookie() []any {
	return []any{map[string]any{"cookieAuth": []any{}}}
}

func secAnyAuth() []any {
	return []any{
		map[string]any{"cookieAuth": []any{}},
		map[string]any{"bearerAuth": []any{}},
		map[string]any{"apiKeyAuth": []any{}},
	}
}

func jsonBody(schema map[string]any) map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
}

func desc(description string) map[string]any {
	return map[string]any{"description": description}
}

func errResp(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": "#/components/schemas/Error"},
			},
		},
	}
}

func jsonRefResp(description string, ref string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{"$ref": ref},
			},
		},
	}
}

func jsonSchemaResp(description string, schema map[string]any) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": schema,
			},
		},
	}
}
