package tdx

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

const (
	ExKLine5Min     = 0
	ExKLine15Min    = 1
	ExKLine30Min    = 2
	ExKLine1Hour    = 3
	ExKLineDaily    = 4
	ExKLineWeekly   = 5
	ExKLineMonthly  = 6
	ExKLineExHQ1Min = 7
	ExKLine1Min     = 8
	ExKLineRi       = 9
	ExKLineQuarter  = 10
	ExKLineYearly   = 11

	DefaultExInstrumentListCount = 100
	MaxExInstrumentListCount     = 1000
	MaxExBarsCount               = 800
	MaxExTransactionCount        = 1800
)

type ExInstrument struct {
	Category int    `json:"category"`
	Market   int    `json:"market"`
	Code     string `json:"code"`
	Name     string `json:"name,omitempty"`
	Desc     string `json:"desc,omitempty"`
}

type ExBarsRequest struct {
	Category int    `json:"category"`
	Market   int    `json:"market"`
	Code     string `json:"code"`
	Start    int    `json:"start"`
	Count    int    `json:"count"`
}

type ExBar struct {
	Market          int     `json:"market"`
	Code            string  `json:"code"`
	Category        int     `json:"category"`
	DateTime        string  `json:"datetime"`
	Year            int     `json:"year"`
	Month           int     `json:"month"`
	Day             int     `json:"day"`
	Hour            int     `json:"hour"`
	Minute          int     `json:"minute"`
	Open            float64 `json:"open"`
	High            float64 `json:"high"`
	Low             float64 `json:"low"`
	Close           float64 `json:"close"`
	Position        int64   `json:"position"`
	Trade           int64   `json:"trade"`
	Price           float64 `json:"price,omitempty"`
	Amount          float64 `json:"amount,omitempty"`
	SettlementPrice float64 `json:"settlement_price,omitempty"`
}

type ExMinutePoint struct {
	Market       int     `json:"market"`
	Code         string  `json:"code"`
	Date         string  `json:"date,omitempty"`
	DateTime     string  `json:"datetime,omitempty"`
	Time         string  `json:"time"`
	Hour         int     `json:"hour"`
	Minute       int     `json:"minute"`
	Price        float64 `json:"price"`
	AvgPrice     float64 `json:"avg_price"`
	Volume       int64   `json:"volume"`
	OpenInterest int64   `json:"open_interest"`
}

type ExTransaction struct {
	Market      int    `json:"market"`
	Code        string `json:"code"`
	Date        string `json:"date,omitempty"`
	DateTime    string `json:"datetime,omitempty"`
	Time        string `json:"time"`
	Hour        int    `json:"hour"`
	Minute      int    `json:"minute"`
	Second      int    `json:"second"`
	Price       int64  `json:"price"`
	Volume      int64  `json:"volume"`
	ZengCang    int64  `json:"zengcang"`
	Nature      int    `json:"nature"`
	NatureMark  int    `json:"nature_mark"`
	NatureValue int    `json:"nature_value"`
	NatureName  string `json:"nature_name"`
	Direction   int    `json:"direction"`
}

func ParseExBarsRequest(category, market int, code string, start, count int) (ExBarsRequest, error) {
	req, err := ParseExQuoteRequest(market, code)
	if err != nil {
		return ExBarsRequest{}, err
	}
	if category < ExKLine5Min || category > ExKLineYearly {
		return ExBarsRequest{}, fmt.Errorf("unsupported extended quote K-line category %d", category)
	}
	if start < 0 {
		return ExBarsRequest{}, fmt.Errorf("extended quote K-line start must be non-negative")
	}
	if count <= 0 || count > MaxExBarsCount {
		return ExBarsRequest{}, fmt.Errorf("extended quote K-line count must be between 1 and %d", MaxExBarsCount)
	}
	return ExBarsRequest{
		Category: category,
		Market:   req.Market,
		Code:     req.Code,
		Start:    start,
		Count:    count,
	}, nil
}

func (s *ExQuoteSession) InstrumentCount() (int, error) {
	body, err := s.call(BuildExInstrumentCountPacket())
	if err != nil {
		return 0, fmt.Errorf("TDX ExHQ instrument count %s: %w", s.server, err)
	}
	count, err := DecodeExInstrumentCountResponse(body)
	if err != nil {
		return 0, fmt.Errorf("decode TDX ExHQ instrument count response from %s: %w", s.server, err)
	}
	return count, nil
}

func (s *ExQuoteSession) Instruments(start, count int) ([]ExInstrument, error) {
	packet, err := BuildExInstrumentInfoPacket(start, count)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ instrument list %s: %w", s.server, err)
	}
	instruments, err := DecodeExInstrumentInfoResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ instrument list response from %s: %w", s.server, err)
	}
	return instruments, nil
}

func (s *ExQuoteSession) Bars(req ExBarsRequest) ([]ExBar, error) {
	normalized, err := ParseExBarsRequest(req.Category, req.Market, req.Code, req.Start, req.Count)
	if err != nil {
		return nil, err
	}
	packet, err := BuildExBarsRequestPacket(normalized)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ K-line request %s: %w", s.server, err)
	}
	bars, err := DecodeExBarsResponse(normalized, body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ K-line response from %s: %w", s.server, err)
	}
	return bars, nil
}

func (s *ExQuoteSession) MinuteTime(req ExQuoteRequest) ([]ExMinutePoint, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return nil, err
	}
	body, err := s.call(BuildExMinuteTimePacket(normalized))
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ minute-time request %s: %w", s.server, err)
	}
	points, err := DecodeExMinuteTimeResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ minute-time response from %s: %w", s.server, err)
	}
	return points, nil
}

func (s *ExQuoteSession) HistoryMinuteTime(req ExQuoteRequest, date int) ([]ExMinutePoint, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return nil, err
	}
	if err := validateExDate(date); err != nil {
		return nil, err
	}
	body, err := s.call(BuildExHistoryMinuteTimePacket(normalized, date))
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ history minute-time request %s: %w", s.server, err)
	}
	points, err := DecodeExHistoryMinuteTimeResponse(date, body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ history minute-time response from %s: %w", s.server, err)
	}
	return points, nil
}

func (s *ExQuoteSession) Transactions(req ExQuoteRequest, start, count int) ([]ExTransaction, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return nil, err
	}
	packet, err := BuildExTransactionPacket(normalized, start, count)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ transaction request %s: %w", s.server, err)
	}
	transactions, err := DecodeExTransactionResponse(body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ transaction response from %s: %w", s.server, err)
	}
	return transactions, nil
}

func (s *ExQuoteSession) HistoryTransactions(req ExQuoteRequest, date, start, count int) ([]ExTransaction, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return nil, err
	}
	packet, err := BuildExHistoryTransactionPacket(normalized, date, start, count)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ history transaction request %s: %w", s.server, err)
	}
	transactions, err := DecodeExHistoryTransactionResponse(date, body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ history transaction response from %s: %w", s.server, err)
	}
	return transactions, nil
}

func (s *ExQuoteSession) HistoryBarsRange(req ExQuoteRequest, startDate, endDate int) ([]ExBar, error) {
	normalized, err := ParseExQuoteRequest(req.Market, req.Code)
	if err != nil {
		return nil, err
	}
	packet, err := BuildExHistoryBarsRangePacket(normalized, startDate, endDate)
	if err != nil {
		return nil, err
	}
	body, err := s.call(packet)
	if err != nil {
		return nil, fmt.Errorf("TDX ExHQ history K-line range request %s: %w", s.server, err)
	}
	bars, err := DecodeExHistoryBarsRangeResponse(normalized, body)
	if err != nil {
		return nil, fmt.Errorf("decode TDX ExHQ history K-line range response from %s: %w", s.server, err)
	}
	return bars, nil
}

func FetchExInstrumentCount(ctx context.Context, opts ExQuoteClientOptions) (int, error) {
	return fetchExRead(ctx, opts, "instrument count", func(session *ExQuoteSession) (int, error) {
		return session.InstrumentCount()
	})
}

func FetchExInstruments(ctx context.Context, start, count int, opts ExQuoteClientOptions) ([]ExInstrument, error) {
	return fetchExRead(ctx, opts, "instrument list", func(session *ExQuoteSession) ([]ExInstrument, error) {
		return session.Instruments(start, count)
	})
}

func FetchExBars(ctx context.Context, req ExBarsRequest, opts ExQuoteClientOptions) ([]ExBar, error) {
	return fetchExRead(ctx, opts, "K-line", func(session *ExQuoteSession) ([]ExBar, error) {
		return session.Bars(req)
	})
}

func FetchExMinuteTime(ctx context.Context, req ExQuoteRequest, opts ExQuoteClientOptions) ([]ExMinutePoint, error) {
	return fetchExRead(ctx, opts, "minute-time", func(session *ExQuoteSession) ([]ExMinutePoint, error) {
		return session.MinuteTime(req)
	})
}

func FetchExHistoryMinuteTime(ctx context.Context, req ExQuoteRequest, date int, opts ExQuoteClientOptions) ([]ExMinutePoint, error) {
	return fetchExRead(ctx, opts, "history minute-time", func(session *ExQuoteSession) ([]ExMinutePoint, error) {
		return session.HistoryMinuteTime(req, date)
	})
}

func FetchExTransactions(ctx context.Context, req ExQuoteRequest, start, count int, opts ExQuoteClientOptions) ([]ExTransaction, error) {
	return fetchExRead(ctx, opts, "transaction", func(session *ExQuoteSession) ([]ExTransaction, error) {
		return session.Transactions(req, start, count)
	})
}

func FetchExHistoryTransactions(ctx context.Context, req ExQuoteRequest, date, start, count int, opts ExQuoteClientOptions) ([]ExTransaction, error) {
	return fetchExRead(ctx, opts, "history transaction", func(session *ExQuoteSession) ([]ExTransaction, error) {
		return session.HistoryTransactions(req, date, start, count)
	})
}

func FetchExHistoryBarsRange(ctx context.Context, req ExQuoteRequest, startDate, endDate int, opts ExQuoteClientOptions) ([]ExBar, error) {
	return fetchExRead(ctx, opts, "history K-line range", func(session *ExQuoteSession) ([]ExBar, error) {
		return session.HistoryBarsRange(req, startDate, endDate)
	})
}

func fetchExRead[T any](ctx context.Context, opts ExQuoteClientOptions, label string, fn func(*ExQuoteSession) (T, error)) (T, error) {
	var zero T
	var attempts []string
	for _, server := range NormalizeExHQServers(opts) {
		session, err := OpenExQuoteSession(ctx, server, opts.Timeout)
		if err != nil {
			attempts = append(attempts, err.Error())
			continue
		}
		value, fetchErr := fn(session)
		_ = session.Close()
		if fetchErr != nil {
			attempts = append(attempts, fetchErr.Error())
			if strings.Contains(fetchErr.Error(), "decode TDX ExHQ") {
				return zero, fetchErr
			}
			continue
		}
		return value, nil
	}
	return zero, fmt.Errorf("TDX ExHQ %s failed on %d server(s): %s", label, len(attempts), strings.Join(attempts, "; "))
}

func BuildExInstrumentCountPacket() []byte {
	return []byte{0x01, 0x03, 0x48, 0x66, 0x00, 0x01, 0x02, 0x00, 0x02, 0x00, 0xf0, 0x23}
}

func BuildExInstrumentInfoPacket(start, count int) ([]byte, error) {
	if start < 0 {
		return nil, fmt.Errorf("extended quote instrument list start must be non-negative")
	}
	if count <= 0 || count > MaxExInstrumentListCount {
		return nil, fmt.Errorf("extended quote instrument list count must be between 1 and %d", MaxExInstrumentListCount)
	}
	packet := []byte{0x01, 0x04, 0x48, 0x67, 0x00, 0x01, 0x08, 0x00, 0x08, 0x00, 0xf5, 0x23}
	packet = binary.LittleEndian.AppendUint32(packet, uint32(start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(count))
	return packet, nil
}

func BuildExBarsRequestPacket(req ExBarsRequest) ([]byte, error) {
	normalized, err := ParseExBarsRequest(req.Category, req.Market, req.Code, req.Start, req.Count)
	if err != nil {
		return nil, err
	}
	packet := []byte{0x01, 0x01, 0x08, 0x6a, 0x01, 0x01, 0x16, 0x00, 0x16, 0x00, 0xff, 0x23}
	packet = append(packet, byte(normalized.Market))
	packet = append(packet, exCodeBytes(normalized.Code)...)
	packet = binary.LittleEndian.AppendUint16(packet, uint16(normalized.Category))
	packet = binary.LittleEndian.AppendUint16(packet, 1)
	packet = binary.LittleEndian.AppendUint32(packet, uint32(normalized.Start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(normalized.Count))
	return packet, nil
}

func BuildExMinuteTimePacket(req ExQuoteRequest) []byte {
	packet := []byte{0x01, 0x07, 0x08, 0x00, 0x01, 0x01, 0x0c, 0x00, 0x0c, 0x00, 0x0b, 0x24}
	packet = append(packet, byte(req.Market))
	packet = append(packet, exCodeBytes(req.Code)...)
	return packet
}

func BuildExHistoryMinuteTimePacket(req ExQuoteRequest, date int) []byte {
	packet := []byte{0x01, 0x01, 0x30, 0x00, 0x01, 0x01, 0x10, 0x00, 0x10, 0x00, 0x0c, 0x24}
	packet = binary.LittleEndian.AppendUint32(packet, uint32(date))
	packet = append(packet, byte(req.Market))
	packet = append(packet, exCodeBytes(req.Code)...)
	return packet
}

func BuildExTransactionPacket(req ExQuoteRequest, start, count int) ([]byte, error) {
	if err := validateExWindow(start, count, MaxExTransactionCount, "transaction"); err != nil {
		return nil, err
	}
	packet := []byte{0x01, 0x01, 0x08, 0x00, 0x03, 0x01, 0x12, 0x00, 0x12, 0x00, 0xfc, 0x23}
	packet = append(packet, byte(req.Market))
	packet = append(packet, exCodeBytes(req.Code)...)
	packet = binary.LittleEndian.AppendUint32(packet, uint32(start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(count))
	return packet, nil
}

func BuildExHistoryTransactionPacket(req ExQuoteRequest, date, start, count int) ([]byte, error) {
	if err := validateExDate(date); err != nil {
		return nil, err
	}
	if err := validateExWindow(start, count, MaxExTransactionCount, "history transaction"); err != nil {
		return nil, err
	}
	packet := []byte{0x01, 0x01, 0x30, 0x00, 0x02, 0x01, 0x16, 0x00, 0x16, 0x00, 0x06, 0x24}
	packet = binary.LittleEndian.AppendUint32(packet, uint32(date))
	packet = append(packet, byte(req.Market))
	packet = append(packet, exCodeBytes(req.Code)...)
	packet = binary.LittleEndian.AppendUint32(packet, uint32(start))
	packet = binary.LittleEndian.AppendUint16(packet, uint16(count))
	return packet, nil
}

func BuildExHistoryBarsRangePacket(req ExQuoteRequest, startDate, endDate int) ([]byte, error) {
	if err := validateExDate(startDate); err != nil {
		return nil, fmt.Errorf("start date: %w", err)
	}
	if err := validateExDate(endDate); err != nil {
		return nil, fmt.Errorf("end date: %w", err)
	}
	if startDate > endDate {
		return nil, fmt.Errorf("extended quote history K-line range start date must be <= end date")
	}
	packet := []byte{0x01, 0x01, 0x38, 0x92, 0x00, 0x01, 0x16, 0x00, 0x16, 0x00, 0x0d, 0x24}
	packet = append(packet, byte(req.Market))
	packet = append(packet, exCodeBytes(req.Code)...)
	packet = binary.LittleEndian.AppendUint16(packet, ExKLineExHQ1Min)
	packet = binary.LittleEndian.AppendUint32(packet, uint32(startDate))
	packet = binary.LittleEndian.AppendUint32(packet, uint32(endDate))
	return packet, nil
}

func DecodeExInstrumentCountResponse(body []byte) (int, error) {
	if len(body) < 23 {
		return 0, fmt.Errorf("TDX ExHQ instrument count response too short: %d bytes", len(body))
	}
	return int(binary.LittleEndian.Uint32(body[19:23])), nil
}

func DecodeExInstrumentInfoResponse(body []byte) ([]ExInstrument, error) {
	if len(body) < 6 {
		return nil, fmt.Errorf("TDX ExHQ instrument list response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[4:6]))
	pos := 6
	instruments := make([]ExInstrument, 0, count)
	for i := 0; i < count; i++ {
		if pos+64 > len(body) {
			return nil, fmt.Errorf("TDX ExHQ instrument list response truncated at item %d", i)
		}
		record := body[pos : pos+64]
		instruments = append(instruments, ExInstrument{
			Category: int(record[0]),
			Market:   int(record[1]),
			Code:     decodeExCString(record[5:14]),
			Name:     decodeExCString(record[14:31]),
			Desc:     decodeExCString(record[31:40]),
		})
		pos += 64
	}
	return instruments, nil
}

func DecodeExBarsResponse(req ExBarsRequest, body []byte) ([]ExBar, error) {
	if len(body) < 20 {
		return nil, fmt.Errorf("TDX ExHQ K-line response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[18:20]))
	pos := 20
	bars := make([]ExBar, 0, count)
	for i := 0; i < count; i++ {
		year, month, day, hour, minute, next, err := decodeExBarDateTime(req.Category, body, pos)
		if err != nil {
			return nil, fmt.Errorf("TDX ExHQ K-line response truncated at item %d datetime: %w", i, err)
		}
		pos = next
		if pos+28 > len(body) {
			return nil, fmt.Errorf("TDX ExHQ K-line response truncated at item %d", i)
		}
		bars = append(bars, ExBar{
			Market:   req.Market,
			Code:     req.Code,
			Category: req.Category,
			DateTime: exDateTimeString(year, month, day, hour, minute),
			Year:     year,
			Month:    month,
			Day:      day,
			Hour:     hour,
			Minute:   minute,
			Open:     readFloat32(body[pos:]),
			High:     readFloat32(body[pos+4:]),
			Low:      readFloat32(body[pos+8:]),
			Close:    readFloat32(body[pos+12:]),
			Position: readUint32AsInt64(body[pos+16:]),
			Trade:    readUint32AsInt64(body[pos+20:]),
			Price:    readFloat32(body[pos+24:]),
			Amount:   readFloat32(body[pos+16:]),
		})
		pos += 28
	}
	return bars, nil
}

func DecodeExMinuteTimeResponse(body []byte) ([]ExMinutePoint, error) {
	return decodeExMinuteTimeResponse(body, 12, 10, 0)
}

func DecodeExHistoryMinuteTimeResponse(date int, body []byte) ([]ExMinutePoint, error) {
	return decodeExMinuteTimeResponse(body, 20, 18, date)
}

func DecodeExTransactionResponse(body []byte) ([]ExTransaction, error) {
	return decodeExTransactionResponse(body, 0)
}

func DecodeExHistoryTransactionResponse(date int, body []byte) ([]ExTransaction, error) {
	return decodeExTransactionResponse(body, date)
}

func DecodeExHistoryBarsRangeResponse(req ExQuoteRequest, body []byte) ([]ExBar, error) {
	if len(body) < 14 {
		return nil, fmt.Errorf("TDX ExHQ history K-line range response too short: %d bytes", len(body))
	}
	count := int(binary.LittleEndian.Uint16(body[12:14]))
	pos := 14
	bars := make([]ExBar, 0, count)
	for i := 0; i < count; i++ {
		if pos+32 > len(body) {
			return nil, fmt.Errorf("TDX ExHQ history K-line range response truncated at item %d", i)
		}
		year, month, day := decodeExPackedDay(binary.LittleEndian.Uint16(body[pos:]))
		hour, minute := decodeExPackedMinute(binary.LittleEndian.Uint16(body[pos+2:]))
		bars = append(bars, ExBar{
			Market:          req.Market,
			Code:            req.Code,
			Category:        ExKLineExHQ1Min,
			DateTime:        exDateTimeString(year, month, day, hour, minute),
			Year:            year,
			Month:           month,
			Day:             day,
			Hour:            hour,
			Minute:          minute,
			Open:            readFloat32(body[pos+4:]),
			High:            readFloat32(body[pos+8:]),
			Low:             readFloat32(body[pos+12:]),
			Close:           readFloat32(body[pos+16:]),
			Position:        readUint32AsInt64(body[pos+20:]),
			Trade:           readUint32AsInt64(body[pos+24:]),
			SettlementPrice: readFloat32(body[pos+28:]),
		})
		pos += 32
	}
	return bars, nil
}

func decodeExMinuteTimeResponse(body []byte, headerLen, countOffset, date int) ([]ExMinutePoint, error) {
	if len(body) < headerLen {
		return nil, fmt.Errorf("TDX ExHQ minute-time response too short: %d bytes", len(body))
	}
	market := int(body[0])
	code := decodeExCString(body[1:10])
	count := int(binary.LittleEndian.Uint16(body[countOffset : countOffset+2]))
	dateText := formatExDate(date)
	pos := headerLen
	points := make([]ExMinutePoint, 0, count)
	for i := 0; i < count; i++ {
		if pos+18 > len(body) {
			return nil, fmt.Errorf("TDX ExHQ minute-time response truncated at item %d", i)
		}
		rawTime := binary.LittleEndian.Uint16(body[pos:])
		hour, minute := decodeExPackedMinute(rawTime)
		timeText := fmt.Sprintf("%02d:%02d", hour, minute)
		point := ExMinutePoint{
			Market:       market,
			Code:         code,
			Date:         dateText,
			Time:         timeText,
			Hour:         hour,
			Minute:       minute,
			Price:        readFloat32(body[pos+2:]),
			AvgPrice:     readFloat32(body[pos+6:]),
			Volume:       readUint32AsInt64(body[pos+10:]),
			OpenInterest: readUint32AsInt64(body[pos+14:]),
		}
		if dateText != "" {
			point.DateTime = dateText + " " + timeText
		}
		points = append(points, point)
		pos += 18
	}
	return points, nil
}

func decodeExTransactionResponse(body []byte, date int) ([]ExTransaction, error) {
	if len(body) < 16 {
		return nil, fmt.Errorf("TDX ExHQ transaction response too short: %d bytes", len(body))
	}
	market := int(body[0])
	code := decodeExCString(body[1:10])
	count := int(binary.LittleEndian.Uint16(body[14:16]))
	dateText := formatExDate(date)
	pos := 16
	transactions := make([]ExTransaction, 0, count)
	for i := 0; i < count; i++ {
		if pos+16 > len(body) {
			return nil, fmt.Errorf("TDX ExHQ transaction response truncated at item %d", i)
		}
		rawTime := binary.LittleEndian.Uint16(body[pos:])
		hour, minute := decodeExPackedMinute(rawTime)
		price := readUint32AsInt64(body[pos+2:])
		volume := readUint32AsInt64(body[pos+6:])
		zengcang := int64(int32(binary.LittleEndian.Uint32(body[pos+10:])))
		nature := int(binary.LittleEndian.Uint16(body[pos+14:]))
		natureName, direction, second := decodeExTransactionNature(market, volume, zengcang, nature)
		timeText := fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
		item := ExTransaction{
			Market:      market,
			Code:        code,
			Date:        dateText,
			Time:        timeText,
			Hour:        hour,
			Minute:      minute,
			Second:      second,
			Price:       price,
			Volume:      volume,
			ZengCang:    zengcang,
			Nature:      nature,
			NatureMark:  nature / 10000,
			NatureValue: nature % 10000,
			NatureName:  natureName,
			Direction:   direction,
		}
		if dateText != "" {
			item.DateTime = dateText + " " + timeText
		}
		transactions = append(transactions, item)
		pos += 16
	}
	return transactions, nil
}

func decodeExTransactionNature(market int, volume, zengcang int64, nature int) (string, int, int) {
	second := nature % 10000
	if second > 59 {
		second = 0
	}
	value := nature / 10000
	natureName := "换手"
	direction := 0

	switch value {
	case 0:
		direction = 1
		if zengcang > 0 {
			if volume > zengcang {
				natureName = "多开"
			} else if volume == zengcang {
				natureName = "双开"
			}
		} else if zengcang == 0 {
			natureName = "多换"
		} else if volume == -zengcang {
			natureName = "双平"
		} else {
			natureName = "空平"
		}
	case 1:
		direction = -1
		if zengcang > 0 {
			if volume > zengcang {
				natureName = "空开"
			} else if volume == zengcang {
				natureName = "双开"
			}
		} else if zengcang == 0 {
			natureName = "空换"
		} else if volume == -zengcang {
			natureName = "双平"
		} else {
			natureName = "多平"
		}
	default:
		if zengcang > 0 {
			if volume > zengcang {
				natureName = "开仓"
			} else if volume == zengcang {
				natureName = "双开"
			}
		} else if zengcang < 0 {
			if volume > -zengcang {
				natureName = "平仓"
			} else if volume == -zengcang {
				natureName = "双平"
			}
		}
	}

	if market == 31 || market == 48 {
		switch nature {
		case 0:
			return "B", 1, second
		case 256:
			return "S", -1, second
		default:
			return "", 0, second
		}
	}
	return natureName, direction, second
}

func decodeExBarDateTime(category int, body []byte, pos int) (int, int, int, int, int, int, error) {
	if category < ExKLineDaily || category == ExKLineExHQ1Min || category == ExKLine1Min {
		if pos+4 > len(body) {
			return 0, 0, 0, 0, 0, pos, fmt.Errorf("need 4 bytes, got %d", len(body)-pos)
		}
		year, month, day := decodeExPackedDay(binary.LittleEndian.Uint16(body[pos:]))
		hour, minute := decodeExPackedMinute(binary.LittleEndian.Uint16(body[pos+2:]))
		return year, month, day, hour, minute, pos + 4, nil
	}
	if pos+4 > len(body) {
		return 0, 0, 0, 0, 0, pos, fmt.Errorf("need 4 bytes, got %d", len(body)-pos)
	}
	rawDate := int(binary.LittleEndian.Uint32(body[pos:]))
	year := rawDate / 10000
	month := rawDate % 10000 / 100
	day := rawDate % 100
	return year, month, day, 15, 0, pos + 4, nil
}

func decodeExPackedDay(raw uint16) (int, int, int) {
	date := int(raw)
	year := date/2048 + 2004
	month := (date % 2048) / 100
	day := (date % 2048) % 100
	return year, month, day
}

func decodeExPackedMinute(raw uint16) (int, int) {
	minutes := int(raw)
	return minutes / 60, minutes % 60
}

func validateExWindow(start, count, max int, label string) error {
	if start < 0 {
		return fmt.Errorf("extended quote %s start must be non-negative", label)
	}
	if count <= 0 || count > max {
		return fmt.Errorf("extended quote %s count must be between 1 and %d", label, max)
	}
	return nil
}

func validateExDate(date int) error {
	if date <= 0 {
		return fmt.Errorf("extended quote date must be YYYYMMDD")
	}
	if _, err := time.Parse("20060102", fmt.Sprintf("%08d", date)); err != nil {
		return fmt.Errorf("extended quote date must be YYYYMMDD: %w", err)
	}
	return nil
}

func formatExDate(date int) string {
	if date <= 0 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", date/10000, date%10000/100, date%100)
}

func exDateTimeString(year, month, day, hour, minute int) string {
	return fmt.Sprintf("%04d-%02d-%02d %02d:%02d", year, month, day, hour, minute)
}

func exCodeBytes(code string) []byte {
	raw := make([]byte, 9)
	copy(raw, code)
	return raw
}

func decodeExCString(raw []byte) string {
	raw = trimExCString(raw)
	if len(raw) == 0 {
		return ""
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(decoded))
}

func trimExCString(raw []byte) []byte {
	end := len(raw)
	for end > 0 && (raw[end-1] == 0 || raw[end-1] == ' ') {
		end--
	}
	return raw[:end]
}
