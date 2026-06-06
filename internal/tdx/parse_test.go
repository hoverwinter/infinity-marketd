package tdx

import (
	"encoding/binary"
	"math"
	"testing"
	"time"
)

func TestParseDayBytes(t *testing.T) {
	loc := mustShanghai(t)
	raw := make([]byte, 32)
	binary.LittleEndian.PutUint32(raw[0:4], 20260605)
	binary.LittleEndian.PutUint32(raw[4:8], 1234)
	binary.LittleEndian.PutUint32(raw[8:12], 1300)
	binary.LittleEndian.PutUint32(raw[12:16], 1200)
	binary.LittleEndian.PutUint32(raw[16:20], 1288)
	binary.LittleEndian.PutUint32(raw[20:24], math.Float32bits(100000.5))
	binary.LittleEndian.PutUint32(raw[24:28], 123456)

	result := ParseDayBytes(raw, "sh600519.day", "sh", "600519", loc)
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %#v", result.Issues)
	}
	if len(result.Bars) != 1 {
		t.Fatalf("bars = %d", len(result.Bars))
	}
	bar := result.Bars[0]
	if bar.Open != 12.34 || bar.High != 13 || bar.Low != 12 || bar.Close != 12.88 {
		t.Fatalf("bad prices: %#v", bar)
	}
	if bar.TradeDate.Format("2006-01-02") != "2026-06-05" {
		t.Fatalf("bad date: %s", bar.TradeDate)
	}
	if bar.Volume != 123456 {
		t.Fatalf("bad volume: %d", bar.Volume)
	}
}

func TestParseLCMinuteBytesUsesShanghaiTradingTime(t *testing.T) {
	loc := mustShanghai(t)
	raw := make([]byte, 32)
	putPackedDate(raw[0:2], 2022, 7, 29)
	binary.LittleEndian.PutUint16(raw[2:4], 9*60+30)
	putFloat(raw[4:8], 12.88)
	putFloat(raw[8:12], 12.90)
	putFloat(raw[12:16], 12.80)
	putFloat(raw[16:20], 12.86)
	putFloat(raw[20:24], 1000)
	binary.LittleEndian.PutUint32(raw[24:28], 200)

	result := ParseMinuteBytes(raw, "sh600519.lc1", "sh", "600519", Period1m, loc)
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %#v", result.Issues)
	}
	bar := result.Bars[0]
	if got := bar.BarTime.Format("2006-01-02 15:04:05"); got != "2022-07-29 09:30:00" {
		t.Fatalf("bar time = %s", got)
	}
	if bar.BarTime.Location().String() != "Asia/Shanghai" {
		t.Fatalf("location = %s", bar.BarTime.Location())
	}
}

func TestParseIntMinuteBytesNormalizesCents(t *testing.T) {
	loc := mustShanghai(t)
	raw := make([]byte, 32)
	putPackedDate(raw[0:2], 2022, 7, 29)
	binary.LittleEndian.PutUint16(raw[2:4], 10*60)
	binary.LittleEndian.PutUint32(raw[4:8], 1288)
	binary.LittleEndian.PutUint32(raw[8:12], 1290)
	binary.LittleEndian.PutUint32(raw[12:16], 1280)
	binary.LittleEndian.PutUint32(raw[16:20], 1286)
	putFloat(raw[20:24], 1000)
	binary.LittleEndian.PutUint32(raw[24:28], 200)

	result := ParseMinuteBytes(raw, "sh600519.1", "sh", "600519", Period1m, loc)
	if len(result.Issues) != 0 {
		t.Fatalf("unexpected issues: %#v", result.Issues)
	}
	bar := result.Bars[0]
	if bar.Open != 12.88 || bar.High != 12.90 || bar.Low != 12.80 || bar.Close != 12.86 {
		t.Fatalf("bad prices: %#v", bar)
	}
}

func TestParseDuplicateConflictRecordsIssue(t *testing.T) {
	loc := mustShanghai(t)
	raw := make([]byte, 64)
	putPackedDate(raw[0:2], 2022, 7, 29)
	binary.LittleEndian.PutUint16(raw[2:4], 9*60+30)
	putFloat(raw[4:8], 12.88)
	putFloat(raw[8:12], 12.90)
	putFloat(raw[12:16], 12.80)
	putFloat(raw[16:20], 12.86)
	copy(raw[32:], raw[:32])
	putFloat(raw[48:52], 12.87)

	result := ParseMinuteBytes(raw, "sh600519.lc1", "sh", "600519", Period1m, loc)
	if len(result.Bars) != 1 {
		t.Fatalf("bars = %d", len(result.Bars))
	}
	if len(result.Issues) != 1 || result.Issues[0].Type != "conflicting_logical_key" {
		t.Fatalf("issues = %#v", result.Issues)
	}
}

func mustShanghai(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func putPackedDate(dst []byte, year int, month int, day int) {
	dateNum := uint16((year-2004)*2048 + month*100 + day)
	binary.LittleEndian.PutUint16(dst, dateNum)
}

func putFloat(dst []byte, value float32) {
	binary.LittleEndian.PutUint32(dst, math.Float32bits(value))
}
