package user

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"hitkeep/internal/api"
)

const (
	defaultLocaleFallback = "en-US"
)

var localeTagPattern = regexp.MustCompile(`^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$`)

type languageCandidate struct {
	tag   string
	q     float64
	order int
}

func defaultPreferencesFromHeader(header string) api.UserPreferences {
	defaultLocale := parseAcceptLanguage(header)
	defaultLocale = normalizeLocaleTag(defaultLocale)
	base := baseLocaleTag(defaultLocale)
	if base == "" {
		defaultLocale = defaultLocaleFallback
		base = defaultLocale
	}
	return api.UserPreferences{
		DefaultLocale: base,
	}
}

func validateUserPreferences(defaultLocale string) (api.UserPreferences, error) {
	normalizedDefault := normalizeLocaleTag(defaultLocale)
	base := baseLocaleTag(normalizedDefault)
	if base == "" {
		return api.UserPreferences{}, errors.New("default locale is required")
	}
	return api.UserPreferences{
		DefaultLocale: base,
	}, nil
}

func parseAcceptLanguage(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}

	parts := strings.Split(header, ",")
	candidates := make([]languageCandidate, 0, len(parts))

	for i, part := range parts {
		lang, q := parseLanguageQuality(part)
		if lang == "" {
			continue
		}
		if q == 0 {
			continue
		}
		normalized := normalizeLocaleTag(lang)
		if normalized == "" {
			continue
		}
		candidates = append(candidates, languageCandidate{
			tag:   normalized,
			q:     q,
			order: i,
		})
	}

	if len(candidates) == 0 {
		return ""
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].q == candidates[j].q {
			return candidates[i].order < candidates[j].order
		}
		return candidates[i].q > candidates[j].q
	})

	return candidates[0].tag
}

func parseLanguageQuality(raw string) (string, float64) {
	parts := strings.Split(strings.TrimSpace(raw), ";")
	if len(parts) == 0 {
		return "", 0
	}

	lang := strings.TrimSpace(parts[0])
	if lang == "" || lang == "*" {
		return "", 0
	}

	q := 1.0
	for _, part := range parts[1:] {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "q=") {
			continue
		}
		value := strings.TrimPrefix(part, "q=")
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			continue
		}
		if parsed < 0 {
			parsed = 0
		}
		if parsed > 1 {
			parsed = 1
		}
		q = parsed
		break
	}

	return lang, q
}

func normalizeLocaleTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	tag = strings.ReplaceAll(tag, "_", "-")
	if !localeTagPattern.MatchString(tag) {
		return ""
	}

	parts := strings.Split(tag, "-")
	for i, part := range parts {
		if part == "" {
			return ""
		}
		switch {
		case i == 0:
			parts[i] = strings.ToLower(part)
		case len(part) == 2:
			parts[i] = strings.ToUpper(part)
		case len(part) == 4:
			parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
		default:
			parts[i] = strings.ToLower(part)
		}
	}
	return strings.Join(parts, "-")
}

func baseLocaleTag(tag string) string {
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, "-")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
