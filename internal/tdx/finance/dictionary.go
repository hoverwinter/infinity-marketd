package finance

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

//go:embed metadata/*.csv
var metadataFS embed.FS

var allowedStatuses = map[string]bool{
	"confirmed":   true,
	"placeholder": true,
}

var allowedValueKinds = map[string]bool{
	"float64": true,
}

var allowedUnits = map[string]bool{
	"raw":              true,
	"percent_or_ratio": true,
	"per_share":        true,
	"date_yyyymmdd":    true,
}

func LoadFinancialItemDictionary() ([]model.FinancialItemDictionaryEntry, error) {
	f, err := metadataFS.Open("metadata/financial_items.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if got, want := strings.Join(header, ","), "item_id,name,title,category,unit,value_kind,status,source_ref"; got != want {
		return nil, fmt.Errorf("unexpected financial item dictionary header %q", got)
	}

	var out []model.FinancialItemDictionaryEntry
	seen := make(map[uint16]bool)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		itemID, err := parseUint16(record[0], "item_id")
		if err != nil {
			return nil, err
		}
		if itemID == 0 {
			return nil, fmt.Errorf("financial item dictionary has zero item_id")
		}
		if seen[itemID] {
			return nil, fmt.Errorf("duplicate financial item_id %d", itemID)
		}
		seen[itemID] = true
		entry := model.FinancialItemDictionaryEntry{
			ItemID:    itemID,
			Name:      strings.TrimSpace(record[1]),
			Title:     strings.TrimSpace(record[2]),
			Category:  strings.TrimSpace(record[3]),
			Unit:      strings.TrimSpace(record[4]),
			ValueKind: strings.TrimSpace(record[5]),
			Status:    strings.TrimSpace(record[6]),
			SourceRef: strings.TrimSpace(record[7]),
		}
		if entry.Name == "" || entry.Title == "" || entry.Category == "" || entry.Unit == "" || entry.SourceRef == "" {
			return nil, fmt.Errorf("financial item %d has missing dictionary fields", itemID)
		}
		if !allowedValueKinds[entry.ValueKind] {
			return nil, fmt.Errorf("financial item %d has unsupported value kind %q", itemID, entry.ValueKind)
		}
		if !allowedUnits[entry.Unit] {
			return nil, fmt.Errorf("financial item %d has unsupported unit %q", itemID, entry.Unit)
		}
		if !allowedStatuses[entry.Status] {
			return nil, fmt.Errorf("financial item %d has invalid status %q", itemID, entry.Status)
		}
		out = append(out, entry)
	}
	return out, nil
}

func LoadGPMetricDictionary() ([]model.GPMetricDictionaryEntry, error) {
	f, err := metadataFS.Open("metadata/gp_metrics.csv")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if got, want := strings.Join(header, ","), "metric_type,name,title,value1_meaning,value2_meaning,status,source_ref"; got != want {
		return nil, fmt.Errorf("unexpected gp metric dictionary header %q", got)
	}

	var out []model.GPMetricDictionaryEntry
	seen := make(map[uint16]bool)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		metricType, err := parseUint16(record[0], "metric_type")
		if err != nil {
			return nil, err
		}
		if metricType == 0 {
			return nil, fmt.Errorf("gp metric dictionary has zero metric_type")
		}
		if seen[metricType] {
			return nil, fmt.Errorf("duplicate gp metric_type %d", metricType)
		}
		seen[metricType] = true
		entry := model.GPMetricDictionaryEntry{
			MetricType:    metricType,
			Name:          strings.TrimSpace(record[1]),
			Title:         strings.TrimSpace(record[2]),
			Value1Meaning: strings.TrimSpace(record[3]),
			Value2Meaning: strings.TrimSpace(record[4]),
			Status:        strings.TrimSpace(record[5]),
			SourceRef:     strings.TrimSpace(record[6]),
		}
		if entry.Name == "" || entry.Title == "" || entry.Value1Meaning == "" || entry.SourceRef == "" {
			return nil, fmt.Errorf("gp metric %d has missing dictionary fields", metricType)
		}
		if !allowedStatuses[entry.Status] {
			return nil, fmt.Errorf("gp metric %d has invalid status %q", metricType, entry.Status)
		}
		out = append(out, entry)
	}
	return out, nil
}

func FinancialItemDictionaryMap(entries []model.FinancialItemDictionaryEntry) map[uint16]model.FinancialItemDictionaryEntry {
	out := make(map[uint16]model.FinancialItemDictionaryEntry, len(entries))
	for _, entry := range entries {
		out[entry.ItemID] = entry
	}
	return out
}

func GPMetricDictionaryMap(entries []model.GPMetricDictionaryEntry) map[uint16]model.GPMetricDictionaryEntry {
	out := make(map[uint16]model.GPMetricDictionaryEntry, len(entries))
	for _, entry := range entries {
		out[entry.MetricType] = entry
	}
	return out
}

func parseUint16(value string, field string) (uint16, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 16)
	if err != nil {
		return 0, fmt.Errorf("parse %s %q: %w", field, value, err)
	}
	return uint16(parsed), nil
}
