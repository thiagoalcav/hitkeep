package system

import (
	"encoding/json"
	"net/http"
	"strings"
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

		spec := map[string]any{
			"openapi": "3.1.0",
			"info": map[string]any{
				"title":       "HitKeep REST API",
				"version":     "v1",
				"description": "Versioned REST API reference for session and API-key based access.",
			},
			"servers": []map[string]string{
				{"url": publicURL},
			},
			"tags": []map[string]string{
				{"name": "Sites", "description": "Site lifecycle and analytics data endpoints."},
				{"name": "Goals", "description": "Goal and funnel analytics endpoints."},
				{"name": "API Clients", "description": "Session-authenticated API client CRUD endpoints."},
				{"name": "Permissions", "description": "Current user permission context."},
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
						"description":  "API client token in Authorization: Bearer <token>.",
					},
					"apiKeyAuth": map[string]any{
						"type":        "apiKey",
						"in":          "header",
						"name":        "X-API-Key",
						"description": "API client token in X-API-Key header.",
					},
				},
				"parameters": map[string]any{
					"siteID": map[string]any{
						"name":     "id",
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type":   "string",
							"format": "uuid",
						},
					},
					"funnelID": map[string]any{
						"name":     "funnelID",
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type":   "string",
							"format": "uuid",
						},
					},
					"apiClientID": map[string]any{
						"name":     "id",
						"in":       "path",
						"required": true,
						"schema": map[string]any{
							"type":   "string",
							"format": "uuid",
						},
					},
				},
				"schemas": map[string]any{
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
							"site_roles": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"site_id": map[string]any{"type": "string", "format": "uuid"},
										"role":    map[string]any{"type": "string"},
									},
								},
							},
						},
					},
					"Error": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"message": map[string]any{"type": "string"},
						},
					},
				},
			},
			"paths": map[string]any{
				"/api/sites": map[string]any{
					"get": map[string]any{
						"tags":        []string{"Sites"},
						"summary":     "List sites visible to the caller",
						"description": "Returns sites based on session membership or API-client delegated scopes.",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"description": "Site list",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type":  "array",
											"items": map[string]any{"$ref": "#/components/schemas/Site"},
										},
									},
								},
							},
						},
					},
					"post": map[string]any{
						"tags":        []string{"Sites"},
						"summary":     "Create a site",
						"description": "Session-authenticated endpoint for creating a tracked site.",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
						},
						"requestBody": map[string]any{
							"required": true,
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"domain": map[string]any{"type": "string"},
										},
										"required": []string{"domain"},
									},
								},
							},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"description": "Created site",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{"$ref": "#/components/schemas/Site"},
									},
								},
							},
						},
					},
				},
				"/api/sites/{id}/stats": map[string]any{
					"get": map[string]any{
						"tags":        []string{"Sites"},
						"summary":     "Get site stats",
						"description": "Returns aggregated analytics metrics for a site.",
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/siteID"},
						},
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{"description": "Stats payload"},
						},
					},
				},
				"/api/sites/{id}/hits": map[string]any{
					"get": map[string]any{
						"tags":    []string{"Sites"},
						"summary": "List raw hits",
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/siteID"},
						},
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{"description": "Paginated hit list"},
						},
					},
				},
				"/api/sites/{id}/hits/export": map[string]any{
					"get": map[string]any{
						"tags":    []string{"Sites"},
						"summary": "Export site hits",
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/siteID"},
						},
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{"description": "Export file stream"},
						},
					},
				},
				"/api/sites/{id}/goals": map[string]any{
					"get":  siteScopedAPIKeyPathOp("Goals", "List goals for a site", "200", "Goal list"),
					"post": siteScopedAPIKeyPathOp("Goals", "Create goal", "201", "Goal created"),
				},
				"/api/sites/{id}/goals/timeseries": map[string]any{
					"get": map[string]any{
						"tags":    []string{"Goals"},
						"summary": "Get goal timeseries",
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/siteID"},
						},
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{"200": map[string]any{"description": "Timeseries payload"}},
					},
				},
				"/api/sites/{id}/funnels": map[string]any{
					"get":  siteScopedAPIKeyPathOp("Goals", "List funnels for a site", "200", "Funnel list"),
					"post": siteScopedAPIKeyPathOp("Goals", "Create funnel", "201", "Funnel created"),
				},
				"/api/sites/{id}/funnels/timeseries": map[string]any{
					"get": map[string]any{
						"tags":    []string{"Goals"},
						"summary": "Get funnel timeseries",
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/siteID"},
						},
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{"200": map[string]any{"description": "Timeseries payload"}},
					},
				},
				"/api/sites/{id}/funnels/{funnelID}/stats": map[string]any{
					"get": map[string]any{
						"tags":    []string{"Goals"},
						"summary": "Get single funnel stats",
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/siteID"},
							map[string]any{"$ref": "#/components/parameters/funnelID"},
						},
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
							map[string]any{"bearerAuth": []any{}},
							map[string]any{"apiKeyAuth": []any{}},
						},
						"responses": map[string]any{"200": map[string]any{"description": "Funnel stats payload"}},
					},
				},
				"/api/user/api-clients": map[string]any{
					"get": map[string]any{
						"tags":    []string{"API Clients"},
						"summary": "List API clients",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{
								"description": "API client list",
								"content": map[string]any{
									"application/json": map[string]any{
										"schema": map[string]any{
											"type":  "array",
											"items": map[string]any{"$ref": "#/components/schemas/APIClient"},
										},
									},
								},
							},
						},
					},
					"post": map[string]any{
						"tags":    []string{"API Clients"},
						"summary": "Create API client",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
						},
						"responses": map[string]any{
							"201": map[string]any{
								"description": "Created API client and one-time token",
							},
						},
					},
				},
				"/api/user/api-clients/{id}": map[string]any{
					"put": map[string]any{
						"tags": []string{"API Clients"},
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/apiClientID"},
						},
						"summary": "Update API client",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{"description": "Updated API client"},
						},
					},
					"delete": map[string]any{
						"tags": []string{"API Clients"},
						"parameters": []any{
							map[string]any{"$ref": "#/components/parameters/apiClientID"},
						},
						"summary": "Delete API client",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
						},
						"responses": map[string]any{
							"204": map[string]any{"description": "API client deleted"},
						},
					},
				},
				"/api/user/permissions": map[string]any{
					"get": map[string]any{
						"tags":    []string{"Permissions"},
						"summary": "Get current user permissions",
						"security": []any{
							map[string]any{"cookieAuth": []any{}},
						},
						"responses": map[string]any{
							"200": map[string]any{"description": "Permission context"},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(spec)
	}
}

func siteScopedAPIKeyPathOp(tag string, summary string, successCode string, successDescription string) map[string]any {
	return map[string]any{
		"tags":    []string{tag},
		"summary": summary,
		"parameters": []any{
			map[string]any{"$ref": "#/components/parameters/siteID"},
		},
		"security": []any{
			map[string]any{"cookieAuth": []any{}},
			map[string]any{"bearerAuth": []any{}},
			map[string]any{"apiKeyAuth": []any{}},
		},
		"responses": map[string]any{successCode: map[string]any{"description": successDescription}},
	}
}
