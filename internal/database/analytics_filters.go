package database

import (
	"fmt"
	"strings"

	"hitkeep/internal/api"
)

func buildHitFilters(filters []api.Filter, alias string) (string, []any) {
	if len(filters) == 0 {
		return "", nil
	}

	var sql strings.Builder
	var args []any
	for _, filter := range filters {
		clause, clauseArgs := buildHitFilter(filter.Type, filter.Value, alias)
		if clause == "" {
			continue
		}
		sql.WriteString(clause)
		args = append(args, clauseArgs...)
	}

	return sql.String(), args
}

func buildHitFilter(filterType, filterValue, alias string) (string, []any) {
	if filterType == "" || filterValue == "" {
		return "", nil
	}

	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}

	switch filterType {
	case "path":
		return fmt.Sprintf(" AND %spath = ?", prefix), []any{filterValue}
	case "referrer":
		if isDirectReferrer(filterValue) {
			return fmt.Sprintf(" AND (%sreferrer IS NULL OR %sreferrer = '')", prefix, prefix), nil
		}
		expr := fmt.Sprintf(`CASE
			WHEN %[1]sreferrer IS NULL OR %[1]sreferrer = '' THEN '(Direct)'
			WHEN %[1]sreferrer LIKE 'http%%' THEN regexp_extract(%[1]sreferrer, 'https?://([^/]+)', 1)
			ELSE %[1]sreferrer
		END`, prefix)
		return " AND " + expr + " = ?", []any{filterValue}
	case "device":
		expr := fmt.Sprintf(`CASE
			WHEN %[1]sviewport_width < 576 THEN 'Mobile'
			WHEN %[1]sviewport_width < 992 THEN 'Tablet'
			ELSE 'Desktop'
		END`, prefix)
		return " AND " + expr + " = ?", []any{filterValue}
	case "country":
		if isUnknownCountry(filterValue) {
			return fmt.Sprintf(" AND (%scountry_code IS NULL OR %scountry_code = '')", prefix, prefix), nil
		}
		return fmt.Sprintf(" AND %scountry_code = ?", prefix), []any{filterValue}
	default:
		return "", nil
	}
}

func isDirectReferrer(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "direct" || normalized == "(direct)"
}

func isUnknownCountry(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "unknown" || normalized == "(unknown)"
}
