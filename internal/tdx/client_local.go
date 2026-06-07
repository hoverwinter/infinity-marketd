package tdx

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type CapitalChangeParseResult struct {
	Events []model.CapitalChangeEvent
	Issues []ParseIssue
}

type BlockParseResult struct {
	Snapshot    model.TDXBlockSnapshot
	Definitions []model.TDXBlockDefinition
	Memberships []model.TDXBlockMembership
	Issues      []ParseIssue
}

type ExDailyParseResult struct {
	Bars   []model.ExDailyBar
	Issues []ParseIssue
	Format string
}

type CustomBlockEdit struct {
	BlockID string
	Add     []string
	Remove  []string
	Replace []string
}

func ParseGBBQBytes(raw []byte, path string, loc *time.Location) CapitalChangeParseResult {
	result := CapitalChangeParseResult{}
	if len(raw) < 4 {
		result.Issues = append(result.Issues, ParseIssue{Type: "file_too_short", Message: "gbbq file shorter than count header", LogicalKey: filepath.Base(path)})
		return result
	}
	count := int(binary.LittleEndian.Uint32(raw[:4]))
	pos := 4
	seen := make(map[string]model.CapitalChangeEvent)
	for i := 0; i < count; i++ {
		if pos+29 > len(raw) {
			o := uint64(pos)
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: "gbbq record truncated", Offset: &o})
			break
		}
		record := raw[pos : pos+29]
		pos += 29
		market, err := quoteMarketName(int(record[0]))
		if err != nil {
			o := uint64(pos - 29)
			result.Issues = append(result.Issues, ParseIssue{Type: "unsupported_market", Message: err.Error(), Offset: &o})
			continue
		}
		symbol := strings.TrimRight(string(record[1:8]), "\x00 ")
		if len(symbol) > 6 {
			symbol = symbol[len(symbol)-6:]
		}
		if len(symbol) != 6 || !allDigits(symbol) {
			o := uint64(pos - 29)
			result.Issues = append(result.Issues, ParseIssue{Type: "unsupported_symbol", Message: fmt.Sprintf("unsupported gbbq symbol %q", symbol), Offset: &o})
			continue
		}
		dateRaw := binary.LittleEndian.Uint32(record[8:12])
		eventDate, ok := validDate(int(dateRaw/10000), time.Month((dateRaw/100)%100), int(dateRaw%100), loc)
		if !ok {
			o := uint64(pos - 29)
			result.Issues = append(result.Issues, ParseIssue{Type: "invalid_date", Message: fmt.Sprintf("invalid date %d", dateRaw), Offset: &o})
			continue
		}
		category := record[12]
		values := [4]float64{
			float64(math.Float32frombits(binary.LittleEndian.Uint32(record[13:17]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(record[17:21]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(record[21:25]))),
			float64(math.Float32frombits(binary.LittleEndian.Uint32(record[25:29]))),
		}
		event := model.CapitalChangeEvent{
			Market:    market,
			Symbol:    symbol,
			EventDate: eventDate,
			Category:  category,
			EventSeq:  uint16(i),
			EventName: hqXDXRCategoryName(int(category)),
		}
		applyGBBQValues(&event, values)
		key := capitalChangeKey(event)
		if prev, exists := seen[key]; exists {
			o := uint64(pos - 29)
			issueType := "duplicate_logical_key"
			if !equalCapitalChange(prev, event) {
				issueType = "conflicting_logical_key"
			}
			result.Issues = append(result.Issues, ParseIssue{Type: issueType, Message: "duplicate gbbq logical key", Offset: &o, LogicalKey: key})
			continue
		}
		seen[key] = event
		result.Events = append(result.Events, event)
	}
	if pos < len(raw) {
		o := uint64(pos)
		result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: fmt.Sprintf("%d trailing bytes", len(raw)-pos), Offset: &o})
	}
	if len(result.Events) == 0 {
		result.Issues = append(result.Issues, ParseIssue{Type: "zero_valid_rows", Message: "no valid gbbq events", LogicalKey: filepath.Base(path)})
	}
	return result
}

func ParseSystemBlockBytes(raw []byte, path string, scope string, snapshotTime time.Time) BlockParseResult {
	kind := blockKindFromPath(path)
	scope = normalizeBlockScope(scope)
	result := BlockParseResult{}
	if len(raw) < 386 {
		result.Issues = append(result.Issues, ParseIssue{Type: "file_too_short", Message: "block file shorter than header", LogicalKey: filepath.Base(path)})
		return result
	}
	count := int(binary.LittleEndian.Uint16(raw[384:386]))
	pos := 386
	for i := 0; i < count; i++ {
		if pos+13 > len(raw) {
			o := uint64(pos)
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: "block definition truncated", Offset: &o})
			break
		}
		name := decodeTDXCString(raw[pos : pos+9])
		pos += 9
		memberCount := int(binary.LittleEndian.Uint16(raw[pos : pos+2]))
		blockType := binary.LittleEndian.Uint16(raw[pos+2 : pos+4])
		pos += 4
		blockID := fmt.Sprintf("%s:%04d:%s", kind, i, slugBlockName(name))
		def := model.TDXBlockDefinition{
			BlockScope:   scope,
			BlockKind:    kind,
			BlockID:      blockID,
			BlockName:    name,
			BlockType:    blockType,
			DisplayOrder: uint32(i),
			MemberCount:  uint32(memberCount),
		}
		result.Definitions = append(result.Definitions, def)
		memberBegin := pos
		for j := 0; j < memberCount; j++ {
			if pos+7 > len(raw) {
				o := uint64(pos)
				result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: "block member truncated", Offset: &o})
				break
			}
			code := strings.TrimRight(string(raw[pos:pos+7]), "\x00 ")
			pos += 7
			market, symbol := splitBlockCode(code)
			if symbol == "" {
				market = InferMarketFromCode(code)
				symbol = code
			}
			result.Memberships = append(result.Memberships, model.TDXBlockMembership{
				BlockScope:  scope,
				BlockID:     blockID,
				MemberOrder: uint32(j),
				Code:        code,
				Market:      market,
				Symbol:      symbol,
			})
		}
		pos = memberBegin + 2800
		if pos > len(raw) {
			o := uint64(len(raw))
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: "block fixed member area truncated", Offset: &o})
			break
		}
	}
	assignBlockSnapshot(&result, scope, snapshotTime)
	return result
}

func ParseCustomBlockDir(dir string, snapshotTime time.Time) (BlockParseResult, error) {
	dir = filepath.Clean(dir)
	cfgPath := filepath.Join(dir, "blocknew.cfg")
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		return BlockParseResult{}, err
	}
	result := BlockParseResult{}
	scope := "custom"
	for pos, order := 0, 0; pos < len(raw); pos, order = pos+120, order+1 {
		if pos+120 > len(raw) {
			o := uint64(pos)
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: "custom block cfg record truncated", Offset: &o})
			break
		}
		name := decodeTDXCString(raw[pos : pos+50])
		blockID := decodeTDXCString(raw[pos+50 : pos+120])
		if blockID == "" {
			continue
		}
		blkPath := filepath.Join(dir, blockID+".blk")
		linesRaw, err := os.ReadFile(blkPath)
		if err != nil {
			result.Issues = append(result.Issues, ParseIssue{Type: "missing_custom_block_member_file", Message: err.Error(), LogicalKey: blockID})
			continue
		}
		codes := parseCustomBlockLines(string(linesRaw))
		result.Definitions = append(result.Definitions, model.TDXBlockDefinition{
			BlockScope:   scope,
			BlockKind:    "custom",
			BlockID:      blockID,
			BlockName:    name,
			DisplayOrder: uint32(order),
			MemberCount:  uint32(len(codes)),
		})
		for i, code := range codes {
			market := InferMarketFromCode(code)
			result.Memberships = append(result.Memberships, model.TDXBlockMembership{
				BlockScope:  scope,
				BlockID:     blockID,
				MemberOrder: uint32(i),
				Code:        code,
				Market:      market,
				Symbol:      code,
			})
		}
	}
	assignBlockSnapshot(&result, scope, snapshotTime)
	return result, nil
}

func ParseExDailyBytes(raw []byte, path string, exMarket uint16, code string, loc *time.Location) ExDailyParseResult {
	result := ExDailyParseResult{Format: "tdx.ex_daily.<IffffIIf>"}
	seen := make(map[string]model.ExDailyBar)
	for offset := 0; offset < len(raw); offset += 32 {
		if len(raw)-offset < 32 {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "incomplete_trailing_bytes", Message: fmt.Sprintf("%d trailing bytes", len(raw)-offset), Offset: &o})
			break
		}
		chunk := raw[offset : offset+32]
		dateRaw := binary.LittleEndian.Uint32(chunk[0:4])
		tradeDate, ok := validDate(int(dateRaw/10000), time.Month((dateRaw/100)%100), int(dateRaw%100), loc)
		if !ok {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "invalid_date", Message: fmt.Sprintf("invalid date %d", dateRaw), Offset: &o})
			continue
		}
		open := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[4:8])))
		high := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[8:12])))
		low := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[12:16])))
		close := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[16:20])))
		if high < low {
			o := uint64(offset)
			result.Issues = append(result.Issues, ParseIssue{Type: "high_less_than_low", Message: "high is less than low", Offset: &o})
			continue
		}
		amountBits := binary.LittleEndian.Uint32(chunk[20:24])
		amount := float64(amountBits)
		hkAmount := float64(math.Float32frombits(amountBits))
		volume := int64(binary.LittleEndian.Uint32(chunk[24:28]))
		settlement := float64(math.Float32frombits(binary.LittleEndian.Uint32(chunk[28:32])))
		bar := model.ExDailyBar{
			ExMarket:        exMarket,
			Code:            strings.TrimSpace(code),
			TradeDate:       tradeDate,
			Open:            open,
			High:            high,
			Low:             low,
			Close:           close,
			Position:        volume,
			Trade:           volume,
			Amount:          floatPtr(amount),
			Price:           floatPtr(hkAmount),
			SettlementPrice: floatPtr(settlement),
		}
		key := exDailyKey(bar)
		if prev, exists := seen[key]; exists {
			o := uint64(offset)
			issueType := "duplicate_logical_key"
			if !equalExDaily(prev, bar) {
				issueType = "conflicting_logical_key"
			}
			result.Issues = append(result.Issues, ParseIssue{Type: issueType, Message: "duplicate ex_daily logical key", Offset: &o, LogicalKey: key})
			continue
		}
		seen[key] = bar
		result.Bars = append(result.Bars, bar)
	}
	if len(result.Bars) == 0 {
		result.Issues = append(result.Issues, ParseIssue{Type: "zero_valid_rows", Message: "no valid ex_daily bars", LogicalKey: filepath.Base(path)})
	}
	return result
}

func ApplyCustomBlockEdit(current BlockParseResult, edit CustomBlockEdit) (BlockParseResult, error) {
	blockID := strings.TrimSpace(edit.BlockID)
	if blockID == "" {
		return BlockParseResult{}, fmt.Errorf("block id is required")
	}
	blocks := make(map[string][]model.TDXBlockMembership)
	defs := make(map[string]model.TDXBlockDefinition)
	for _, def := range current.Definitions {
		defs[def.BlockID] = def
	}
	for _, member := range current.Memberships {
		blocks[member.BlockID] = append(blocks[member.BlockID], member)
	}
	if _, ok := defs[blockID]; !ok {
		return BlockParseResult{}, fmt.Errorf("custom block %q not found", blockID)
	}
	var desired []string
	if len(edit.Replace) > 0 {
		var err error
		desired, err = normalizeCustomEditSymbols(edit.Replace)
		if err != nil {
			return BlockParseResult{}, err
		}
	} else {
		for _, member := range blocks[blockID] {
			desired = append(desired, member.Symbol)
		}
		remove := make(map[string]bool)
		removeSymbols, err := normalizeCustomEditSymbols(edit.Remove)
		if err != nil {
			return BlockParseResult{}, err
		}
		for _, symbol := range removeSymbols {
			remove[symbol] = true
		}
		var kept []string
		for _, symbol := range desired {
			if !remove[symbol] {
				kept = append(kept, symbol)
			}
		}
		addSymbols, err := normalizeCustomEditSymbols(edit.Add)
		if err != nil {
			return BlockParseResult{}, err
		}
		desired = append(kept, addSymbols...)
	}
	desired = uniqueStrings(desired)
	if len(desired) == 0 {
		return BlockParseResult{}, fmt.Errorf("custom block %q would be empty", blockID)
	}
	out := current
	var memberships []model.TDXBlockMembership
	for _, member := range out.Memberships {
		if member.BlockID != blockID {
			memberships = append(memberships, member)
		}
	}
	for i, symbol := range desired {
		memberships = append(memberships, model.TDXBlockMembership{
			BlockScope:  "custom",
			BlockID:     blockID,
			MemberOrder: uint32(i),
			Code:        symbol,
			Market:      InferMarketFromCode(symbol),
			Symbol:      symbol,
		})
	}
	out.Memberships = memberships
	for i := range out.Definitions {
		if out.Definitions[i].BlockID == blockID {
			out.Definitions[i].MemberCount = uint32(len(desired))
		}
	}
	assignBlockSnapshot(&out, "custom", current.Snapshot.SnapshotTime)
	return out, nil
}

func WriteCustomBlockDir(dir string, parsed BlockParseResult) error {
	dir = filepath.Clean(dir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cfgPath := filepath.Join(dir, "blocknew.cfg")
	if _, err := os.Stat(cfgPath); err == nil {
		backup := cfgPath + ".bak"
		raw, readErr := os.ReadFile(cfgPath)
		if readErr != nil {
			return readErr
		}
		if err := os.WriteFile(backup, raw, 0o644); err != nil {
			return err
		}
	}
	cfg, files, err := encodeCustomBlockDir(parsed)
	if err != nil {
		return err
	}
	tmpDir := dir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "blocknew.cfg"), cfg, 0o644); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name+".blk"), content, 0o644); err != nil {
			return err
		}
	}
	for name := range files {
		if err := os.Rename(filepath.Join(tmpDir, name+".blk"), filepath.Join(dir, name+".blk")); err != nil {
			return err
		}
	}
	if err := os.Rename(filepath.Join(tmpDir, "blocknew.cfg"), cfgPath); err != nil {
		return err
	}
	_ = os.RemoveAll(tmpDir)
	verified, err := ParseCustomBlockDir(dir, parsed.Snapshot.SnapshotTime)
	if err != nil {
		return err
	}
	if verified.Snapshot.ContentHash != parsed.Snapshot.ContentHash {
		return fmt.Errorf("post-write validation failed")
	}
	return nil
}

func applyGBBQValues(event *model.CapitalChangeEvent, values [4]float64) {
	switch event.Category {
	case 1:
		event.CashDividend = floatPtr(values[0])
		event.AllotmentPrice = floatPtr(values[1])
		event.BonusShares = floatPtr(values[2])
		event.AllotmentShares = floatPtr(values[3])
	default:
		event.PreFloatShares = floatPtr(values[0])
		event.PreTotalShares = floatPtr(values[1])
		event.PostFloatShares = floatPtr(values[2])
		event.PostTotalShares = floatPtr(values[3])
	}
}

func assignBlockSnapshot(result *BlockParseResult, scope string, snapshotTime time.Time) {
	if snapshotTime.IsZero() {
		snapshotTime = time.Now()
	}
	hash := blockContentHash(result.Definitions, result.Memberships)
	for i := range result.Definitions {
		result.Definitions[i].SnapshotID = hash
		result.Definitions[i].BlockScope = scope
	}
	for i := range result.Memberships {
		result.Memberships[i].SnapshotID = hash
		result.Memberships[i].BlockScope = scope
	}
	result.Snapshot = model.TDXBlockSnapshot{
		SnapshotID:   hash,
		BlockScope:   scope,
		SnapshotTime: snapshotTime,
		ContentHash:  hash,
		BlockCount:   uint32(len(result.Definitions)),
		MemberCount:  uint32(len(result.Memberships)),
	}
}

func blockContentHash(defs []model.TDXBlockDefinition, members []model.TDXBlockMembership) string {
	var lines []string
	for _, def := range defs {
		lines = append(lines, fmt.Sprintf("D|%s|%s|%s|%d|%d", def.BlockScope, def.BlockKind, def.BlockID, def.BlockType, def.MemberCount))
	}
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("M|%s|%s|%d|%s|%s|%s", member.BlockScope, member.BlockID, member.MemberOrder, member.Code, member.Market, member.Symbol))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func blockKindFromPath(path string) string {
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if stem == "" {
		return "block"
	}
	return stem
}

func normalizeBlockScope(scope string) string {
	scope = strings.ToLower(strings.TrimSpace(scope))
	if scope == "custom" {
		return "custom"
	}
	return "system"
}

func slugBlockName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "unnamed"
	}
	return strings.ReplaceAll(name, " ", "_")
}

func parseCustomBlockLines(content string) []string {
	var out []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 6 {
			line = line[len(line)-6:]
		}
		if len(line) == 6 && allDigits(line) {
			out = append(out, line)
		}
	}
	return uniqueStrings(out)
}

func normalizeCustomEditSymbols(values []string) ([]string, error) {
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, symbol, err := ParseMarketSymbol("", "", value)
		if err != nil {
			if strings.Contains(value, ":") {
				parts := strings.Split(value, ":")
				if len(parts) == 2 {
					_, symbol, err = ParseMarketSymbol("", parts[0], parts[1])
				}
			}
		}
		if err == nil {
			out = append(out, symbol)
			continue
		}
		return nil, err
	}
	return uniqueStrings(out), nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func encodeCustomBlockDir(parsed BlockParseResult) ([]byte, map[string][]byte, error) {
	defs := append([]model.TDXBlockDefinition(nil), parsed.Definitions...)
	sort.Slice(defs, func(i, j int) bool { return defs[i].DisplayOrder < defs[j].DisplayOrder })
	files := make(map[string][]byte)
	var cfg []byte
	for _, def := range defs {
		if def.BlockID == "" {
			return nil, nil, fmt.Errorf("custom block id is required")
		}
		nameBytes, err := encodeTDXFixed(def.BlockName, 50)
		if err != nil {
			return nil, nil, err
		}
		idBytes, err := encodeTDXFixed(def.BlockID, 70)
		if err != nil {
			return nil, nil, err
		}
		cfg = append(cfg, nameBytes...)
		cfg = append(cfg, idBytes...)
		var lines []string
		for _, member := range parsed.Memberships {
			if member.BlockID == def.BlockID {
				prefix := "1"
				if member.Market == "sz" {
					prefix = "0"
				}
				lines = append(lines, prefix+member.Symbol)
			}
		}
		files[def.BlockID] = []byte(strings.Join(lines, "\n") + "\n")
	}
	return cfg, files, nil
}

func encodeTDXFixed(value string, size int) ([]byte, error) {
	encoded, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(value))
	if err != nil {
		return nil, err
	}
	if len(encoded) > size {
		return nil, fmt.Errorf("value %q exceeds %d bytes", value, size)
	}
	out := make([]byte, size)
	copy(out, encoded)
	return out, nil
}

func capitalChangeKey(event model.CapitalChangeEvent) string {
	return fmt.Sprintf("%s:%s:%s:%d:%d", event.Market, event.Symbol, event.EventDate.Format("2006-01-02"), event.Category, event.EventSeq)
}

func exDailyKey(bar model.ExDailyBar) string {
	return fmt.Sprintf("%d:%s:%s", bar.ExMarket, bar.Code, bar.TradeDate.Format("2006-01-02"))
}

func equalCapitalChange(a, b model.CapitalChangeEvent) bool {
	return capitalChangeKey(a) == capitalChangeKey(b) &&
		ptrEqual(a.CashDividend, b.CashDividend) &&
		ptrEqual(a.AllotmentPrice, b.AllotmentPrice) &&
		ptrEqual(a.BonusShares, b.BonusShares) &&
		ptrEqual(a.AllotmentShares, b.AllotmentShares) &&
		ptrEqual(a.PreFloatShares, b.PreFloatShares) &&
		ptrEqual(a.PostFloatShares, b.PostFloatShares) &&
		ptrEqual(a.PreTotalShares, b.PreTotalShares) &&
		ptrEqual(a.PostTotalShares, b.PostTotalShares)
}

func equalExDaily(a, b model.ExDailyBar) bool {
	return exDailyKey(a) == exDailyKey(b) &&
		a.Open == b.Open && a.High == b.High && a.Low == b.Low && a.Close == b.Close &&
		a.Position == b.Position && a.Trade == b.Trade &&
		ptrEqual(a.Price, b.Price) && ptrEqual(a.Amount, b.Amount) && ptrEqual(a.SettlementPrice, b.SettlementPrice)
}

func ptrEqual(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func ParseExMarket(value string) (uint16, error) {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || n < 0 || n > math.MaxUint16 {
		return 0, fmt.Errorf("invalid extension market %q", value)
	}
	return uint16(n), nil
}
