package securitymaster

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

type FileSource struct {
	Path   string
	Source string
}

func (s FileSource) Fetch(ctx context.Context, markets []string) ([]SourceRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := strings.TrimSpace(s.Path)
	if path == "" {
		return nil, fmt.Errorf("--file is required for file source")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	allowed := map[string]struct{}{}
	for _, market := range markets {
		allowed[market] = struct{}{}
	}
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	index := headerIndex(header)
	var rows []SourceRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		market := field(record, index, "market")
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(market))]; !ok {
			continue
		}
		lotSize, err := parseOptionalInt(field(record, index, "lot_size"))
		if err != nil {
			return nil, fmt.Errorf("lot_size: %w", err)
		}
		pricePrecision, err := parseOptionalInt(field(record, index, "price_precision"))
		if err != nil {
			return nil, fmt.Errorf("price_precision: %w", err)
		}
		source := strings.TrimSpace(field(record, index, "source"))
		if source == "" {
			source = s.Source
		}
		row := SourceRow{
			Market:         market,
			Symbol:         field(record, index, "symbol"),
			Name:           firstNonEmpty(field(record, index, "current_name"), field(record, index, "name")),
			Exchange:       field(record, index, "exchange"),
			Board:          field(record, index, "board"),
			Status:         field(record, index, "status"),
			ListingDate:    field(record, index, "listing_date"),
			DelistingDate:  field(record, index, "delisting_date"),
			LotSize:        lotSize,
			PricePrecision: pricePrecision,
			Source:         source,
			Aliases:        aliasesFromCSV(field(record, index, "aliases"), source),
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func headerIndex(header []string) map[string]int {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.ToLower(strings.TrimSpace(name))] = i
	}
	return index
}

func field(record []string, index map[string]int, name string) string {
	i, ok := index[name]
	if !ok || i < 0 || i >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[i])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func aliasesFromCSV(value string, source string) []Alias {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '|' || r == ';' || r == ','
	})
	aliases := make([]Alias, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		aliases = append(aliases, Alias{Alias: part, AliasType: "source", Priority: 50, Source: source})
	}
	return aliases
}
