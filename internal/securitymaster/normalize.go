package securitymaster

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

var symbolPattern = regexp.MustCompile(`^[0-9]{6}$`)

type SourceRow struct {
	Market         string
	Symbol         string
	Name           string
	Exchange       string
	Board          string
	Status         string
	ListingDate    string
	DelistingDate  string
	LotSize        int
	PricePrecision int
	Source         string
	Aliases        []Alias
	NameHistory    []NameHistory
}

type NormalizedRow struct {
	Security Security
	Aliases  []Alias
	History  []NameHistory
}

func NormalizeMarket(value string) (string, error) {
	market := strings.ToLower(strings.TrimSpace(value))
	switch market {
	case "sh", "sz", "bj":
		return market, nil
	default:
		return "", fmt.Errorf("market must be sh, sz, or bj")
	}
}

func NormalizeMarkets(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"sh", "sz", "bj"}, nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		market, err := NormalizeMarket(value)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[market]; ok {
			continue
		}
		seen[market] = struct{}{}
		out = append(out, market)
	}
	return out, nil
}

func NormalizeSymbol(value string) (string, error) {
	symbol := strings.TrimSpace(value)
	if !symbolPattern.MatchString(symbol) {
		return "", fmt.Errorf("symbol must be six digits")
	}
	return symbol, nil
}

func NormalizeText(value string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(value) {
		if unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func NormalizeSourceRow(row SourceRow, defaultSource string) (NormalizedRow, error) {
	market, err := NormalizeMarket(row.Market)
	if err != nil {
		return NormalizedRow{}, err
	}
	symbol, err := NormalizeSymbol(row.Symbol)
	if err != nil {
		return NormalizedRow{}, err
	}
	name := strings.TrimSpace(row.Name)
	if name == "" {
		return NormalizedRow{}, fmt.Errorf("current name is required")
	}
	source := strings.TrimSpace(row.Source)
	if source == "" {
		source = defaultSource
	}
	if source == "" {
		source = "unknown"
	}
	listingDate, err := normalizeDateString(row.ListingDate)
	if err != nil {
		return NormalizedRow{}, fmt.Errorf("listing_date: %w", err)
	}
	delistingDate, err := normalizeDateString(row.DelistingDate)
	if err != nil {
		return NormalizedRow{}, fmt.Errorf("delisting_date: %w", err)
	}
	status := normalizeStatus(row.Status)
	security := Security{
		Market:          market,
		Symbol:          symbol,
		Exchange:        normalizeExchange(market, row.Exchange),
		CurrentName:     name,
		CurrentNameNorm: NormalizeText(name),
		Board:           normalizeBoard(market, symbol, row.Board),
		Status:          status,
		ListingDate:     listingDate,
		DelistingDate:   delistingDate,
		LotSize:         row.LotSize,
		PricePrecision:  row.PricePrecision,
		Source:          source,
	}
	aliases := normalizeAliases(market, symbol, source, name, row.Aliases)
	history, err := normalizeHistory(market, symbol, source, row.NameHistory)
	if err != nil {
		return NormalizedRow{}, err
	}
	if listingDate != "" {
		history = append(history, NameHistory{
			Market:    market,
			Symbol:    symbol,
			Name:      name,
			NameNorm:  NormalizeText(name),
			ValidFrom: listingDate,
			Source:    source,
		})
	}
	return NormalizedRow{Security: security, Aliases: aliases, History: history}, nil
}

func normalizeAliases(market string, symbol string, source string, currentName string, aliases []Alias) []Alias {
	out := make([]Alias, 0, len(aliases)+1)
	add := func(alias Alias) {
		alias.Market = market
		alias.Symbol = symbol
		alias.Alias = strings.TrimSpace(alias.Alias)
		if alias.Alias == "" {
			return
		}
		alias.AliasNorm = NormalizeText(alias.Alias)
		if alias.AliasNorm == "" {
			return
		}
		if alias.AliasType == "" {
			alias.AliasType = "source"
		}
		if alias.Priority == 0 {
			alias.Priority = 50
		}
		if alias.Source == "" {
			alias.Source = source
		}
		out = append(out, alias)
	}
	add(Alias{Alias: currentName, AliasType: "name", Priority: 100, Source: source})
	for _, alias := range aliases {
		add(alias)
	}
	return dedupeAliases(out)
}

func normalizeHistory(market string, symbol string, source string, history []NameHistory) ([]NameHistory, error) {
	out := make([]NameHistory, 0, len(history))
	for _, item := range history {
		item.Market = market
		item.Symbol = symbol
		item.Name = strings.TrimSpace(item.Name)
		if item.Name == "" {
			continue
		}
		validFrom, err := normalizeDateString(item.ValidFrom)
		if err != nil {
			return nil, fmt.Errorf("history valid_from: %w", err)
		}
		if validFrom == "" {
			return nil, fmt.Errorf("history valid_from is required")
		}
		validTo, err := normalizeDateString(item.ValidTo)
		if err != nil {
			return nil, fmt.Errorf("history valid_to: %w", err)
		}
		item.ValidFrom = validFrom
		item.ValidTo = validTo
		item.NameNorm = NormalizeText(item.Name)
		if item.Source == "" {
			item.Source = source
		}
		out = append(out, item)
	}
	return out, nil
}

func normalizeDateString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02", "20060102"} {
		t, err := time.ParseInLocation(layout, value, time.UTC)
		if err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("expected YYYY-MM-DD or YYYYMMDD")
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", StatusUnknown:
		return StatusUnknown
	case StatusListed, "active", "上市":
		return StatusListed
	case StatusSuspended, "停牌":
		return StatusSuspended
	case StatusDelisted, "退市":
		return StatusDelisted
	default:
		return StatusUnknown
	}
}

func normalizeExchange(market string, value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value != "" {
		return value
	}
	switch market {
	case "sh":
		return "SSE"
	case "sz":
		return "SZSE"
	case "bj":
		return "BSE"
	default:
		return ""
	}
}

func normalizeBoard(market string, symbol string, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value != "" {
		return value
	}
	switch market {
	case "bj":
		return "bse"
	case "sh":
		if strings.HasPrefix(symbol, "688") {
			return "kcb"
		}
		return "main"
	case "sz":
		if strings.HasPrefix(symbol, "300") || strings.HasPrefix(symbol, "301") {
			return "cyb"
		}
		return "main"
	default:
		return "unknown"
	}
}

func dedupeAliases(aliases []Alias) []Alias {
	seen := map[string]Alias{}
	order := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		key := alias.AliasType + "\x00" + alias.AliasNorm
		if existing, ok := seen[key]; ok {
			if alias.Priority > existing.Priority {
				seen[key] = alias
			}
			continue
		}
		seen[key] = alias
		order = append(order, key)
	}
	out := make([]Alias, 0, len(order))
	for _, key := range order {
		out = append(out, seen[key])
	}
	return out
}

func parseOptionalInt(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return n, nil
}
