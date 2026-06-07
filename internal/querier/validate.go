package querier

import (
	"regexp"
	"strings"
	"time"
)

var (
	marketPattern = regexp.MustCompile(`^(sh|sz|bj)$`)
	symbolPattern = regexp.MustCompile(`^[0-9]{6}$`)
)

func NormalizeQuery(query BarQuery) (BarQuery, error) {
	query.Market = strings.ToLower(strings.TrimSpace(query.Market))
	query.Symbol = strings.TrimSpace(query.Symbol)
	query.Period = strings.ToLower(strings.TrimSpace(query.Period))
	query.Adjust = strings.ToLower(strings.TrimSpace(query.Adjust))
	query.Since = strings.TrimSpace(query.Since)
	query.Until = strings.TrimSpace(query.Until)
	if query.Period == "" {
		query.Period = "1d"
	}
	if query.Adjust == "" {
		query.Adjust = "none"
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
	switch query.Adjust {
	case "none", "qfq", "hfq":
	default:
		return query, validationError("adjust must be none, qfq, or hfq")
	}
	return query, nil
}

func NormalizeIntradayPointQuery(query IntradayPointQuery) (IntradayPointQuery, error) {
	query.Market = strings.ToLower(strings.TrimSpace(query.Market))
	query.Symbol = strings.TrimSpace(query.Symbol)
	query.Date = strings.TrimSpace(query.Date)
	query.Since = strings.TrimSpace(query.Since)
	query.Until = strings.TrimSpace(query.Until)
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
	if query.Date != "" {
		if query.Since != "" || query.Until != "" {
			return query, validationError("date cannot be combined with since or until")
		}
		date, err := normalizeAPIDate(query.Date)
		if err != nil {
			return query, err
		}
		query.Date = date
		return query, nil
	}
	if query.Since == "" || query.Until == "" {
		return query, validationError("either date or both since and until are required")
	}
	since, err := parseAPIDateTime(query.Since, false)
	if err != nil {
		return query, validationError("invalid since %q", query.Since)
	}
	until, err := parseAPIDateTime(query.Until, true)
	if err != nil {
		return query, validationError("invalid until %q", query.Until)
	}
	if since.After(until) {
		return query, validationError("since must be <= until")
	}
	return query, nil
}

func normalizeAPIDate(value string) (string, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, layout := range []string{"2006-01-02", "20060102"} {
		if t, err := time.ParseInLocation(layout, value, loc); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", validationError("invalid date %q, expected YYYY-MM-DD or YYYYMMDD", value)
}

func parseAPIDateTime(value string, endOfDay bool) (time.Time, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05", time.RFC3339} {
		t, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			if layout == "2006-01-02" && endOfDay {
				return t.AddDate(0, 0, 1), nil
			}
			return t, nil
		}
	}
	return time.Time{}, validationError("invalid datetime %q", value)
}
