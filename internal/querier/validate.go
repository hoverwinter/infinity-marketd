package querier

import (
	"regexp"
	"strings"
)

var (
	marketPattern = regexp.MustCompile(`^(sh|sz|bj)$`)
	symbolPattern = regexp.MustCompile(`^[0-9]{6}$`)
)

func NormalizeQuery(query BarQuery) (BarQuery, error) {
	query.Market = strings.ToLower(strings.TrimSpace(query.Market))
	query.Symbol = strings.TrimSpace(query.Symbol)
	query.Period = strings.ToLower(strings.TrimSpace(query.Period))
	query.Since = strings.TrimSpace(query.Since)
	query.Until = strings.TrimSpace(query.Until)
	if query.Period == "" {
		query.Period = "1d"
	}
	if query.Limit <= 0 {
		query.Limit = 1000
	}
	if query.Limit > 10000 {
		return query, validationError("limit must be <= 10000")
	}
	if !marketPattern.MatchString(query.Market) {
		return query, validationError("market must be sh, sz, or bj")
	}
	if !symbolPattern.MatchString(query.Symbol) {
		return query, validationError("symbol must be six digits")
	}
	switch query.Period {
	case "1d", "1m", "5m":
	default:
		return query, validationError("period must be 1d, 1m, or 5m")
	}
	return query, nil
}
