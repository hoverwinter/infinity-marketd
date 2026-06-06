package tdx

import (
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type ParseIssue struct {
	Type       string
	Message    string
	Offset     *uint64
	LogicalKey string
}

type DailyParseResult struct {
	Bars   []model.DailyBar
	Issues []ParseIssue
}

type MinuteParseResult struct {
	Bars   []model.MinuteBar
	Issues []ParseIssue
	Format string
}

func ParseDayBytes(raw []byte, path string, market string, symbol string, loc *time.Location) DailyParseResult {
	result := DailyParseResult{}
	seen := make(map[string]model.DailyBar)
	for offset := 0; offset < len(raw); offset += 32 {
		if len(raw)-offset < 32 {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: fmt.Sprintf("%d trailing bytes", len(raw)-offset), Offset: &o})
			break
		}
		chunk := raw[offset : offset+32]
		dateRaw := binary.LittleEndian.Uint32(chunk[0:4])
		year := int(dateRaw / 10000)
		month := time.Month((dateRaw / 100) % 100)
		day := int(dateRaw % 100)
		tradeDate, ok := validDate(year, month, day, loc)
		if !ok {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "invalid_date", Message: fmt.Sprintf("invalid date %d", dateRaw), Offset: &o})
			continue
		}
		open := float64(binary.LittleEndian.Uint32(chunk[4:8])) / 100.0
		high := float64(binary.LittleEndian.Uint32(chunk[8:12])) / 100.0
		low := float64(binary.LittleEndian.Uint32(chunk[12:16])) / 100.0
		close := float64(binary.LittleEndian.Uint32(chunk[16:20])) / 100.0
		amount := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[20:24])))
		volume := uint64(binary.LittleEndian.Uint32(chunk[24:28]))
		if high < low {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "high_less_than_low", Message: "high is less than low", Offset: &o})
			continue
		}
		bar := model.DailyBar{
			Market:    market,
			Symbol:    symbol,
			TradeDate: tradeDate,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Amount:    amount,
		}
		key := dailyKey(bar)
		if prev, exists := seen[key]; exists {
			o := uint64(offset)
			issueType := "duplicate_logical_key"
			if !equalDaily(prev, bar) {
				issueType = "conflicting_logical_key"
			}
			result.Issues = append(result.Issues, ParseIssue{Type: issueType, Message: "duplicate daily logical key", Offset: &o, LogicalKey: key})
			continue
		}
		seen[key] = bar
		result.Bars = append(result.Bars, bar)
	}
	if len(result.Bars) == 0 {
		result.Issues = append(result.Issues, ParseIssue{Type: "zero_valid_rows", Message: "no valid daily bars", LogicalKey: filepath.Base(path)})
	}
	return result
}

func ParseMinuteBytes(raw []byte, path string, market string, symbol string, period Period, loc *time.Location) MinuteParseResult {
	ext := filepath.Ext(path)
	intFormat := ext == ".1" || ext == ".5"
	format := "tdx.lcmin.<HHfffffII>"
	if intFormat {
		format = "tdx.intmin.<HHIIIIfII>"
	}
	result := MinuteParseResult{Format: format}
	seen := make(map[string]model.MinuteBar)
	for offset := 0; offset < len(raw); offset += 32 {
		if len(raw)-offset < 32 {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: fmt.Sprintf("%d trailing bytes", len(raw)-offset), Offset: &o})
			break
		}
		chunk := raw[offset : offset+32]
		barTime, ok := decodeMinuteTime(binary.LittleEndian.Uint16(chunk[0:2]), binary.LittleEndian.Uint16(chunk[2:4]), loc)
		if !ok {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "invalid_time", Message: "invalid packed date or minute", Offset: &o})
			continue
		}
		var open, high, low, close, amount float64
		if intFormat {
			open = float64(binary.LittleEndian.Uint32(chunk[4:8])) / 100.0
			high = float64(binary.LittleEndian.Uint32(chunk[8:12])) / 100.0
			low = float64(binary.LittleEndian.Uint32(chunk[12:16])) / 100.0
			close = float64(binary.LittleEndian.Uint32(chunk[16:20])) / 100.0
			amount = float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[20:24])))
		} else {
			open = float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[4:8])))
			high = float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[8:12])))
			low = float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[12:16])))
			close = float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[16:20])))
			amount = float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[20:24])))
		}
		volume := uint64(binary.LittleEndian.Uint32(chunk[24:28]))
		if high < low {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "high_less_than_low", Message: "high is less than low", Offset: &o})
			continue
		}
		tradeDate := time.Date(barTime.Year(), barTime.Month(), barTime.Day(), 0, 0, 0, 0, loc)
		bar := model.MinuteBar{
			Market:    market,
			Symbol:    symbol,
			BarTime:   barTime,
			TradeDate: tradeDate,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
			Amount:    amount,
		}
		key := minuteKey(bar)
		if prev, exists := seen[key]; exists {
			o := uint64(offset)
			issueType := "duplicate_logical_key"
			if !equalMinute(prev, bar) {
				issueType = "conflicting_logical_key"
			}
			result.Issues = append(result.Issues, ParseIssue{Type: issueType, Message: fmt.Sprintf("duplicate %s logical key", period), Offset: &o, LogicalKey: key})
			continue
		}
		seen[key] = bar
		result.Bars = append(result.Bars, bar)
	}
	if len(result.Bars) == 0 {
		result.Issues = append(result.Issues, ParseIssue{Type: "zero_valid_rows", Message: "no valid minute bars", LogicalKey: filepath.Base(path)})
	}
	return result
}

func decodeMinuteTime(dateNum uint16, minuteNum uint16, loc *time.Location) (time.Time, bool) {
	year := int(dateNum/2048) + 2004
	rem := int(dateNum % 2048)
	month := time.Month(rem / 100)
	day := rem % 100
	hour := int(minuteNum / 60)
	minute := int(minuteNum % 60)
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return time.Time{}, false
	}
	d, ok := validDate(year, month, day, loc)
	if !ok {
		return time.Time{}, false
	}
	return time.Date(d.Year(), d.Month(), d.Day(), hour, minute, 0, 0, loc), true
}

func validDate(year int, month time.Month, day int, loc *time.Location) (time.Time, bool) {
	if year < 1900 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Time{}, false
	}
	d := time.Date(year, month, day, 0, 0, 0, 0, loc)
	return d, d.Year() == year && d.Month() == month && d.Day() == day
}

func dailyKey(bar model.DailyBar) string {
	return fmt.Sprintf("%s:%s:%s", bar.Market, bar.Symbol, bar.TradeDate.Format("2006-01-02"))
}

func minuteKey(bar model.MinuteBar) string {
	return fmt.Sprintf("%s:%s:%s", bar.Market, bar.Symbol, bar.BarTime.Format("2006-01-02 15:04:05"))
}

func equalDaily(a, b model.DailyBar) bool {
	return a.Market == b.Market && a.Symbol == b.Symbol && a.TradeDate.Equal(b.TradeDate) && a.Open == b.Open && a.High == b.High && a.Low == b.Low && a.Close == b.Close && a.Volume == b.Volume && a.Amount == b.Amount
}

func equalMinute(a, b model.MinuteBar) bool {
	return a.Market == b.Market && a.Symbol == b.Symbol && a.BarTime.Equal(b.BarTime) && a.Open == b.Open && a.High == b.High && a.Low == b.Low && a.Close == b.Close && a.Volume == b.Volume && a.Amount == b.Amount
}
