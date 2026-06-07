package finance

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"github.com/hoverwinter/infinity-marketd/internal/tdx"
)

const (
	gpcwHeaderSize    = 20
	gpcwStockItemSize = 11
	gpRecordSize      = 13
)

var (
	gpcwDATPattern = regexp.MustCompile(`^gpcw(\d{8})\.dat$`)
	gpcwZIPPattern = regexp.MustCompile(`^gpcw(\d{8})\.zip$`)
	gpDATPattern   = regexp.MustCompile(`^gp(sh|sz|bj)(\d{6})\.dat$`)
)

type FinancialDATResult struct {
	Rows       []model.FinancialRawItem
	Issues     []tdx.ParseIssue
	ReportDate time.Time
	Format     string
}

type GPMetricDATResult struct {
	Rows   []model.GPMetricValue
	Issues []tdx.ParseIssue
	Format string
}

type FinancialZipEntry struct {
	Name      string
	InputPath string
	Rows      []model.FinancialRawItem
	Issues    []tdx.ParseIssue
}

type GPZipEntry struct {
	Name      string
	InputPath string
	Rows      []model.GPMetricValue
	Issues    []tdx.ParseIssue
	Market    string
	Symbol    string
}

type FinancialZipResult struct {
	Entries         []FinancialZipEntry
	Issues          []tdx.ParseIssue
	ManifestFiles   int
	FilesDiscovered int
	Format          string
}

type GPZipResult struct {
	Entries         []GPZipEntry
	Issues          []tdx.ParseIssue
	ManifestFiles   int
	FilesDiscovered int
	Format          string
}

type manifestItem struct {
	Name string
	MD5  string
	Size int64
}

func ParseFinancialDAT(raw []byte, path string, loc *time.Location, dictionary map[uint16]model.FinancialItemDictionaryEntry) FinancialDATResult {
	result := FinancialDATResult{Format: "tdx.gpcw.<hIHL3L,index,fields:f32>"}
	if loc == nil {
		loc = time.Local
	}
	if len(raw) < gpcwHeaderSize {
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "file_too_short", Message: "gpcw file shorter than header", LogicalKey: filepath.Base(path)})
		return result
	}
	reportDateRaw := binary.LittleEndian.Uint32(raw[2:6])
	reportDate, ok := parseYYYYMMDD(int(reportDateRaw), loc)
	if !ok {
		o := uint64(2)
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "invalid_date", Message: fmt.Sprintf("invalid report date %d", reportDateRaw), Offset: &o})
	}
	result.ReportDate = reportDate
	count := int(binary.LittleEndian.Uint16(raw[6:8]))
	reportSize := int(binary.LittleEndian.Uint32(raw[12:16]))
	if count == 0 {
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "zero_valid_rows", Message: "gpcw file has zero stock records", LogicalKey: filepath.Base(path)})
		return result
	}
	if reportSize <= 0 || reportSize%4 != 0 {
		o := uint64(12)
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "unsupported_record_size", Message: fmt.Sprintf("invalid report item byte size %d", reportSize), Offset: &o})
		return result
	}
	fields := reportSize / 4
	indexEnd := gpcwHeaderSize + count*gpcwStockItemSize
	if len(raw) < indexEnd {
		o := uint64(len(raw))
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "incomplete_trailing_bytes", Message: "gpcw stock index truncated", Offset: &o})
		return result
	}
	if len(dictionary) > 0 {
		for itemID := 1; itemID <= fields; itemID++ {
			if _, ok := dictionary[uint16(itemID)]; !ok {
				result.Issues = append(result.Issues, tdx.ParseIssue{Type: "unknown_dictionary_id", Message: fmt.Sprintf("unknown financial item_id %d", itemID), LogicalKey: fmt.Sprintf("FN%d", itemID)})
			}
		}
	}

	seen := make(map[string]bool, count)
	for idx := 0; idx < count; idx++ {
		offset := gpcwHeaderSize + idx*gpcwStockItemSize
		record := raw[offset : offset+gpcwStockItemSize]
		code := strings.TrimRight(string(record[:6]), "\x00 ")
		market := tdx.InferMarketFromCode(code)
		if market == "" {
			o := uint64(offset)
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "unsupported_market", Message: fmt.Sprintf("unsupported gpcw symbol %q", code), Offset: &o, LogicalKey: code})
			continue
		}
		if !validSixDigitCode(code) {
			o := uint64(offset)
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "unsupported_symbol", Message: fmt.Sprintf("unsupported gpcw symbol %q", code), Offset: &o, LogicalKey: code})
			continue
		}
		dataOffset := int(binary.LittleEndian.Uint32(record[7:11]))
		if dataOffset < indexEnd || dataOffset+reportSize > len(raw) {
			o := uint64(offset + 7)
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "incomplete_trailing_bytes", Message: fmt.Sprintf("gpcw report data outside file at offset %d", dataOffset), Offset: &o, LogicalKey: code})
			continue
		}
		key := fmt.Sprintf("%s:%s:%s", market, code, reportDate.Format("2006-01-02"))
		if seen[key] {
			o := uint64(offset)
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "duplicate_logical_key", Message: "duplicate gpcw stock logical key", Offset: &o, LogicalKey: key})
			continue
		}
		seen[key] = true
		for itemIdx := 0; itemIdx < fields; itemIdx++ {
			value := float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[dataOffset+itemIdx*4 : dataOffset+itemIdx*4+4])))
			result.Rows = append(result.Rows, model.FinancialRawItem{
				Market:     market,
				Symbol:     code,
				ReportDate: reportDate,
				ItemID:     uint16(itemIdx + 1),
				Value:      value,
			})
		}
	}
	if len(result.Rows) == 0 {
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "zero_valid_rows", Message: "no valid gpcw rows", LogicalKey: filepath.Base(path)})
	}
	return result
}

func ParseGPMetricDAT(raw []byte, path string, market string, symbol string, loc *time.Location, dictionary map[uint16]model.GPMetricDictionaryEntry) GPMetricDATResult {
	result := GPMetricDATResult{Format: "tdx.gp.<BIff>"}
	if loc == nil {
		loc = time.Local
	}
	if len(raw)%gpRecordSize != 0 {
		o := uint64(len(raw) - len(raw)%gpRecordSize)
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "incomplete_trailing_bytes", Message: fmt.Sprintf("%d trailing bytes", len(raw)%gpRecordSize), Offset: &o})
	}
	seen := make(map[string]bool)
	for offset := 0; offset+gpRecordSize <= len(raw); offset += gpRecordSize {
		metricType := uint16(raw[offset])
		if len(dictionary) > 0 {
			if _, ok := dictionary[metricType]; !ok {
				o := uint64(offset)
				result.Issues = append(result.Issues, tdx.ParseIssue{Type: "unknown_dictionary_id", Message: fmt.Sprintf("unknown gp metric_type %d", metricType), Offset: &o, LogicalKey: fmt.Sprintf("GP%02d", metricType)})
				continue
			}
		}
		dateRaw := int(binary.LittleEndian.Uint32(raw[offset+1 : offset+5]))
		eventDate, ok := parseYYYYMMDD(dateRaw, loc)
		if !ok {
			o := uint64(offset + 1)
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "invalid_date", Message: fmt.Sprintf("invalid gp date %d", dateRaw), Offset: &o})
			continue
		}
		key := fmt.Sprintf("%s:%s:%d:%s", market, symbol, metricType, eventDate.Format("2006-01-02"))
		if seen[key] {
			o := uint64(offset)
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "duplicate_logical_key", Message: "duplicate gp metric logical key", Offset: &o, LogicalKey: key})
			continue
		}
		seen[key] = true
		result.Rows = append(result.Rows, model.GPMetricValue{
			Market:     market,
			Symbol:     symbol,
			MetricType: metricType,
			EventDate:  eventDate,
			Value1:     float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[offset+5 : offset+9]))),
			Value2:     float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[offset+9 : offset+13]))),
		})
	}
	if len(result.Rows) == 0 {
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "zero_valid_rows", Message: "no valid gp metric rows", LogicalKey: filepath.Base(path)})
	}
	return result
}

func ParseFinancialZip(zipPath string, loc *time.Location, dictionary map[uint16]model.FinancialItemDictionaryEntry) (FinancialZipResult, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return FinancialZipResult{}, err
	}
	defer reader.Close()
	result := FinancialZipResult{Format: "tdxfin.zip"}
	manifest := map[string]manifestItem{}
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if base == "gpcw.txt" {
			result.ManifestFiles++
			raw, err := readZipFileBytes(file)
			if err != nil {
				return result, err
			}
			items, issues := parseCommaManifest(raw, zipInputPath(zipPath, file.Name))
			for _, issue := range issues {
				result.Issues = append(result.Issues, issue)
			}
			for _, item := range items {
				manifest[item.Name] = item
			}
		}
	}

	directDAT := make(map[string]bool)
	var datEntries []*zip.File
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if file.FileInfo().IsDir() || !gpcwDATPattern.MatchString(base) {
			continue
		}
		directDAT[strings.TrimSuffix(base, ".dat")] = true
		datEntries = append(datEntries, file)
	}
	sort.Slice(datEntries, func(i, j int) bool { return datEntries[i].Name < datEntries[j].Name })
	var zipEntries []*zip.File
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if file.FileInfo().IsDir() || !gpcwZIPPattern.MatchString(base) {
			continue
		}
		if directDAT[strings.TrimSuffix(base, ".zip")] {
			continue
		}
		zipEntries = append(zipEntries, file)
	}
	sort.Slice(zipEntries, func(i, j int) bool { return zipEntries[i].Name < zipEntries[j].Name })
	result.FilesDiscovered = len(datEntries) + len(zipEntries)
	for _, file := range datEntries {
		raw, err := readZipFileBytes(file)
		if err != nil {
			return result, err
		}
		inputPath := zipInputPath(zipPath, file.Name)
		result.Issues = append(result.Issues, validateManifest(file.Name, raw, manifest)...)
		parsed := ParseFinancialDAT(raw, inputPath, loc, dictionary)
		result.Entries = append(result.Entries, FinancialZipEntry{Name: file.Name, InputPath: inputPath, Rows: parsed.Rows, Issues: parsed.Issues})
	}
	for _, file := range zipEntries {
		rawZip, err := readZipFileBytes(file)
		if err != nil {
			return result, err
		}
		innerName, raw, err := firstDATFromZipBytes(rawZip)
		if err != nil {
			result.Issues = append(result.Issues, tdx.ParseIssue{Type: "invalid_archive", Message: err.Error(), LogicalKey: file.Name})
			continue
		}
		inputPath := zipInputPath(zipPath, file.Name+"::"+innerName)
		result.Issues = append(result.Issues, validateManifest(file.Name, rawZip, manifest)...)
		parsed := ParseFinancialDAT(raw, inputPath, loc, dictionary)
		result.Entries = append(result.Entries, FinancialZipEntry{Name: file.Name, InputPath: inputPath, Rows: parsed.Rows, Issues: parsed.Issues})
	}
	if result.FilesDiscovered == 0 {
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "zero_valid_rows", Message: "no gpcw dat files found", LogicalKey: filepath.Base(zipPath)})
	}
	return result, nil
}

func ParseGPZip(zipPath string, loc *time.Location, dictionary map[uint16]model.GPMetricDictionaryEntry) (GPZipResult, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return GPZipResult{}, err
	}
	defer reader.Close()
	result := GPZipResult{Format: "tdxgp.zip"}
	manifest := map[string]manifestItem{}
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if base == "gpszsh.txt" {
			result.ManifestFiles++
			raw, err := readZipFileBytes(file)
			if err != nil {
				return result, err
			}
			items, issues := parseCommaManifest(raw, zipInputPath(zipPath, file.Name))
			for _, issue := range issues {
				result.Issues = append(result.Issues, issue)
			}
			for _, item := range items {
				manifest[item.Name] = item
			}
		}
		if base == "gpszsh.local" {
			result.ManifestFiles++
			raw, err := readZipFileBytes(file)
			if err != nil {
				return result, err
			}
			result.Issues = append(result.Issues, parseLocalManifestIssues(raw, zipInputPath(zipPath, file.Name))...)
		}
	}

	var datEntries []*zip.File
	for _, file := range reader.File {
		base := filepath.Base(file.Name)
		if file.FileInfo().IsDir() || !gpDATPattern.MatchString(base) {
			continue
		}
		datEntries = append(datEntries, file)
	}
	sort.Slice(datEntries, func(i, j int) bool { return datEntries[i].Name < datEntries[j].Name })
	result.FilesDiscovered = len(datEntries)
	for _, file := range datEntries {
		base := filepath.Base(file.Name)
		matches := gpDATPattern.FindStringSubmatch(base)
		market, symbol := matches[1], matches[2]
		raw, err := readZipFileBytes(file)
		if err != nil {
			return result, err
		}
		inputPath := zipInputPath(zipPath, file.Name)
		result.Issues = append(result.Issues, validateManifest(file.Name, raw, manifest)...)
		parsed := ParseGPMetricDAT(raw, inputPath, market, symbol, loc, dictionary)
		result.Entries = append(result.Entries, GPZipEntry{Name: file.Name, InputPath: inputPath, Rows: parsed.Rows, Issues: parsed.Issues, Market: market, Symbol: symbol})
	}
	if result.FilesDiscovered == 0 {
		result.Issues = append(result.Issues, tdx.ParseIssue{Type: "zero_valid_rows", Message: "no gp dat files found", LogicalKey: filepath.Base(zipPath)})
	}
	return result, nil
}

func parseCommaManifest(raw []byte, inputPath string) ([]manifestItem, []tdx.ParseIssue) {
	var out []manifestItem
	var issues []tdx.ParseIssue
	for lineNo, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			o := uint64(lineNo + 1)
			issues = append(issues, tdx.ParseIssue{Type: "invalid_manifest", Message: "manifest line must have filename,md5,size", Offset: &o, LogicalKey: inputPath})
			continue
		}
		size, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil {
			o := uint64(lineNo + 1)
			issues = append(issues, tdx.ParseIssue{Type: "invalid_manifest", Message: fmt.Sprintf("invalid manifest size %q", parts[2]), Offset: &o, LogicalKey: inputPath})
			continue
		}
		out = append(out, manifestItem{Name: strings.TrimSpace(parts[0]), MD5: strings.ToLower(strings.TrimSpace(parts[1])), Size: size})
	}
	return out, issues
}

func parseLocalManifestIssues(raw []byte, inputPath string) []tdx.ParseIssue {
	var issues []tdx.ParseIssue
	for lineNo, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "[MD5]" {
			continue
		}
		if !strings.Contains(line, "=") {
			o := uint64(lineNo + 1)
			issues = append(issues, tdx.ParseIssue{Type: "invalid_manifest", Message: "local manifest line must be filename=md5", Offset: &o, LogicalKey: inputPath})
		}
	}
	return issues
}

func validateManifest(name string, raw []byte, manifest map[string]manifestItem) []tdx.ParseIssue {
	base := filepath.Base(name)
	item, ok := manifest[base]
	if !ok {
		zipName := strings.TrimSuffix(base, ".dat") + ".zip"
		item, ok = manifest[zipName]
		if !ok {
			return nil
		}
	}
	var issues []tdx.ParseIssue
	if item.Size > 0 && item.Size != int64(len(raw)) && strings.HasSuffix(item.Name, ".dat") {
		issues = append(issues, tdx.ParseIssue{Type: "checksum_mismatch", Message: fmt.Sprintf("manifest size %d != actual size %d", item.Size, len(raw)), LogicalKey: item.Name})
	}
	if item.MD5 != "" && strings.HasSuffix(item.Name, ".dat") {
		sum := fmt.Sprintf("%x", md5.Sum(raw))
		if sum != item.MD5 {
			issues = append(issues, tdx.ParseIssue{Type: "checksum_mismatch", Message: fmt.Sprintf("manifest md5 %s != actual md5 %s", item.MD5, sum), LogicalKey: item.Name})
		}
	}
	return issues
}

func readZipFileBytes(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, rc); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstDATFromZipBytes(raw []byte) (string, []byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return "", nil, err
	}
	for _, file := range reader.File {
		if file.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(file.Name), ".dat") {
			continue
		}
		data, err := readZipFileBytes(file)
		if err != nil {
			return "", nil, err
		}
		return file.Name, data, nil
	}
	return "", nil, fmt.Errorf("no dat file found in nested zip")
}

func zipInputPath(zipPath string, entry string) string {
	return zipPath + "::" + entry
}

func parseYYYYMMDD(value int, loc *time.Location) (time.Time, bool) {
	if value <= 0 {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("20060102", fmt.Sprintf("%08d", value), loc)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func validSixDigitCode(code string) bool {
	if len(code) != 6 {
		return false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
