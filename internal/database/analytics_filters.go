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
		normalized := filterValue
		if isDirectReferrer(filterValue) {
			normalized = "(Direct)"
		}
		expr := fmt.Sprintf("hk_referrer(%sreferrer)", prefix)
		return " AND " + expr + " = ?", []any{normalized}
	case "device":
		expr := fmt.Sprintf("hk_device(%sviewport_width)", prefix)
		return " AND " + expr + " = ?", []any{filterValue}
	case "country":
		normalized := filterValue
		if isUnknownCountry(filterValue) {
			normalized = "(Unknown)"
		}
		expr := fmt.Sprintf("hk_country(%scountry_code)", prefix)
		return " AND " + expr + " = ?", []any{normalized}
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
