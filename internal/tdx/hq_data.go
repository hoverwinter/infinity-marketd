package tdx

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	HQKLine5Min    = 0
	HQKLine15Min   = 1
	HQKLine30Min   = 2
	HQKLine1Hour   = 3
	HQKLineDaily   = 4
	HQKLineWeekly  = 5
	HQKLineMonthly = 6
	HQKLine1MinAlt = 7
	HQKLine1Min    = 8
	HQKLineDayAlt  = 9
	HQKLineQuarter = 10
	HQKLineYearly  = 11

	DefaultHQKLineCount       = 100
	MaxHQKLineCount           = 800
	DefaultHQTransactionCount = 1000
	MaxHQTransactionCount     = 1800
	DefaultHQBlockChunkSize   = 0x7530
)

type HQBarsRequest struct {
	Category int    `json:"category"`
	Market   string `json:"market"`
	Symbol   string `json:"symbol"`
	Start    int    `json:"start"`
	Count    int    `json:"count"`
}

type HQBar struct {
	Market    string  `json:"market"`
	Symbol    string  `json:"symbol"`
	Category  int     `json:"category"`
	DateTime  string  `json:"datetime"`
	Year      int     `json:"year"`
	Month     int     `json:"month"`
	Day       int     `json:"day"`
	Hour      int     `json:"hour"`
	Minute    int     `json:"minute"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Amount    float64 `json:"amount"`
	UpCount   uint16  `json:"up_count,omitempty"`
	DownCount uint16  `json:"down_count,omitempty"`
}

type HQMinuteRequest struct {
	Market string `json:"market"`
	Symbol string `json:"symbol"`
}

type HQMinutePoint struct {
	Market string  `json:"market"`
	Symbol string  `json:"symbol"`
	Date   string  `json:"date,omitempty"`
	Time   string  `json:"time"`
	Index  int     `json:"index"`
	Price  float64 `json:"price"`
	Volume int64   `json:"volume"`
}

type HQTransaction struct {
	Market    string  `json:"market"`
	Symbol    string  `json:"symbol"`
	Date      string  `json:"date,omitempty"`
	Time      string  `json:"time"`
	Hour      int     `json:"hour"`
	Minute    int     `json:"minute"`
	Price     float64 `json:"price"`
	Volume    int64   `json:"volume"`
	Num       int64   `json:"num,omitempty"`
	BuyOrSell int64   `json:"buy_or_sell"`
}

type HQCompanyInfoCategory struct {
	Market   string `json:"market"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Start    uint32 `json:"start"`
	Length   uint32 `json:"length"`
}

type HQCompanyInfoContent struct {
	Market   string `json:"market"`
	Symbol   string `json:"symbol"`
	Filename string `json:"filename"`
	Start    uint32 `json:"start"`
	Length   uint32 `json:"length"`
	Content  string `json:"content"`
}

type HQXDXRInfo struct {
	Market         string   `json:"market"`
	Symbol         string   `json:"symbol"`
	Year           int      `json:"year"`
	Month          int      `json:"month"`
	Day            int      `json:"day"`
	Date           string   `json:"date"`
	Category       int      `json:"category"`
	Name           string   `json:"name"`
	FenHong        *float64 `json:"fenhong,omitempty"`
	PeiGuJia       *float64 `json:"peigujia,omitempty"`
	SongZhuanGu    *float64 `json:"songzhuangu,omitempty"`
	PeiGu          *float64 `json:"peigu,omitempty"`
	SuoGu          *float64 `json:"suogu,omitempty"`
	PanQianLiuTong *float64 `json:"panqianliutong,omitempty"`
	PanHouLiuTong  *float64 `json:"panhouliutong,omitempty"`
	QianZongGuBen  *float64 `json:"qianzongguben,omitempty"`
	HouZongGuBen   *float64 `json:"houzongguben,omitempty"`
	FenShu         *float64 `json:"fenshu,omitempty"`
	XingQuanJia    *float64 `json:"xingquanjia,omitempty"`
}

type HQFinanceInfo struct {
	Market            string  `json:"market"`
	Symbol            string  `json:"symbol"`
	LiuTongGuBen      float64 `json:"liutongguben"`
	Province          uint16  `json:"province"`
	Industry          uint16  `json:"industry"`
	UpdatedDate       uint32  `json:"updated_date"`
	IPODate           uint32  `json:"ipo_date"`
	ZongGuBen         float64 `json:"zongguben"`
	GuoJiaGu          float64 `json:"guojiagu"`
	FaQiRenFaRenGu    float64 `json:"faqirenfarengu"`
	FaRenGu           float64 `json:"farengu"`
	BGu               float64 `json:"bgu"`
	HGu               float64 `json:"hgu"`
	ZhiGongGu         float64 `json:"zhigonggu"`
	ZongZiChan        float64 `json:"zongzichan"`
	LiuDongZiChan     float64 `json:"liudongzichan"`
	GuDingZiChan      float64 `json:"gudingzichan"`
	WuXingZiChan      float64 `json:"wuxingzichan"`
	GuDongRenShu      float64 `json:"gudongrenshu"`
	LiuDongFuZhai     float64 `json:"liudongfuzhai"`
	ChangQiFuZhai     float64 `json:"changqifuzhai"`
	ZiBenGongJiJin    float64 `json:"zibengongjijin"`
	JingZiChan        float64 `json:"jingzichan"`
	ZhuYingShouRu     float64 `json:"zhuyingshouru"`
	ZhuYingLiRun      float64 `json:"zhuyinglirun"`
	YingShouZhangKuan float64 `json:"yingshouzhangkuan"`
	YingYeLiRun       float64 `json:"yingyelirun"`
	TouZiShouYi       float64 `json:"touzishouyi"`
	JingYingXianJin   float64 `json:"jingyingxianjinliu"`
	ZongXianJin       float64 `json:"zongxianjinliu"`
	CunHuo            float64 `json:"cunhuo"`
	LiRunZongE        float64 `json:"lirunzonge"`
	ShuiHouLiRun      float64 `json:"shuihoulirun"`
	JingLiRun         float64 `json:"jinglirun"`
	WeiFenPeiLiRun    float64 `json:"weifenpeilirun"`
	MeiGuJingZiChan   float64 `json:"meigujingzichan"`
	BaoLiu2           float64 `json:"baoliu2"`
}

type HQBlockMeta struct {
	File       string `json:"file"`
	Size       uint32 `json:"size"`
	HashBase64 string `json:"hash_base64"`
}

type HQBlockMember struct {
	BlockName string `json:"blockname"`
	BlockType uint16 `json:"block_type"`
	CodeIndex int    `json:"code_index"`
	Code      string `json:"code"`
	Market    string `json:"market,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
}

type HQBlockChunk struct {
	File          string `json:"file"`
	Start         uint32 `json:"start"`
	Size          uint32 `json:"size"`
	ContentBase64 string `json:"content_base64"`
}

func ParseHQBarsRequest(category int, market, symbol string, start, count int) (HQBarsRequest, error) {
	market, symbol, err := parseHQMarketSymbol(market, symbol, "K-line")
	if err != nil {
		return HQBarsRequest{}, err
	}
	if category < HQKLine5Min || category > HQKLineYearly {
		return HQBarsRequest{}, fmt.Errorf("unsupported standard HQ K-line category %d", category)
	}
	if start < 0 {
		return HQBarsRequest{}, fmt.Errorf("standard HQ K-line start must be non-negative")
	}
	if count <= 0 || count > MaxHQKLineCount {
		return HQBarsRequest{}, fmt.Errorf("standard HQ K-line count must be between 1 and %d", MaxHQKLineCount)
	}
	return HQBarsRequest{Category: category, Market: market, Symbol: symbol, Start: start, Count: count}, nil
}

func ParseHQMinuteRequest(market, symbol string) (HQMinuteRequest, error) {
	market, symbol, err := parseHQMarketSymbol(market, symbol, "minute-time")
	if err != nil {
		return HQMinuteRequest{}, err
	}
	return HQMinuteRequest{Market: market, Symbol: symbol}, nil
}

func ParseHQTransactionRequest(market, symbol string, start, count int) (HQMinuteRequest, error) {
	req, err := ParseHQMinuteRequest(market, symbol)
	if err != nil {
		return HQMinuteRequest{}, err
	}
	if err := validateHQWindow(start, count, MaxHQTransactionCount, "transaction"); err != nil {
		return HQMinuteRequest{}, err
	}
	return req, nil
}

func (s *QuoteSession) SecurityBars(req HQBarsRequest) ([]HQBar, error) {
	normalized, err := ParseHQBarsRequest(req.Category, req.Market, req.Symbol, req.Start, req.Count)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQSecurityBarsPacket(normalized))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ security K-line request %s: %w", s.server, err)
	}
	return DecodeHQBarsResponse(normalized, false, body)
}

func (s *QuoteSession) IndexBars(req HQBarsRequest) ([]HQBar, error) {
	normalized, err := ParseHQBarsRequest(req.Category, req.Market, req.Symbol, req.Start, req.Count)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQIndexBarsPacket(normalized))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ index K-line request %s: %w", s.server, err)
	}
	return DecodeHQBarsResponse(normalized, true, body)
}

func (s *QuoteSession) MinuteTime(req HQMinuteRequest) ([]HQMinutePoint, error) {
	normalized, err := ParseHQMinuteRequest(req.Market, req.Symbol)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQMinuteTimePacket(normalized))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ minute-time request %s: %w", s.server, err)
	}
	return DecodeHQMinuteTimeResponse(normalized, 0, body)
}

func (s *QuoteSession) HistoryMinuteTime(req HQMinuteRequest, date int) ([]HQMinutePoint, error) {
	normalized, err := ParseHQMinuteRequest(req.Market, req.Symbol)
	if err != nil {
		return nil, err
	}
	if err := validateHQDate(date); err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQHistoryMinuteTimePacket(normalized, date))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ history minute-time request %s: %w", s.server, err)
	}
	return DecodeHQHistoryMinuteTimeResponse(normalized, date, body)
}

func (s *QuoteSession) Transactions(req HQMinuteRequest, start, count int) ([]HQTransaction, error) {
	normalized, err := ParseHQTransactionRequest(req.Market, req.Symbol, start, count)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQTransactionPacket(normalized, start, count))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ transaction request %s: %w", s.server, err)
	}
	return DecodeHQTransactionResponse(normalized, 0, body)
}

func (s *QuoteSession) HistoryTransactions(req HQMinuteRequest, date, start, count int) ([]HQTransaction, error) {
	normalized, err := ParseHQTransactionRequest(req.Market, req.Symbol, start, count)
	if err != nil {
		return nil, err
	}
	if err := validateHQDate(date); err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQHistoryTransactionPacket(normalized, date, start, count))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ history transaction request %s: %w", s.server, err)
	}
	return DecodeHQHistoryTransactionResponse(normalized, date, body)
}

func (s *QuoteSession) CompanyInfoCategories(req HQMinuteRequest) ([]HQCompanyInfoCategory, error) {
	normalized, err := ParseHQMinuteRequest(req.Market, req.Symbol)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQCompanyInfoCategoryPacket(normalized))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ company info category request %s: %w", s.server, err)
	}
	return DecodeHQCompanyInfoCategoryResponse(normalized, body)
}

func (s *QuoteSession) CompanyInfoContent(req HQMinuteRequest, filename string, start, length uint32) (HQCompanyInfoContent, error) {
	normalized, err := ParseHQMinuteRequest(req.Market, req.Symbol)
	if err != nil {
		return HQCompanyInfoContent{}, err
	}
	packet, err := BuildHQCompanyInfoContentPacket(normalized, filename, start, length)
	if err != nil {
		return HQCompanyInfoContent{}, err
	}
	body, err := s.call(packet)
	if err != nil {
		return HQCompanyInfoContent{}, fmt.Errorf("TDX HQ company info content request %s: %w", s.server, err)
	}
	content, err := DecodeHQCompanyInfoContentResponse(body)
	if err != nil {
		return HQCompanyInfoContent{}, err
	}
	return HQCompanyInfoContent{Market: normalized.Market, Symbol: normalized.Symbol, Filename: strings.TrimSpace(filename), Start: start, Length: length, Content: content}, nil
}

func (s *QuoteSession) XDXRInfo(req HQMinuteRequest) ([]HQXDXRInfo, error) {
	normalized, err := ParseHQMinuteRequest(req.Market, req.Symbol)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildHQXDXRPacket(normalized))
	if err != nil {
		return nil, fmt.Errorf("TDX HQ xdxr request %s: %w", s.server, err)
	}
	return DecodeHQXDXRResponse(normalized, body)
}

func (s *QuoteSession) FinanceInfo(req HQMinuteRequest) (HQFinanceInfo, error) {
	normalized, err := ParseHQMinuteRequest(req.Market, req.Symbol)
	if err != nil {
		return HQFinanceInfo{}, err
	}
	body, err := s.call(BuildHQFinanceInfoPacket(normalized))
	if err != nil {
		return HQFinanceInfo{}, fmt.Errorf("TDX HQ finance info request %s: %w", s.server, err)
	}
	return DecodeHQFinanceInfoResponse(normalized, body)
}

func (s *QuoteSession) BlockMeta(file string) (HQBlockMeta, error) {
	body, err := s.call(BuildHQBlockMetaPacket(file))
	if err != nil {
		return HQBlockMeta{}, fmt.Errorf("TDX HQ block meta request %s: %w", s.server, err)
	}
	return DecodeHQBlockMetaResponse(file, body)
}

func (s *QuoteSession) BlockChunk(file string, start, size uint32) (HQBlockChunk, error) {
	body, err := s.call(BuildHQBlockInfoPacket(file, start, size))
	if err != nil {
		return HQBlockChunk{}, fmt.Errorf("TDX HQ block content request %s: %w", s.server, err)
	}
	chunk, err := DecodeHQBlockInfoResponse(body)
	if err != nil {
		return HQBlockChunk{}, err
	}
	return HQBlockChunk{File: strings.TrimSpace(file), Start: start, Size: uint32(len(chunk)), ContentBase64: base64.StdEncoding.EncodeToString(chunk)}, nil
}

func (s *QuoteSession) BlockMembers(file string) ([]HQBlockMember, error) {
	meta, err := s.BlockMeta(file)
	if err != nil {
		return nil, err
	}
	var content []byte
	for start := uint32(0); start < meta.Size; start += DefaultHQBlockChunkSize {
		size := uint32(DefaultHQBlockChunkSize)
		if remain := meta.Size - start; remain < size {
			size = remain
		}
		body, err := s.call(BuildHQBlockInfoPacket(file, start, size))
		if err != nil {
			return nil, fmt.Errorf("TDX HQ block content request %s: %w", s.server, err)
		}
		chunk, err := DecodeHQBlockInfoResponse(body)
		if err != nil {
			return nil, err
		}
		content = append(content, chunk...)
		if uint32(len(chunk)) < size {
			break
		}
	}
	return DecodeHQBlockMembers(file, content)
}

func FetchHQSecurityBars(ctx context.Context, req HQBarsRequest, opts QuoteClientOptions) ([]HQBar, error) {
	return fetchHQRead(ctx, opts, "security K-line", func(session *QuoteSession) ([]HQBar, error) {
		return session.SecurityBars(req)
	})
}

func FetchHQIndexBars(ctx context.Context, req HQBarsRequest, opts QuoteClientOptions) ([]HQBar, error) {
	return fetchHQRead(ctx, opts, "index K-line", func(session *QuoteSession) ([]HQBar, error) {
		return session.IndexBars(req)
	})
}

func FetchHQMinuteTime(ctx context.Context, req HQMinuteRequest, opts QuoteClientOptions) ([]HQMinutePoint, error) {
	return fetchHQRead(ctx, opts, "minute-time", func(session *QuoteSession) ([]HQMinutePoint, error) {
		return session.MinuteTime(req)
	})
}

func FetchHQHistoryMinuteTime(ctx context.Context, req HQMinuteRequest, date int, opts QuoteClientOptions) ([]HQMinutePoint, error) {
	return fetchHQRead(ctx, opts, "history minute-time", func(session *QuoteSession) ([]HQMinutePoint, error) {
		return session.HistoryMinuteTime(req, date)
	})
}

func FetchHQTransactions(ctx context.Context, req HQMinuteRequest, start, count int, opts QuoteClientOptions) ([]HQTransaction, error) {
	return fetchHQRead(ctx, opts, "transaction", func(session *QuoteSession) ([]HQTransaction, error) {
		return session.Transactions(req, start, count)
	})
}

func FetchHQHistoryTransactions(ctx context.Context, req HQMinuteRequest, date, start, count int, opts QuoteClientOptions) ([]HQTransaction, error) {
	return fetchHQRead(ctx, opts, "history transaction", func(session *QuoteSession) ([]HQTransaction, error) {
		return session.HistoryTransactions(req, date, start, count)
	})
}

func FetchHQCompanyInfoCategories(ctx context.Context, req HQMinuteRequest, opts QuoteClientOptions) ([]HQCompanyInfoCategory, error) {
	return fetchHQRead(ctx, opts, "company info category", func(session *QuoteSession) ([]HQCompanyInfoCategory, error) {
		return session.CompanyInfoCategories(req)
	})
}

func FetchHQCompanyInfoContent(ctx context.Context, req HQMinuteRequest, filename string, start, length uint32, opts QuoteClientOptions) (HQCompanyInfoContent, error) {
	return fetchHQRead(ctx, opts, "company info content", func(session *QuoteSession) (HQCompanyInfoContent, error) {
		return session.CompanyInfoContent(req, filename, start, length)
	})
}

func FetchHQXDXRInfo(ctx context.Context, req HQMinuteRequest, opts QuoteClientOptions) ([]HQXDXRInfo, error) {
	return fetchHQRead(ctx, opts, "xdxr", func(session *QuoteSession) ([]HQXDXRInfo, error) {
		return session.XDXRInfo(req)
	})
}

func FetchHQFinanceInfo(ctx context.Context, req HQMinuteRequest, opts QuoteClientOptions) (HQFinanceInfo, error) {
	return fetchHQRead(ctx, opts, "finance info", func(session *QuoteSession) (HQFinanceInfo, error) {
		return session.FinanceInfo(req)
	})
}

func FetchHQBlockMeta(ctx context.Context, file string, opts QuoteClientOptions) (HQBlockMeta, error) {
	return fetchHQRead(ctx, opts, "block meta", func(session *QuoteSession) (HQBlockMeta, error) {
		return session.BlockMeta(file)
	})
}

func FetchHQBlockChunk(ctx context.Context, file string, start, size uint32, opts QuoteClientOptions) (HQBlockChunk, error) {
	return fetchHQRead(ctx, opts, "block content", func(session *QuoteSession) (HQBlockChunk, error) {
		return session.BlockChunk(file, start, size)
	})
}

func FetchHQBlockMembers(ctx context.Context, file string, opts QuoteClientOptions) ([]HQBlockMember, error) {
	return fetchHQRead(ctx, opts, "block members", func(session *QuoteSession) ([]HQBlockMember, error) {
		return session.BlockMembers(file)
	})
}

func fetchHQRead[T any](ctx context.Context, opts QuoteClientOptions, label string, fn func(*QuoteSession) (T, error)) (T, error) {
	var zero T
	var attempts []string
	for _, server := range NormalizeHQServers(opts) {
		session, err := OpenQuoteSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		value, fetchErr := fn(session)
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			if strings.Contains(fetchErr.Error(), "decode TDX HQ") {
				return zero, fetchErr
			}
			continue
		}
		return value, nil
	}
	return zero, fmt.Errorf("TDX HQ %s failed on %d server(s): %s", label, len(attempts), strings.Join(attempts, "; "))
}

func BuildHQSecurityBarsPacket(req HQBarsRequest) []byte {
	return buildHQBarsPacket(req)
}

func BuildHQIndexBarsPacket(req HQBarsRequest) []byte {
	return buildHQBarsPacket(req)
}

func BuildHQMinuteTimePacket(req HQMinuteRequest) []byte {
	packet := []byte{0x0c, 0x1b, 0x08, 0x00, 0x01, 0x01, 0x0e, 0x00, 0x0e, 0x00, 0x1d, 0x05}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	packet = binary.LittleEndian.AppendUint32(packet, 0)
	return packet
}

func BuildHQHistoryMinuteTimePacket(req HQMinuteRequest, date int) []byte {
	packet := []byte{0x0c, 0x01, 0x30, 0x00, 0x01, 0x01, 0x0d, 0x00, 0x0d, 0x00, 0xb4, 0x0f}
	packet = binary.LittleEndian.AppendUint32(packet, uint32(date))
	packet = append(packet, byte(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	return packet
}

func BuildHQTransactionPacket(req HQMinuteRequest, start, count int) []byte {
	packet := []byte{0x0c, 0x17, 0x08, 0x01, 0x01, 0x01, 0x0e, 0x00, 0x0e, 0x00, 0xc5, 0x0f}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	packet = binary.LittleEndian.AppendUint16(packet, uint16(start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(count))
	return packet
}

func BuildHQHistoryTransactionPacket(req HQMinuteRequest, date, start, count int) []byte {
	packet := []byte{0x0c, 0x01, 0x30, 0x01, 0x00, 0x01, 0x12, 0x00, 0x12, 0x00, 0xb5, 0x0f}
	packet = binary.LittleEndian.AppendUint32(packet, uint32(date))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	packet = binary.LittleEndian.AppendUint16(packet, uint16(start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(count))
	return packet
}

func BuildHQCompanyInfoCategoryPacket(req HQMinuteRequest) []byte {
	packet := []byte{0x0c, 0x0f, 0x10, 0x9b, 0x00, 0x01, 0x0e, 0x00, 0x0e, 0x00, 0xcf, 0x02}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	packet = binary.LittleEndian.AppendUint32(packet, 0)
	return packet
}

func BuildHQCompanyInfoContentPacket(req HQMinuteRequest, filename string, start, length uint32) ([]byte, error) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return nil, fmt.Errorf("company info filename is required")
	}
	if length == 0 {
		return nil, fmt.Errorf("company info content length must be positive")
	}
	packet := []byte{0x0c, 0x07, 0x10, 0x9c, 0x00, 0x01, 0x68, 0x00, 0x68, 0x00, 0xd0, 0x02}
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	packet = binary.LittleEndian.AppendUint16(packet, 0)
	packet = appendFixedASCII(packet, filename, 80)
	packet = binary.LittleEndian.AppendUint32(packet, start)
	packet = binary.LittleEndian.AppendUint32(packet, length)
	packet = binary.LittleEndian.AppendUint32(packet, 0)
	return packet, nil
}

func BuildHQXDXRPacket(req HQMinuteRequest) []byte {
	packet := []byte{0x0c, 0x1f, 0x18, 0x76, 0x00, 0x01, 0x0b, 0x00, 0x0b, 0x00, 0x0f, 0x00, 0x01, 0x00}
	packet = append(packet, byte(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	return packet
}

func BuildHQFinanceInfoPacket(req HQMinuteRequest) []byte {
	packet := []byte{0x0c, 0x1f, 0x18, 0x76, 0x00, 0x01, 0x0b, 0x00, 0x0b, 0x00, 0x10, 0x00, 0x01, 0x00}
	packet = append(packet, byte(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	return packet
}

func BuildHQBlockMetaPacket(file string) []byte {
	packet := []byte{0x0c, 0x39, 0x18, 0x69, 0x00, 0x01, 0x2a, 0x00, 0x2a, 0x00, 0xc5, 0x02}
	return appendFixedASCII(packet, strings.TrimSpace(file), 0x2a-2)
}

func BuildHQBlockInfoPacket(file string, start, size uint32) []byte {
	packet := []byte{0x0c, 0x37, 0x18, 0x6a, 0x00, 0x01, 0x6e, 0x00, 0x6e, 0x00, 0xb9, 0x06}
	packet = binary.LittleEndian.AppendUint32(packet, start)
	packet = binary.LittleEndian.AppendUint32(packet, size)
	return appendFixedASCII(packet, strings.TrimSpace(file), 0x6e-10)
}

func DecodeHQBarsResponse(req HQBarsRequest, index bool, body []byte) ([]HQBar, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("TDX HQ K-line response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	bars := make([]HQBar, 0, count)
	preDiffBase := 0
	for i := 0; i < count; i++ {
		year, month, day, hour, minute, next, err := decodeHQDateTime(req.Category, body, pos)
		if err != nil {
			return nil, fmt.Errorf("TDX HQ K-line response truncated at item %d datetime: %w", i, err)
		}
		pos = next
		openDiff, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		closeDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		highDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		lowDiff, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		if pos+8 > len(body) {
			return nil, fmt.Errorf("TDX HQ K-line response truncated at item %d volume", i)
		}
		volume := decodeTDXFloat(binary.LittleEndian.Uint32(body[pos : pos+4]))
		pos += 4
		amount := decodeTDXFloat(binary.LittleEndian.Uint32(body[pos : pos+4]))
		pos += 4

		var upCount, downCount uint16
		if index {
			if pos+4 > len(body) {
				return nil, fmt.Errorf("TDX HQ index K-line response truncated at item %d up/down", i)
			}
			upCount = binary.LittleEndian.Uint16(body[pos : pos+2])
			downCount = binary.LittleEndian.Uint16(body[pos+2 : pos+4])
			pos += 4
		}

		open := float64(openDiff+preDiffBase) / 1000.0
		openBase := openDiff + preDiffBase
		bar := HQBar{
			Market:    req.Market,
			Symbol:    req.Symbol,
			Category:  req.Category,
			DateTime:  fmt.Sprintf("%04d-%02d-%02d %02d:%02d", year, month, day, hour, minute),
			Year:      year,
			Month:     month,
			Day:       day,
			Hour:      hour,
			Minute:    minute,
			Open:      open,
			Close:     float64(openBase+closeDiff) / 1000.0,
			High:      float64(openBase+highDiff) / 1000.0,
			Low:       float64(openBase+lowDiff) / 1000.0,
			Volume:    volume,
			Amount:    amount,
			UpCount:   upCount,
			DownCount: downCount,
		}
		preDiffBase = openBase + closeDiff
		bars = append(bars, bar)
	}
	return bars, nil
}

func DecodeHQMinuteTimeResponse(req HQMinuteRequest, date int, body []byte) ([]HQMinutePoint, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("TDX HQ minute-time response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	return decodeHQMinutePoints(req, date, body, 4, count)
}

func DecodeHQHistoryMinuteTimeResponse(req HQMinuteRequest, date int, body []byte) ([]HQMinutePoint, error) {
	if len(body) == 0 {
		return []HQMinutePoint{}, nil
	}
	if len(body) < 6 {
		return nil, fmt.Errorf("TDX HQ history minute-time response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	return decodeHQMinutePoints(req, date, body, 6, count)
}

func DecodeHQTransactionResponse(req HQMinuteRequest, date int, body []byte) ([]HQTransaction, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("TDX HQ transaction response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	return decodeHQTransactions(req, date, body, 2, count, true)
}

func DecodeHQHistoryTransactionResponse(req HQMinuteRequest, date int, body []byte) ([]HQTransaction, error) {
	if len(body) == 0 {
		return []HQTransaction{}, nil
	}
	if len(body) < 6 {
		return nil, fmt.Errorf("TDX HQ history transaction response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	return decodeHQTransactions(req, date, body, 6, count, false)
}

func DecodeHQCompanyInfoCategoryResponse(req HQMinuteRequest, body []byte) ([]HQCompanyInfoCategory, error) {
	if len(body) < 2 {
		return nil, fmt.Errorf("TDX HQ company info category response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[:2]))
	pos := 2
	categories := make([]HQCompanyInfoCategory, 0, count)
	for i := 0; i < count; i++ {
		if pos+152 > len(body) {
			return nil, fmt.Errorf("TDX HQ company info category response truncated at item %d", i)
		}
		record := body[pos : pos+152]
		categories = append(categories, HQCompanyInfoCategory{
			Market:   req.Market,
			Symbol:   req.Symbol,
			Name:     decodeTDXCString(record[:64]),
			Filename: decodeTDXCString(record[64:144]),
			Start:    binary.LittleEndian.Uint32(record[144:148]),
			Length:   binary.LittleEndian.Uint32(record[148:152]),
		})
		pos += 152
	}
	return categories, nil
}

func DecodeHQCompanyInfoContentResponse(body []byte) (string, error) {
	if len(body) < 12 {
		return "", fmt.Errorf("TDX HQ company info content response too short: %d bytes", len(body))
	}
	length := int(binary.LittleEndian.Uint16(body[10:12]))
	if 12+length > len(body) {
		return "", fmt.Errorf("TDX HQ company info content response truncated: need %d have %d", 12+length, len(body))
	}
	return decodeTDXText(body[12 : 12+length]), nil
}

func DecodeHQXDXRResponse(req HQMinuteRequest, body []byte) ([]HQXDXRInfo, error) {
	if len(body) < 11 {
		return []HQXDXRInfo{}, nil
	}
	pos := 9
	count := int(binary.LittleEndian.Uint16(body[pos : pos+2]))
	pos += 2
	rows := make([]HQXDXRInfo, 0, count)
	for i := 0; i < count; i++ {
		if pos+29 > len(body) {
			return nil, fmt.Errorf("TDX HQ xdxr response truncated at item %d", i)
		}
		marketCode := int(body[pos])
		symbol := strings.TrimRight(string(body[pos+1:pos+7]), "\x00")
		pos += 8
		year, month, day, _, _, next, err := decodeHQDateTime(HQKLineDayAlt, body, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		category := int(body[pos])
		pos++
		market, err := quoteMarketName(marketCode)
		if err != nil {
			return nil, err
		}
		row := HQXDXRInfo{
			Market:   market,
			Symbol:   symbol,
			Year:     year,
			Month:    month,
			Day:      day,
			Date:     fmt.Sprintf("%04d-%02d-%02d", year, month, day),
			Category: category,
			Name:     hqXDXRCategoryName(category),
		}
		if category == 1 {
			if pos+16 > len(body) {
				return nil, fmt.Errorf("TDX HQ xdxr response truncated at item %d fields", i)
			}
			row.FenHong = floatPtr(readFloat32(body[pos:]))
			row.PeiGuJia = floatPtr(readFloat32(body[pos+4:]))
			row.SongZhuanGu = floatPtr(readFloat32(body[pos+8:]))
			row.PeiGu = floatPtr(readFloat32(body[pos+12:]))
		} else if category == 11 || category == 12 {
			if pos+16 > len(body) {
				return nil, fmt.Errorf("TDX HQ xdxr response truncated at item %d fields", i)
			}
			row.SuoGu = floatPtr(readFloat32(body[pos+8:]))
		} else if category == 13 || category == 14 {
			if pos+16 > len(body) {
				return nil, fmt.Errorf("TDX HQ xdxr response truncated at item %d fields", i)
			}
			row.XingQuanJia = floatPtr(readFloat32(body[pos:]))
			row.FenShu = floatPtr(readFloat32(body[pos+8:]))
		} else {
			if pos+16 > len(body) {
				return nil, fmt.Errorf("TDX HQ xdxr response truncated at item %d fields", i)
			}
			row.PanQianLiuTong = floatPtr(decodeTDXFloat(binary.LittleEndian.Uint32(body[pos:])))
			row.QianZongGuBen = floatPtr(decodeTDXFloat(binary.LittleEndian.Uint32(body[pos+4:])))
			row.PanHouLiuTong = floatPtr(decodeTDXFloat(binary.LittleEndian.Uint32(body[pos+8:])))
			row.HouZongGuBen = floatPtr(decodeTDXFloat(binary.LittleEndian.Uint32(body[pos+12:])))
		}
		pos += 16
		rows = append(rows, row)
	}
	return rows, nil
}

func DecodeHQFinanceInfoResponse(req HQMinuteRequest, body []byte) (HQFinanceInfo, error) {
	if len(body) < 145 {
		return HQFinanceInfo{}, fmt.Errorf("TDX HQ finance info response too short: %d bytes", len(body))
	}
	pos := 2
	if pos+7 > len(body) {
		return HQFinanceInfo{}, fmt.Errorf("TDX HQ finance info response truncated identity")
	}
	market, err := quoteMarketName(int(body[pos]))
	if err != nil {
		market = req.Market
	}
	symbol := strings.TrimRight(string(body[pos+1:pos+7]), "\x00")
	pos += 7
	if pos+136 > len(body) {
		return HQFinanceInfo{}, fmt.Errorf("TDX HQ finance info response truncated fields")
	}
	readF := func(offset int) float64 { return readFloat32(body[pos+offset:]) }
	return HQFinanceInfo{
		Market:            market,
		Symbol:            symbol,
		LiuTongGuBen:      readF(0) * 10000,
		Province:          binary.LittleEndian.Uint16(body[pos+4:]),
		Industry:          binary.LittleEndian.Uint16(body[pos+6:]),
		UpdatedDate:       binary.LittleEndian.Uint32(body[pos+8:]),
		IPODate:           binary.LittleEndian.Uint32(body[pos+12:]),
		ZongGuBen:         readF(16) * 10000,
		GuoJiaGu:          readF(20) * 10000,
		FaQiRenFaRenGu:    readF(24) * 10000,
		FaRenGu:           readF(28) * 10000,
		BGu:               readF(32) * 10000,
		HGu:               readF(36) * 10000,
		ZhiGongGu:         readF(40) * 10000,
		ZongZiChan:        readF(44) * 10000,
		LiuDongZiChan:     readF(48) * 10000,
		GuDingZiChan:      readF(52) * 10000,
		WuXingZiChan:      readF(56) * 10000,
		GuDongRenShu:      readF(60),
		LiuDongFuZhai:     readF(64) * 10000,
		ChangQiFuZhai:     readF(68) * 10000,
		ZiBenGongJiJin:    readF(72) * 10000,
		JingZiChan:        readF(76) * 10000,
		ZhuYingShouRu:     readF(80) * 10000,
		ZhuYingLiRun:      readF(84) * 10000,
		YingShouZhangKuan: readF(88) * 10000,
		YingYeLiRun:       readF(92) * 10000,
		TouZiShouYi:       readF(96) * 10000,
		JingYingXianJin:   readF(100) * 10000,
		ZongXianJin:       readF(104) * 10000,
		CunHuo:            readF(108) * 10000,
		LiRunZongE:        readF(112) * 10000,
		ShuiHouLiRun:      readF(116) * 10000,
		JingLiRun:         readF(120) * 10000,
		WeiFenPeiLiRun:    readF(124) * 10000,
		MeiGuJingZiChan:   readF(128),
		BaoLiu2:           readF(132),
	}, nil
}

func DecodeHQBlockMetaResponse(file string, body []byte) (HQBlockMeta, error) {
	if len(body) < 38 {
		return HQBlockMeta{}, fmt.Errorf("TDX HQ block meta response too short: %d bytes", len(body))
	}
	return HQBlockMeta{
		File:       strings.TrimSpace(file),
		Size:       binary.LittleEndian.Uint32(body[:4]),
		HashBase64: base64.StdEncoding.EncodeToString(body[5:37]),
	}, nil
}

func DecodeHQBlockInfoResponse(body []byte) ([]byte, error) {
	if len(body) < 4 {
		return nil, fmt.Errorf("TDX HQ block info response too short: %d bytes", len(body))
	}
	return append([]byte(nil), body[4:]...), nil
}

func DecodeHQBlockMembers(file string, data []byte) ([]HQBlockMember, error) {
	if len(data) < 386 {
		return nil, fmt.Errorf("TDX HQ block file %s too short: %d bytes", file, len(data))
	}
	count := int(binary.LittleEndian.Uint16(data[384:386]))
	pos := 386
	var out []HQBlockMember
	for i := 0; i < count; i++ {
		if pos+13 > len(data) {
			return nil, fmt.Errorf("TDX HQ block file truncated at block %d", i)
		}
		blockName := decodeTDXCString(data[pos : pos+9])
		pos += 9
		stockCount := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
		blockType := binary.LittleEndian.Uint16(data[pos+2 : pos+4])
		pos += 4
		blockStockBegin := pos
		for codeIndex := 0; codeIndex < stockCount; codeIndex++ {
			if pos+7 > len(data) {
				return nil, fmt.Errorf("TDX HQ block file truncated at block %d code %d", i, codeIndex)
			}
			code := strings.TrimRight(string(data[pos:pos+7]), "\x00")
			pos += 7
			market, symbol := splitBlockCode(code)
			out = append(out, HQBlockMember{
				BlockName: blockName,
				BlockType: blockType,
				CodeIndex: codeIndex,
				Code:      code,
				Market:    market,
				Symbol:    symbol,
			})
		}
		pos = blockStockBegin + 2800
	}
	return out, nil
}

func buildHQBarsPacket(req HQBarsRequest) []byte {
	packet := []byte{}
	packet = binary.LittleEndian.AppendUint16(packet, 0x010c)
	packet = binary.LittleEndian.AppendUint32(packet, 0x01016408)
	packet = binary.LittleEndian.AppendUint16(packet, 0x001c)
	packet = binary.LittleEndian.AppendUint16(packet, 0x001c)
	packet = binary.LittleEndian.AppendUint16(packet, 0x052d)
	packet = binary.LittleEndian.AppendUint16(packet, uint16(marketCodeForStandardHQ(req.Market)))
	packet = appendFixedASCII(packet, req.Symbol, 6)
	packet = binary.LittleEndian.AppendUint16(packet, uint16(req.Category))
	packet = binary.LittleEndian.AppendUint16(packet, 1)
	packet = binary.LittleEndian.AppendUint16(packet, uint16(req.Start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(req.Count))
	packet = binary.LittleEndian.AppendUint32(packet, 0)
	packet = binary.LittleEndian.AppendUint32(packet, 0)
	packet = binary.LittleEndian.AppendUint16(packet, 0)
	return packet
}

func decodeHQMinutePoints(req HQMinuteRequest, date int, body []byte, pos, count int) ([]HQMinutePoint, error) {
	lastPrice := 0
	points := make([]HQMinutePoint, 0, count)
	dateText := formatYYYYMMDD(date)
	for i := 0; i < count; i++ {
		priceRaw, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		_, pos, err = readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		vol, next, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		lastPrice += priceRaw
		minuteOfDay := 9*60 + 30 + i
		if i >= 120 {
			minuteOfDay = 13*60 + (i - 120)
		}
		points = append(points, HQMinutePoint{
			Market: req.Market,
			Symbol: req.Symbol,
			Date:   dateText,
			Time:   fmt.Sprintf("%02d:%02d", minuteOfDay/60, minuteOfDay%60),
			Index:  i,
			Price:  float64(lastPrice) / 100.0,
			Volume: int64(vol),
		})
	}
	return points, nil
}

func decodeHQTransactions(req HQMinuteRequest, date int, body []byte, pos, count int, includeNum bool) ([]HQTransaction, error) {
	lastPrice := 0
	dateText := formatYYYYMMDD(date)
	ticks := make([]HQTransaction, 0, count)
	for i := 0; i < count; i++ {
		hour, minute, next, err := decodeHQTime(body, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		priceRaw, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		vol, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		var num int
		if includeNum {
			num, pos, err = readTDXVarInt(body, pos)
			if err != nil {
				return nil, err
			}
		}
		buyOrSell, pos, err := readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		_, pos, err = readTDXVarInt(body, pos)
		if err != nil {
			return nil, err
		}
		lastPrice += priceRaw
		ticks = append(ticks, HQTransaction{
			Market:    req.Market,
			Symbol:    req.Symbol,
			Date:      dateText,
			Time:      fmt.Sprintf("%02d:%02d", hour, minute),
			Hour:      hour,
			Minute:    minute,
			Price:     float64(lastPrice) / 100.0,
			Volume:    int64(vol),
			Num:       int64(num),
			BuyOrSell: int64(buyOrSell),
		})
	}
	return ticks, nil
}

func decodeHQDateTime(category int, data []byte, pos int) (int, int, int, int, int, int, error) {
	if pos+4 > len(data) {
		return 0, 0, 0, 0, 0, pos, fmt.Errorf("datetime truncated at %d", pos)
	}
	if category < 4 || category == HQKLine1MinAlt || category == HQKLine1Min {
		zipDay := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
		tminutes := int(binary.LittleEndian.Uint16(data[pos+2 : pos+4]))
		year := (zipDay >> 11) + 2004
		rem := zipDay % 2048
		return year, rem / 100, rem % 100, tminutes / 60, tminutes % 60, pos + 4, nil
	}
	raw := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
	return raw / 10000, (raw % 10000) / 100, raw % 100, 15, 0, pos + 4, nil
}

func decodeHQTime(data []byte, pos int) (int, int, int, error) {
	if pos+2 > len(data) {
		return 0, 0, pos, fmt.Errorf("time truncated at %d", pos)
	}
	raw := int(binary.LittleEndian.Uint16(data[pos : pos+2]))
	return raw / 60, raw % 60, pos + 2, nil
}

func parseHQMarketSymbol(market, symbol, label string) (string, string, error) {
	market = strings.ToLower(strings.TrimSpace(market))
	symbol = strings.TrimSpace(symbol)
	if market == "" {
		market = InferMarketFromCode(symbol)
	}
	if market != "sh" && market != "sz" && market != "bj" {
		return "", "", fmt.Errorf("unsupported standard HQ %s market %q", label, market)
	}
	if len(symbol) != 6 || !allDigits(symbol) {
		return "", "", fmt.Errorf("unsupported standard HQ %s symbol %q", label, symbol)
	}
	return market, symbol, nil
}

func validateHQWindow(start, count, max int, label string) error {
	if start < 0 {
		return fmt.Errorf("standard HQ %s start must be non-negative", label)
	}
	if count <= 0 || count > max {
		return fmt.Errorf("standard HQ %s count must be between 1 and %d", label, max)
	}
	return nil
}

func validateHQDate(date int) error {
	if date < 19000101 || date > 29991231 {
		return fmt.Errorf("standard HQ date must be YYYYMMDD")
	}
	year := date / 10000
	month := (date % 10000) / 100
	day := date % 100
	if month < 1 || month > 12 || day < 1 || day > 31 || year < 1900 {
		return fmt.Errorf("standard HQ date must be YYYYMMDD")
	}
	return nil
}

func appendFixedASCII(packet []byte, value string, size int) []byte {
	raw := make([]byte, size)
	copy(raw, []byte(value))
	return append(packet, raw...)
}

func decodeTDXCString(raw []byte) string {
	raw = bytesTrimRight(raw, 0x00, ' ')
	if len(raw) == 0 {
		return ""
	}
	return decodeTDXText(raw)
}

func decodeTDXText(raw []byte) string {
	text, err := simplifiedchinese.GB18030.NewDecoder().String(string(raw))
	if err != nil {
		return ""
	}
	if strings.ContainsRune(text, '\uFFFD') {
		return ""
	}
	return strings.TrimRight(text, "\x00 ")
}

func formatYYYYMMDD(date int) string {
	if date == 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", date/10000, (date%10000)/100, date%100)
}

func splitBlockCode(code string) (string, string) {
	code = strings.TrimSpace(code)
	if len(code) == 7 && (code[0] == '0' || code[0] == '1' || code[0] == '2') {
		market, err := quoteMarketName(int(code[0] - '0'))
		if err == nil {
			return market, code[1:]
		}
	}
	if len(code) == 6 && allDigits(code) {
		return InferMarketFromCode(code), code
	}
	return "", code
}

func floatPtr(v float64) *float64 {
	return &v
}

func hqXDXRCategoryName(category int) string {
	switch category {
	case 1:
		return "除权除息"
	case 2:
		return "送配股上市"
	case 3:
		return "非流通股上市"
	case 4:
		return "未知股本变动"
	case 5:
		return "股本变化"
	case 6:
		return "增发新股"
	case 7:
		return "股份回购"
	case 8:
		return "增发新股上市"
	case 9:
		return "转配股上市"
	case 10:
		return "可转债上市"
	case 11:
		return "扩缩股"
	case 12:
		return "非流通股缩股"
	case 13:
		return "送认购权证"
	case 14:
		return "送认沽权证"
	default:
		return fmt.Sprintf("%d", category)
	}
}
