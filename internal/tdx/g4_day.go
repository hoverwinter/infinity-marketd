package tdx

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

const (
	DefaultG4DayBaseURL = "https://www.tdx.com.cn/products/data/data/g4day/"
	G4DayInputFormat    = "tdx.g4day.cod150.md1-512.v1"

	g4DayCodeRecordSize      = 150
	g4DayQuoteRecordSize     = 512
	maxG4DayPackageBytes     = 16 << 20
	maxG4DayEntryBytes       = 64 << 20
	maxG4DayExpandedBytes    = 128 << 20
	maxG4DayRecordsPerMarket = 100000
)

var g4DayEntryPattern = regexp.MustCompile(`^(sh|sz|bj)([0-9]{6})\.(cod|md1)$`)

type G4DayFetchOptions struct {
	BaseURL    string
	HTTPClient *http.Client
}

type G4DayParseResult struct {
	TradeDate        time.Time
	Bars             []model.DailyBar
	SHA256           string
	PackageBytes     uint64
	Records          uint64
	SHRecords        uint64
	SZRecords        uint64
	BJRecords        uint64
	EquityRecords    uint64
	NoTradeRecords   uint64
	NonEquityRecords uint64
}

type g4DayEntryPair struct {
	code *zip.File
	md1  *zip.File
}

func ReadG4DayPackageFile(filePath string) ([]byte, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("g4day package %s is not a regular file", filePath)
	}
	if info.Size() <= 0 {
		return nil, fmt.Errorf("g4day package %s is empty", filePath)
	}
	if info.Size() > maxG4DayPackageBytes {
		return nil, fmt.Errorf("g4day package %s is %d bytes, limit is %d", filePath, info.Size(), maxG4DayPackageBytes)
	}
	return os.ReadFile(filePath)
}

func FetchG4DayPackage(ctx context.Context, tradeDate time.Time, opts G4DayFetchOptions) ([]byte, string, error) {
	if tradeDate.IsZero() {
		return nil, "", fmt.Errorf("g4day trade date is required")
	}
	downloadURL, err := resolveG4DayURL(opts.BaseURL, tradeDate)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, downloadURL, err
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, downloadURL, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, downloadURL, fmt.Errorf("GET %s returned %s", downloadURL, resp.Status)
	}
	if resp.ContentLength > maxG4DayPackageBytes {
		return nil, downloadURL, fmt.Errorf("g4day response is %d bytes, limit is %d", resp.ContentLength, maxG4DayPackageBytes)
	}
	raw, err := readG4DayLimited(resp.Body, maxG4DayPackageBytes)
	if err != nil {
		return nil, downloadURL, err
	}
	if len(raw) == 0 {
		return nil, downloadURL, fmt.Errorf("GET %s returned an empty package", downloadURL)
	}
	return raw, downloadURL, nil
}

func ParseG4DayPackage(raw []byte, source string, expectedDate *time.Time, loc *time.Location) (G4DayParseResult, error) {
	result := G4DayParseResult{
		SHA256:       fmt.Sprintf("%x", sha256.Sum256(raw)),
		PackageBytes: uint64(len(raw)),
	}
	if len(raw) == 0 {
		return result, fmt.Errorf("g4day package %s is empty", source)
	}
	if len(raw) > maxG4DayPackageBytes {
		return result, fmt.Errorf("g4day package %s is %d bytes, limit is %d", source, len(raw), maxG4DayPackageBytes)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return result, fmt.Errorf("open g4day package %s: %w", source, err)
	}
	pairs, tradeDate, err := collectG4DayEntries(reader.File, expectedDate, loc)
	if err != nil {
		return result, fmt.Errorf("validate g4day package %s: %w", source, err)
	}
	result.TradeDate = tradeDate

	for _, market := range []string{"sh", "sz", "bj"} {
		pair := pairs[market]
		codeRaw, err := readG4DayZipEntry(pair.code)
		if err != nil {
			return result, fmt.Errorf("read %s code entry: %w", market, err)
		}
		quoteRaw, err := readG4DayZipEntry(pair.md1)
		if err != nil {
			return result, fmt.Errorf("read %s quote entry: %w", market, err)
		}
		count, err := validateG4DayPair(market, codeRaw, quoteRaw)
		if err != nil {
			return result, err
		}
		result.Records += uint64(count)
		switch market {
		case "sh":
			result.SHRecords = uint64(count)
		case "sz":
			result.SZRecords = uint64(count)
		case "bj":
			result.BJRecords = uint64(count)
		}
		if err := parseG4DayPair(&result, market, codeRaw, quoteRaw); err != nil {
			return result, err
		}
	}
	return result, nil
}

func collectG4DayEntries(files []*zip.File, expectedDate *time.Time, loc *time.Location) (map[string]g4DayEntryPair, time.Time, error) {
	pairs := make(map[string]g4DayEntryPair, 3)
	var packageDate time.Time
	var expanded uint64
	for _, file := range files {
		if file.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(path.Base(strings.ReplaceAll(file.Name, `\`, "/")))
		matches := g4DayEntryPattern.FindStringSubmatch(name)
		if matches == nil {
			continue
		}
		if file.UncompressedSize64 > maxG4DayEntryBytes {
			return nil, time.Time{}, fmt.Errorf("entry %s is %d bytes, limit is %d", file.Name, file.UncompressedSize64, maxG4DayEntryBytes)
		}
		expanded += file.UncompressedSize64
		if expanded > maxG4DayExpandedBytes {
			return nil, time.Time{}, fmt.Errorf("expanded package is %d bytes, limit is %d", expanded, maxG4DayExpandedBytes)
		}
		entryDate, err := parseG4DayEntryDate(matches[2], loc)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("entry %s: %w", file.Name, err)
		}
		if packageDate.IsZero() {
			packageDate = entryDate
		} else if !sameG4DayDate(packageDate, entryDate) {
			return nil, time.Time{}, fmt.Errorf("entry %s date %s does not match package date %s", file.Name, entryDate.Format("2006-01-02"), packageDate.Format("2006-01-02"))
		}
		market, kind := matches[1], matches[3]
		pair := pairs[market]
		switch kind {
		case "cod":
			if pair.code != nil {
				return nil, time.Time{}, fmt.Errorf("duplicate %s .cod entry", market)
			}
			pair.code = file
		case "md1":
			if pair.md1 != nil {
				return nil, time.Time{}, fmt.Errorf("duplicate %s .md1 entry", market)
			}
			pair.md1 = file
		}
		pairs[market] = pair
	}
	if packageDate.IsZero() {
		return nil, time.Time{}, fmt.Errorf("no g4day entries found")
	}
	if expectedDate != nil && !sameG4DayDate(packageDate, *expectedDate) {
		return nil, time.Time{}, fmt.Errorf("package date %s does not match requested date %s", packageDate.Format("2006-01-02"), expectedDate.Format("2006-01-02"))
	}
	for _, market := range []string{"sh", "sz", "bj"} {
		pair := pairs[market]
		if pair.code == nil || pair.md1 == nil {
			return nil, time.Time{}, fmt.Errorf("missing %s .cod/.md1 pair", market)
		}
	}
	return pairs, packageDate, nil
}

func validateG4DayPair(market string, codeRaw []byte, quoteRaw []byte) (int, error) {
	if len(codeRaw)%g4DayCodeRecordSize != 0 {
		return 0, fmt.Errorf("%s .cod size %d is not divisible by %d", market, len(codeRaw), g4DayCodeRecordSize)
	}
	if len(quoteRaw)%g4DayQuoteRecordSize != 0 {
		return 0, fmt.Errorf("%s .md1 size %d is not divisible by %d", market, len(quoteRaw), g4DayQuoteRecordSize)
	}
	codeCount := len(codeRaw) / g4DayCodeRecordSize
	quoteCount := len(quoteRaw) / g4DayQuoteRecordSize
	if codeCount == 0 {
		return 0, fmt.Errorf("%s pair has zero records", market)
	}
	if codeCount != quoteCount {
		return 0, fmt.Errorf("%s record count mismatch: .cod=%d .md1=%d", market, codeCount, quoteCount)
	}
	if codeCount > maxG4DayRecordsPerMarket {
		return 0, fmt.Errorf("%s record count %d exceeds limit %d", market, codeCount, maxG4DayRecordsPerMarket)
	}
	return codeCount, nil
}

func parseG4DayPair(result *G4DayParseResult, market string, codeRaw []byte, quoteRaw []byte) error {
	seen := make(map[string]struct{}, len(codeRaw)/g4DayCodeRecordSize)
	for record := 0; record < len(codeRaw)/g4DayCodeRecordSize; record++ {
		codeOffset := record * g4DayCodeRecordSize
		quoteOffset := record * g4DayQuoteRecordSize
		symbol := string(codeRaw[codeOffset : codeOffset+6])
		if len(symbol) != 6 || !allDigits(symbol) {
			return fmt.Errorf("%s .cod record %d has invalid symbol %q", market, record, symbol)
		}
		if _, ok := seen[symbol]; ok {
			return fmt.Errorf("%s .cod record %d duplicates symbol %s", market, record, symbol)
		}
		seen[symbol] = struct{}{}
		if !isG4DayAShare(market, symbol) {
			result.NonEquityRecords++
			continue
		}
		result.EquityRecords++
		recordRaw := quoteRaw[quoteOffset : quoteOffset+g4DayQuoteRecordSize]
		open := math.Float64frombits(binary.LittleEndian.Uint64(recordRaw[12:20]))
		high := math.Float64frombits(binary.LittleEndian.Uint64(recordRaw[20:28]))
		low := math.Float64frombits(binary.LittleEndian.Uint64(recordRaw[28:36]))
		closeValue := math.Float64frombits(binary.LittleEndian.Uint64(recordRaw[36:44]))
		volume := binary.LittleEndian.Uint64(recordRaw[56:64])
		amount := math.Float64frombits(binary.LittleEndian.Uint64(recordRaw[72:80]))

		if g4DayNoTrade(open, high, low, closeValue, volume, amount) {
			result.NoTradeRecords++
			continue
		}
		if err := validateG4DayBar(open, high, low, closeValue, volume, amount); err != nil {
			return fmt.Errorf("%s:%s record %d: %w", market, symbol, record, err)
		}
		result.Bars = append(result.Bars, model.DailyBar{
			Market:    market,
			Symbol:    symbol,
			TradeDate: result.TradeDate,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeValue,
			Volume:    volume,
			Amount:    amount,
		})
	}
	return nil
}

func isG4DayAShare(market string, symbol string) bool {
	switch market {
	case "sh":
		return strings.HasPrefix(symbol, "6")
	case "sz":
		for _, prefix := range []string{"000", "001", "002", "003", "300", "301"} {
			if strings.HasPrefix(symbol, prefix) {
				return true
			}
		}
		return false
	case "bj":
		return strings.HasPrefix(symbol, "920")
	default:
		return false
	}
}

func g4DayNoTrade(open float64, high float64, low float64, closeValue float64, volume uint64, amount float64) bool {
	return finiteG4DayValue(open) && finiteG4DayValue(high) && finiteG4DayValue(low) && finiteG4DayValue(closeValue) && finiteG4DayValue(amount) &&
		open == 0 && high == 0 && low == 0 && volume == 0 && amount == 0 && closeValue >= 0
}

func validateG4DayBar(open float64, high float64, low float64, closeValue float64, volume uint64, amount float64) error {
	for name, value := range map[string]float64{"open": open, "high": high, "low": low, "close": closeValue, "amount": amount} {
		if !finiteG4DayValue(value) {
			return fmt.Errorf("%s is not finite", name)
		}
		if value <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if volume == 0 {
		return fmt.Errorf("volume must be positive")
	}
	if high < low || high < open || high < closeValue || low > open || low > closeValue {
		return fmt.Errorf("inconsistent OHLC values")
	}
	return nil
}

func finiteG4DayValue(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func readG4DayZipEntry(file *zip.File) ([]byte, error) {
	if file.UncompressedSize64 > maxG4DayEntryBytes {
		return nil, fmt.Errorf("entry %s is %d bytes, limit is %d", file.Name, file.UncompressedSize64, maxG4DayEntryBytes)
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	raw, err := readG4DayLimited(reader, int64(maxG4DayEntryBytes))
	if err != nil {
		return nil, err
	}
	if uint64(len(raw)) != file.UncompressedSize64 {
		return nil, fmt.Errorf("entry %s read %d bytes, expected %d", file.Name, len(raw), file.UncompressedSize64)
	}
	return raw, nil
}

func readG4DayLimited(reader io.Reader, limit int64) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit {
		return nil, fmt.Errorf("g4day input exceeds %d-byte limit", limit)
	}
	return raw, nil
}

func resolveG4DayURL(baseURL string, tradeDate time.Time) (string, error) {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = DefaultG4DayBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("g4day base URL must be an absolute http(s) URL")
	}
	ref := &url.URL{Path: tradeDate.Format("20060102") + ".zip"}
	return parsed.ResolveReference(ref).String(), nil
}

func parseG4DayEntryDate(value string, loc *time.Location) (time.Time, error) {
	if len(value) != 6 || !allDigits(value) {
		return time.Time{}, fmt.Errorf("invalid YYMMDD date %q", value)
	}
	year := 2000 + int(value[0]-'0')*10 + int(value[1]-'0')
	month := time.Month(int(value[2]-'0')*10 + int(value[3]-'0'))
	day := int(value[4]-'0')*10 + int(value[5]-'0')
	date, ok := validDate(year, month, day, loc)
	if !ok {
		return time.Time{}, fmt.Errorf("invalid YYMMDD date %q", value)
	}
	return date, nil
}

func sameG4DayDate(a time.Time, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
