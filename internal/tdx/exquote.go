package tdx

import (
	"fmt"
	"strings"
)

type ExQuoteRequest struct {
	Market int
	Code   string
}

func ParseExQuoteRequest(market int, code string) (ExQuoteRequest, error) {
	code = strings.TrimSpace(code)
	if market <= 0 {
		return ExQuoteRequest{}, fmt.Errorf("extended quote market must be positive")
	}
	if code == "" || len(code) > 9 {
		return ExQuoteRequest{}, fmt.Errorf("unsupported extended quote code %q", code)
	}
	return ExQuoteRequest{Market: market, Code: code}, nil
}
