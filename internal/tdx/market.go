package tdx

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var marketFilePattern = regexp.MustCompile(`^(sh|sz|bj)([0-9]{6})\.[^.]+$`)

func InferMarketFromCode(code string) string {
	s := strings.TrimSpace(code)
	if strings.HasPrefix(s, "920") || strings.HasPrefix(s, "8") || strings.HasPrefix(s, "4") {
		return "bj"
	}
	if strings.HasPrefix(s, "6") || strings.HasPrefix(s, "9") {
		return "sh"
	}
	return "sz"
}

func ParseMarketSymbol(path string, explicitMarket string, explicitCode string) (string, string, error) {
	market := strings.ToLower(strings.TrimSpace(explicitMarket))
	symbol := strings.TrimSpace(explicitCode)

	clean := strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
	parts := strings.Split(clean, "/")
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "sh", "sz", "bj":
			if market == "" {
				market = strings.ToLower(part)
			}
		}
	}

	base := strings.ToLower(filepath.Base(clean))
	if matches := marketFilePattern.FindStringSubmatch(base); matches != nil {
		if market == "" {
			market = matches[1]
		}
		if symbol == "" {
			symbol = matches[2]
		}
	}

	if symbol == "" {
		stem := strings.TrimSuffix(base, filepath.Ext(base))
		if len(stem) == 6 && allDigits(stem) {
			symbol = stem
		}
	}
	if market == "" && symbol != "" {
		market = InferMarketFromCode(symbol)
	}
	if market != "sh" && market != "sz" && market != "bj" {
		return "", "", fmt.Errorf("unsupported market %q", market)
	}
	if len(symbol) != 6 || !allDigits(symbol) {
		return "", "", fmt.Errorf("unsupported symbol %q", symbol)
	}
	return market, symbol, nil
}

func allDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}
