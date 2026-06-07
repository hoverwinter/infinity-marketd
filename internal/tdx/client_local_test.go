package tdx

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func TestParseGBBQBytes(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	raw := make([]byte, 4)
	binary.LittleEndian.PutUint32(raw[:4], 3)
	raw = append(raw, gbbqRecord(tdxMarketSH, "600519", 20260605, 1, 1.2, 2.3, 3.4, 4.5)...)
	raw = append(raw, gbbqRecord(tdxMarketSZ, "000001", 20260606, 5, 10, 20, 30, 40)...)
	raw = append(raw, gbbqRecord(tdxMarketSH, "600000", 20261301, 99, 1, 2, 3, 4)...)

	result := ParseGBBQBytes(raw, "gbbq", loc)
	if len(result.Events) != 2 {
		t.Fatalf("events = %d issues=%#v", len(result.Events), result.Issues)
	}
	first := result.Events[0]
	if first.Market != "sh" || first.Symbol != "600519" || first.Category != 1 || first.CashDividend == nil || math.Abs(*first.CashDividend-1.2) > 0.0001 {
		t.Fatalf("first event = %#v", first)
	}
	second := result.Events[1]
	if second.EventName != "股本变化" || second.PreFloatShares == nil || math.Abs(*second.PreFloatShares-10) > 0.0001 {
		t.Fatalf("second event = %#v", second)
	}
	if len(result.Issues) != 1 || result.Issues[0].Type != "invalid_date" {
		t.Fatalf("issues = %#v", result.Issues)
	}
}

func TestParseSystemBlockBytes(t *testing.T) {
	raw := make([]byte, 386)
	binary.LittleEndian.PutUint16(raw[384:386], 1)
	raw = append(raw, fixedBytes([]byte("TEST"), 9)...)
	tmp := make([]byte, 4)
	binary.LittleEndian.PutUint16(tmp[:2], 2)
	binary.LittleEndian.PutUint16(tmp[2:4], 7)
	raw = append(raw, tmp...)
	memberArea := make([]byte, 2800)
	copy(memberArea[0:7], []byte("1600519"))
	copy(memberArea[7:14], []byte("0000001"))
	raw = append(raw, memberArea...)

	result := ParseSystemBlockBytes(raw, "block_gn.dat", "system", time.Unix(0, 0))
	if len(result.Definitions) != 1 || len(result.Memberships) != 2 {
		t.Fatalf("defs=%d members=%d issues=%#v", len(result.Definitions), len(result.Memberships), result.Issues)
	}
	if result.Definitions[0].BlockKind != "block_gn" || result.Definitions[0].BlockType != 7 {
		t.Fatalf("definition = %#v", result.Definitions[0])
	}
	if result.Memberships[0].Market != "sh" || result.Memberships[0].Symbol != "600519" {
		t.Fatalf("member 0 = %#v", result.Memberships[0])
	}
	if result.Memberships[1].Market != "sz" || result.Memberships[1].Symbol != "000001" {
		t.Fatalf("member 1 = %#v", result.Memberships[1])
	}
	if result.Snapshot.SnapshotID == "" || result.Snapshot.SnapshotID != result.Snapshot.ContentHash {
		t.Fatalf("snapshot = %#v", result.Snapshot)
	}

	again := ParseSystemBlockBytes(raw, "block_gn.dat", "system", time.Unix(10, 0))
	if again.Snapshot.SnapshotID != result.Snapshot.SnapshotID {
		t.Fatalf("snapshot id not deterministic: %s != %s", again.Snapshot.SnapshotID, result.Snapshot.SnapshotID)
	}
}

func TestParseCustomBlockDirAndEdit(t *testing.T) {
	dir := t.TempDir()
	cfg := append(fixedBytes([]byte("Watch"), 50), fixedBytes([]byte("watch"), 70)...)
	cfg = append(cfg, append(fixedBytes([]byte("Empty"), 50), fixedBytes([]byte("empty"), 70)...)...)
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch.blk"), []byte("1600519\n0000001\n1600519\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty.blk"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Definitions) != 2 || len(result.Memberships) != 2 {
		t.Fatalf("defs=%d members=%d issues=%#v", len(result.Definitions), len(result.Memberships), result.Issues)
	}
	edited, err := ApplyCustomBlockEdit(result, CustomBlockEdit{BlockID: "watch", Add: []string{"sz:000002"}, Remove: []string{"600519"}})
	if err != nil {
		t.Fatal(err)
	}
	var symbols []string
	for _, member := range edited.Memberships {
		if member.BlockID == "watch" {
			symbols = append(symbols, member.Symbol)
		}
	}
	if len(symbols) != 2 || symbols[0] != "000001" || symbols[1] != "000002" {
		t.Fatalf("symbols = %#v", symbols)
	}
}

func TestWriteCustomBlockDirRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfg := append(fixedBytes([]byte("Watch"), 50), fixedBytes([]byte("watch"), 70)...)
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch.blk"), []byte("1600519\n0000002\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	edited, err := ApplyCustomBlockEdit(parsed, CustomBlockEdit{BlockID: "watch", Add: []string{"sz:000001"}, Remove: []string{"600519"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCustomBlockDir(dir, edited); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "blocknew.cfg.bak")); err != nil {
		t.Fatalf("missing backup: %v", err)
	}
	verified, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if verified.Snapshot.ContentHash != edited.Snapshot.ContentHash {
		t.Fatalf("content hash = %s, want %s", verified.Snapshot.ContentHash, edited.Snapshot.ContentHash)
	}
	if len(verified.Memberships) != 2 {
		t.Fatalf("memberships = %d issues=%#v", len(verified.Memberships), verified.Issues)
	}
	got := string(mustReadFile(t, filepath.Join(dir, "watch.blk")))
	if !strings.Contains(got, "0000002") || !strings.Contains(got, "0000001") || strings.Contains(got, "600519") {
		t.Fatalf("watch.blk = %q", got)
	}
}

func TestApplyCustomBlockEditRejectsUnsupportedSymbol(t *testing.T) {
	current := BlockParseResult{
		Definitions: []model.TDXBlockDefinition{{BlockID: "watch", BlockName: "Watch"}},
	}
	assignBlockSnapshot(&current, "custom", time.Unix(0, 0))
	if _, err := ApplyCustomBlockEdit(current, CustomBlockEdit{BlockID: "watch", Add: []string{"not-a-symbol"}}); err == nil {
		t.Fatal("expected unsupported symbol error")
	}
}

func TestWriteCustomBlockDirBackupFailureDoesNotModifyTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod write denial is platform-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions, so chmod cannot force a write failure")
	}
	dir := t.TempDir()
	cfg := append(fixedBytes([]byte("Watch"), 50), fixedBytes([]byte("watch"), 70)...)
	blk := []byte("1600519\n")
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch.blk"), blk, 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	edited, err := ApplyCustomBlockEdit(parsed, CustomBlockEdit{BlockID: "watch", Add: []string{"000001"}})
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	err = WriteCustomBlockDir(dir, edited)
	if chmodErr := os.Chmod(dir, 0o700); chmodErr != nil {
		t.Fatal(chmodErr)
	}
	if err == nil {
		t.Fatal("expected backup failure")
	}
	if got := mustReadFile(t, filepath.Join(dir, "blocknew.cfg")); string(got) != string(cfg) {
		t.Fatalf("cfg modified after backup failure")
	}
	if got := mustReadFile(t, filepath.Join(dir, "watch.blk")); string(got) != string(blk) {
		t.Fatalf("blk modified after backup failure: %q", got)
	}
}

func TestWriteCustomBlockDirRejectsMalformedParsedDataWithoutPartialWrite(t *testing.T) {
	dir := t.TempDir()
	cfg := append(fixedBytes([]byte("Watch"), 50), fixedBytes([]byte("watch"), 70)...)
	blk := []byte("1600519\n")
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch.blk"), blk, 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	parsed.Definitions[0].BlockName = strings.Repeat("X", 51)
	if err := WriteCustomBlockDir(dir, parsed); err == nil {
		t.Fatal("expected malformed write plan error")
	}
	if got := mustReadFile(t, filepath.Join(dir, "blocknew.cfg")); string(got) != string(cfg) {
		t.Fatalf("cfg modified after malformed write")
	}
	if got := mustReadFile(t, filepath.Join(dir, "watch.blk")); string(got) != string(blk) {
		t.Fatalf("blk modified after malformed write: %q", got)
	}
}

func TestWriteCustomBlockDirPostWriteValidationFailure(t *testing.T) {
	dir := t.TempDir()
	cfg := append(fixedBytes([]byte("Watch"), 50), fixedBytes([]byte("watch"), 70)...)
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), cfg, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "watch.blk"), []byte("1600519\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	parsed.Snapshot.ContentHash = "wrong"
	if err := WriteCustomBlockDir(dir, parsed); err == nil || !strings.Contains(err.Error(), "post-write validation failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestParseCustomBlockDirMalformedConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blocknew.cfg"), []byte{1}, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := ParseCustomBlockDir(dir, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Issues) != 1 || result.Issues[0].Type != "incomplete_trailing_bytes" {
		t.Fatalf("issues = %#v", result.Issues)
	}
}

func TestParseExDailyBytes(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	raw := exDailyRecord(20260605, 1, 2, 0.5, 1.5, 100, 20.5)
	raw = append(raw, exDailyRecord(20261301, 1, 2, 0.5, 1.5, 100, 20.5)...)
	raw = append(raw, []byte{1, 2, 3}...)

	result := ParseExDailyBytes(raw, "L001/29#A1801.day", 29, "A1801", loc)
	if len(result.Bars) != 1 {
		t.Fatalf("bars=%d issues=%#v", len(result.Bars), result.Issues)
	}
	bar := result.Bars[0]
	if bar.ExMarket != 29 || bar.Code != "A1801" || bar.Amount == nil || *bar.Amount != 100 {
		t.Fatalf("bar = %#v", bar)
	}
	if len(result.Issues) != 2 {
		t.Fatalf("issues = %#v", result.Issues)
	}
}

func gbbqRecord(market int, symbol string, date uint32, category byte, v1, v2, v3, v4 float32) []byte {
	out := make([]byte, 29)
	out[0] = byte(market)
	copy(out[1:8], []byte(symbol))
	binary.LittleEndian.PutUint32(out[8:12], date)
	out[12] = category
	binary.LittleEndian.PutUint32(out[13:17], math.Float32bits(v1))
	binary.LittleEndian.PutUint32(out[17:21], math.Float32bits(v2))
	binary.LittleEndian.PutUint32(out[21:25], math.Float32bits(v3))
	binary.LittleEndian.PutUint32(out[25:29], math.Float32bits(v4))
	return out
}

func exDailyRecord(date uint32, open, high, low, close float32, amount uint32, settlement float32) []byte {
	out := make([]byte, 32)
	binary.LittleEndian.PutUint32(out[0:4], date)
	binary.LittleEndian.PutUint32(out[4:8], math.Float32bits(open))
	binary.LittleEndian.PutUint32(out[8:12], math.Float32bits(high))
	binary.LittleEndian.PutUint32(out[12:16], math.Float32bits(low))
	binary.LittleEndian.PutUint32(out[16:20], math.Float32bits(close))
	binary.LittleEndian.PutUint32(out[20:24], amount)
	binary.LittleEndian.PutUint32(out[24:28], 1000)
	binary.LittleEndian.PutUint32(out[28:32], math.Float32bits(settlement))
	return out
}

func fixedBytes(value []byte, size int) []byte {
	out := make([]byte, size)
	copy(out, value)
	return out
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
