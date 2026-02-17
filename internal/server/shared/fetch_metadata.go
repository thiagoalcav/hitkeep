package shared

import (
	"net/http"
	"net/url"
	"strings"
)

// FetchMetadataMiddleware applies defense-in-depth request isolation rules for browser requests.
func FetchMetadataMiddleware(publicURL string, next http.Handler) http.Handler {
	configuredOrigin := parseConfiguredOrigin(publicURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secFetchSite := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
		path := r.URL.Path

		if secFetchSite == "" {
			// Older/limited browsers fallback: for state-changing API requests, enforce
			// same-origin via Origin or Referer validation.
			if strings.HasPrefix(path, "/api/") && isStateChangingMethod(r.Method) {
				expectedOrigin := configuredOrigin
				if expectedOrigin == "" {
					expectedOrigin = requestOrigin(r)
				}
				if !matchesOriginOrReferer(r, expectedOrigin) {
					http.Error(w, "Missing or invalid Origin/Referer for state-changing API request", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
			return
		}

		// Dashboard/API isolation: deny browser requests coming from other sites.
		if strings.HasPrefix(path, "/api/") && secFetchSite != "same-origin" {
			http.Error(w, "Cross-site request to API forbidden", http.StatusForbidden)
			return
		}

		// Public ingest endpoints may be cross-site, but should never be navigated directly.
		if path == "/ingest" || strings.HasPrefix(path, "/ingest/") {
			mode := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Mode")))
			if mode == "navigate" || mode == "nested-navigate" {
				http.Error(w, "Ingestion endpoint is not navigable", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func parseConfiguredOrigin(publicURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil {
		return ""
	}
	return canonicalOrigin(parsed)
}

func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func matchesOriginOrReferer(r *http.Request, expectedOrigin string) bool {
	if expectedOrigin == "" {
		return false
	}

	origin := originFromURLString(r.Header.Get("Origin"))
	if origin != "" {
		return origin == expectedOrigin
	}

	refererOrigin := originFromURLString(r.Header.Get("Referer"))
	if refererOrigin != "" {
		return refererOrigin == expectedOrigin
	}

	return false
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if forwarded := firstCSVToken(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = strings.ToLower(forwarded)
	}

	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = strings.TrimSpace(r.URL.Host)
	}
	if host == "" {
		return ""
	}

	return canonicalOrigin(&url.URL{
		Scheme: scheme,
		Host:   host,
	})
}

func originFromURLString(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	return canonicalOrigin(parsed)
}

func canonicalOrigin(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}

	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return ""
	}

	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return ""
	}
	port := strings.TrimSpace(parsed.Port())
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	if strings.Contains(hostname, ":") {
		hostname = "[" + hostname + "]"
	}
	if port != "" {
		return scheme + "://" + hostname + ":" + port
	}
	return scheme + "://" + hostname
}

func firstCSVToken(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ",")
	return strings.TrimSpace(parts[0])
}
