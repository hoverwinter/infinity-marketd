package tdx

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// Dragon-Tiger (龙虎榜 / LHB) extraction from TDX F10 company information,
// ported from millken/tdx. This is not a binary packet — it locates the
// 资金动向 F10 section (via the existing company category/content reads) and
// parses its text. Live read; never writes ClickHouse.

const defaultLHBAlias = "资金动向"

// HQLHBSeat is one broker-branch seat in a Dragon-Tiger record (amounts 万元).
type HQLHBSeat struct {
	Name    string  `json:"name"`
	BuyAmt  float64 `json:"buy_amount"`
	SellAmt float64 `json:"sell_amount"`
}

// HQLHBRecord is one Dragon-Tiger list entry.
type HQLHBRecord struct {
	Date      string      `json:"date"`
	InfoType  string      `json:"info_type"`
	ChangePct float64     `json:"change_pct"`
	Volume    float64     `json:"volume_wan_shares"`
	Amount    float64     `json:"amount_wan_yuan"`
	BuySeats  []HQLHBSeat `json:"buy_seats"`
	SellSeats []HQLHBSeat `json:"sell_seats"`
}

// HQLHBResult wraps records plus metadata so a missing section is not an error.
type HQLHBResult struct {
	Market  string        `json:"market"`
	Symbol  string        `json:"symbol"`
	Found   bool          `json:"found"`
	Message string        `json:"message,omitempty"`
	Records []HQLHBRecord `json:"records"`
}

var (
	reLHBRecord  = regexp.MustCompile(`●交易日期:(\d{4}-\d{2}-\d{2})\s+信息类型:(.+)`)
	reLHBSummary = regexp.MustCompile(`涨跌幅\(%\):([\-\d.]+)\s+成交量\(万股\):([\d.]+)\s+成交额\(万元\):([\d.]+)`)
	reLHBSeat    = regexp.MustCompile(`│(.+)│\s*([\-\d.]+)│\s*([\-\d.]+)│`)
)

// ParseLHB extracts Dragon-Tiger records from 资金动向 section text.
func ParseLHB(text string) []HQLHBRecord {
	var records []HQLHBRecord
	var cur *HQLHBRecord
	var section string
	var pendingName string

	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")

		if m := reLHBRecord.FindStringSubmatch(line); m != nil {
			if cur != nil {
				records = append(records, *cur)
			}
			cur = &HQLHBRecord{Date: m[1], InfoType: strings.TrimSpace(m[2])}
			section = ""
			pendingName = ""
			continue
		}
		if cur == nil {
			continue
		}
		if m := reLHBSummary.FindStringSubmatch(line); m != nil {
			cur.ChangePct, _ = strconv.ParseFloat(m[1], 64)
			cur.Volume, _ = strconv.ParseFloat(m[2], 64)
			cur.Amount, _ = strconv.ParseFloat(m[3], 64)
			continue
		}
		if strings.Contains(line, "买入前五") {
			section = "buy"
			pendingName = ""
			continue
		}
		if strings.Contains(line, "卖出前五") {
			section = "sell"
			pendingName = ""
			continue
		}
		if strings.Contains(line, "营业部名称") || strings.Contains(line, "金额") ||
			strings.Contains(line, "────") || strings.Contains(line, "┌") ||
			strings.Contains(line, "└") || strings.Contains(line, "├") {
			continue
		}
		if !strings.Contains(line, "│") {
			continue
		}
		m := reLHBSeat.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name := strings.TrimSpace(m[1])
		buyAmt, _ := strconv.ParseFloat(strings.TrimSpace(m[2]), 64)
		sellAmt, _ := strconv.ParseFloat(strings.TrimSpace(m[3]), 64)
		if name == "" {
			if pendingName != "" && (buyAmt != 0 || sellAmt != 0) {
				cur.addSeat(section, HQLHBSeat{Name: strings.TrimSpace(pendingName), BuyAmt: buyAmt, SellAmt: sellAmt})
				pendingName = ""
			}
			continue
		}
		pendingName = ""
		if buyAmt != 0 || sellAmt != 0 {
			cur.addSeat(section, HQLHBSeat{Name: name, BuyAmt: buyAmt, SellAmt: sellAmt})
		} else {
			pendingName = name
		}
	}
	if cur != nil {
		records = append(records, *cur)
	}
	return records
}

func (r *HQLHBRecord) addSeat(section string, seat HQLHBSeat) {
	switch section {
	case "buy":
		r.BuySeats = append(r.BuySeats, seat)
	case "sell":
		r.SellSeats = append(r.SellSeats, seat)
	}
}

// findLHBSection returns the F10 category matching 资金动向 or a configured alias.
func findLHBSection(cats []HQCompanyInfoCategory, aliases []string) (HQCompanyInfoCategory, bool) {
	if len(aliases) == 0 {
		aliases = []string{defaultLHBAlias}
	}
	for _, cat := range cats {
		for _, alias := range aliases {
			if strings.TrimSpace(cat.Name) == strings.TrimSpace(alias) {
				return cat, true
			}
		}
	}
	return HQCompanyInfoCategory{}, false
}

// extractLHBContent trims the F10 content to the 交易龙虎榜 section when present.
func extractLHBContent(content string) string {
	if idx := strings.Index(content, "\n【1.交易龙虎榜】"); idx >= 0 {
		return content[idx+1:]
	}
	return content
}

// LHB fetches and parses the Dragon-Tiger list over an open session.
func (s *QuoteSession) LHB(req HQMinuteRequest, aliases []string) (HQLHBResult, error) {
	result := HQLHBResult{Market: req.Market, Symbol: req.Symbol, Records: []HQLHBRecord{}}
	cats, err := s.CompanyInfoCategories(req)
	if err != nil {
		return result, err
	}
	cat, ok := findLHBSection(cats, aliases)
	if !ok {
		result.Message = "资金动向 section not found"
		return result, nil
	}
	content, err := s.CompanyInfoContent(req, cat.Filename, cat.Start, cat.Length)
	if err != nil {
		return result, err
	}
	result.Found = true
	result.Records = ParseLHB(extractLHBContent(content.Content))
	return result, nil
}

// FetchHQLHB fetches Dragon-Tiger records with server fallback.
func FetchHQLHB(ctx context.Context, req HQMinuteRequest, aliases []string, opts QuoteClientOptions) (HQLHBResult, error) {
	return fetchHQRead(ctx, opts, "lhb", func(s *QuoteSession) (HQLHBResult, error) {
		return s.LHB(req, aliases)
	})
}
