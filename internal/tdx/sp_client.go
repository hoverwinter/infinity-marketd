package tdx

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

// SP (mac_quotation) protocol support ported from millken/tdx: live board
// member reads (0x122C). SP mode uses a distinct bootstrap (ping/connect-auth/
// stage2) plus an SP login (0x2454), and its responses arrive as 0x01-prefixed
// SP frames rather than the standard 16-byte response header. This is a live
// read; it never writes ClickHouse. There are no known-good public SP server
// defaults, so callers must pass an explicit server.

const (
	cmdSPPing        uint16 = 0x0015
	cmdSPConnectAuth uint16 = 0x000D
	cmdSPStage2      uint16 = 0x0FDB
	cmdSPLogin       uint16 = 0x2454
	cmdBoardMembers  uint16 = 0x122C
)

// spLoginBody is the 80-byte encrypted SP login blob (verbatim from millken/tdx).
var spLoginBody = []byte{
	0xe5, 0xbb, 0x1c, 0x2f, 0xaf, 0xe5, 0x25, 0x94,
	0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41,
	0x5b, 0x73, 0x4c, 0xc9, 0xcd, 0xbf, 0x0a, 0xc9,
	0x20, 0x21, 0xbf, 0xdd, 0x1e, 0xb0, 0x6d, 0x22,
	0xd0, 0x08, 0x88, 0x4c, 0x16, 0x11, 0xcb, 0x13,
	0x78, 0xf6, 0xab, 0xd8, 0x24, 0xd8, 0x99, 0xd2,
	0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41,
	0x1f, 0x32, 0xc6, 0xe5, 0xd5, 0x3d, 0xfb, 0x41,
	0xa9, 0x32, 0x5a, 0xc9, 0x35, 0xdc, 0x08, 0x37,
	0x33, 0x5a, 0x16, 0xe4, 0xce, 0x17, 0xc1, 0xbb,
}

// spBoardBitmap selects which per-stock fields the server returns (millken default).
var spBoardBitmap = []byte{
	0xff, 0xfc, 0xe1, 0xcc, 0x3f, 0x08, 0x03, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x12, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00,
}

// HQBoardMember is one stock in an SP board members list. Decoded fields are
// keyed by name in Fields; raw active bit ids are preserved in ActiveBits.
type HQBoardMember struct {
	Market     string             `json:"market"`
	MarketCode uint16             `json:"market_code"`
	Symbol     string             `json:"symbol"`
	Name       string             `json:"name"`
	Fields     map[string]float64 `json:"fields"`
	ActiveBits []int              `json:"active_bits"`
}

// SPSession is a connected, bootstrapped, logged-in SP-mode session.
type SPSession struct {
	server  string
	conn    net.Conn
	r       *bufio.Reader
	timeout time.Duration
	seq     uint16
}

// OpenSPSession dials, performs the SP bootstrap (ping/connect-auth/stage2) and
// SP login, returning a ready session.
func OpenSPSession(ctx context.Context, server string, timeout time.Duration) (*SPSession, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, fmt.Errorf("SP server is required (no public SP defaults)")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", server)
	if err != nil {
		return nil, fmt.Errorf("connect SP server %s: %w", server, err)
	}
	return spHandshake(conn, server, timeout)
}

// spHandshake performs the SP bootstrap + login on an already-connected conn.
func spHandshake(conn net.Conn, server string, timeout time.Duration) (*SPSession, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	s := &SPSession{server: server, conn: conn, r: bufio.NewReader(conn), timeout: timeout}
	steps := []struct {
		name   string
		packet []byte
		cmd    uint16
	}{
		{"ping", pingPacket(s.nextSeq()), cmdSPPing},
		{"connect-auth", connectAuthPacket(s.nextSeq()), cmdSPConnectAuth},
		{"stage2", stage2Packet(s.nextSeq()), cmdSPStage2},
		{"sp-login", buildSPFrame(0x01, cmdSPLogin, spLoginBody), cmdSPLogin},
	}
	for _, step := range steps {
		if _, err := s.exchange(step.packet, step.cmd); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("SP %s %s: %w", step.name, server, err)
		}
	}
	return s, nil
}

// Close closes the underlying connection.
func (s *SPSession) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *SPSession) nextSeq() uint16 {
	seq := s.seq
	s.seq++
	return seq
}

// exchange sends a packet and reads frames until one matches cmd.
func (s *SPSession) exchange(packet []byte, cmd uint16) ([]byte, error) {
	if err := s.conn.SetDeadline(time.Now().Add(s.timeout)); err != nil {
		return nil, err
	}
	if _, err := s.conn.Write(packet); err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	deadline := time.Now().Add(s.timeout)
	for {
		gotCmd, body, err := readSPFrame(s.r)
		if err != nil {
			return nil, err
		}
		if gotCmd == cmd {
			return body, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for cmd 0x%04X", cmd)
		}
	}
}

// BoardMembers fetches board members with auto-pagination up to count.
func (s *SPSession) BoardMembers(boardCode uint32, sortType uint16, count int, sortOrder uint16) ([]HQBoardMember, error) {
	const pageSize = 80
	var all []HQBoardMember
	start := uint32(0)
	for len(all) < count {
		body, err := s.exchange(buildBoardMembersFrame(boardCode, sortType, start, pageSize, sortOrder), cmdBoardMembers)
		if err != nil {
			return nil, err
		}
		items, err := DecodeBoardMembers(body)
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			break
		}
		all = append(all, items...)
		start += uint32(len(items))
		if len(items) < pageSize {
			break
		}
	}
	if len(all) > count {
		all = all[:count]
	}
	return all, nil
}

// FetchSPBoardMembers opens an SP session to an explicit server and fetches members.
func FetchSPBoardMembers(ctx context.Context, server string, board string, sortType uint16, count int, sortOrder uint16, timeout time.Duration) ([]HQBoardMember, error) {
	if count <= 0 {
		return nil, fmt.Errorf("board member count must be > 0")
	}
	session, err := OpenSPSession(ctx, server, timeout)
	if err != nil {
		return nil, err
	}
	defer session.Close()
	return session.BoardMembers(ExchangeBoardCode(board), sortType, count, sortOrder)
}

// buildBoardMembersFrame builds the 0x122C SP request body + frame.
func buildBoardMembersFrame(boardCode uint32, sortType uint16, start uint32, pageSize uint8, sortOrder uint16) []byte {
	body := make([]byte, 0, 43)
	part1 := make([]byte, 13)
	binary.LittleEndian.PutUint32(part1[0:4], boardCode)
	body = append(body, part1...)
	part2 := make([]byte, 10)
	binary.LittleEndian.PutUint16(part2[0:2], sortType)
	binary.LittleEndian.PutUint32(part2[2:6], start)
	part2[6] = pageSize
	part2[8] = byte(sortOrder)
	body = append(body, part2...)
	body = append(body, spBoardBitmap...)
	return buildSPFrame(0x01, cmdBoardMembers, body)
}

// DecodeBoardMembers decodes a 0x122C response: 20-byte bitmap echo + 4-byte
// total + 2-byte row_count, then rows of 68-byte fixed header (market(2) +
// code(22 GBK) + name(44 GBK)) followed by 4 bytes per active bitmap field.
func DecodeBoardMembers(body []byte) ([]HQBoardMember, error) {
	if len(body) < 26 {
		return nil, fmt.Errorf("decode TDX SP board members: body too short: %d", len(body))
	}
	activeBits := spActiveBits(body[:20])
	rowCount := int(binary.LittleEndian.Uint16(body[24:26]))
	rowLen := 68 + 4*len(activeBits)
	rest := body[26:]
	if len(activeBits) == 0 {
		return nil, fmt.Errorf("decode TDX SP board members: no active bitmap fields")
	}
	if len(rest) < rowCount*rowLen {
		rowCount = len(rest) / rowLen
	}
	items := make([]HQBoardMember, 0, rowCount)
	for i := 0; i < rowCount; i++ {
		row := rest[i*rowLen : (i+1)*rowLen]
		marketCode := binary.LittleEndian.Uint16(row[0:2])
		item := HQBoardMember{
			Market:     quotesListMarketName(marketCode),
			MarketCode: marketCode,
			Symbol:     decodeSecurityName(row[2:24]),
			Name:       decodeSecurityName(row[24:68]),
			Fields:     make(map[string]float64, len(activeBits)),
			ActiveBits: activeBits,
		}
		offset := 68
		for _, bit := range activeBits {
			raw := binary.LittleEndian.Uint32(row[offset : offset+4])
			offset += 4
			item.Fields[spBoardFieldName(bit)] = spBoardFieldValue(bit, raw)
		}
		items = append(items, item)
	}
	return items, nil
}

func spActiveBits(bitmap []byte) []int {
	var bits []int
	for byteIdx := 0; byteIdx < len(bitmap); byteIdx++ {
		b := bitmap[byteIdx]
		for bitIdx := 0; bitIdx < 8; bitIdx++ {
			if b&(1<<uint(bitIdx)) != 0 {
				bits = append(bits, byteIdx*8+bitIdx)
			}
		}
	}
	return bits
}

// spBoardFieldValue reinterprets a raw uint32 per the field's numeric kind.
func spBoardFieldValue(bit int, raw uint32) float64 {
	switch bit {
	case 0x05, 0x08, 0x09, 0x13, 0x14, 0x18, 0x19, 0x1a, 0x1c, 0x1f, 0x23, 0x2b, 0x2c, 0x59:
		return float64(raw) // uint32 fields
	case 0x5c:
		return float64(int32(raw)) // signed int32
	default:
		return float64(math.Float32frombits(raw)) // float32 fields
	}
}

var spBoardFieldNames = map[int]string{
	0x00: "pre_close", 0x01: "open", 0x02: "high", 0x03: "low", 0x04: "close",
	0x05: "volume", 0x06: "vol_ratio", 0x07: "amount", 0x08: "inside_vol", 0x09: "outside_vol",
	0x0a: "total_shares", 0x0b: "float_shares", 0x0c: "eps", 0x0d: "net_assets",
	0x0f: "total_mcap_ab", 0x10: "pe_dynamic", 0x11: "bid", 0x12: "ask",
	0x13: "server_date", 0x14: "server_time", 0x18: "bid_vol", 0x19: "ask_vol",
	0x1a: "last_vol", 0x1b: "turnover", 0x1c: "industry", 0x1f: "decimal_point",
	0x20: "buy_price_limit", 0x21: "sell_price_limit", 0x23: "lot_size", 0x25: "speed_pct",
	0x26: "avg_price", 0x2b: "kcb_flag", 0x2c: "bj_flag", 0x30: "pe_ttm", 0x31: "pe_static",
	0x3b: "change_20d", 0x3c: "ytd_pct", 0x44: "change_60d", 0x45: "change_5d",
	0x46: "change_10d", 0x59: "activity", 0x5c: "consecutive_up",
}

func spBoardFieldName(bit int) string {
	if name, ok := spBoardFieldNames[bit]; ok {
		return name
	}
	return fmt.Sprintf("bit_0x%02x", bit)
}

// ExchangeBoardCode converts a visual board symbol to the internal SP board code.
func ExchangeBoardCode(symbol string) uint32 {
	n, _ := strconv.Atoi(symbol)
	switch {
	case len(symbol) > 2 && symbol[:2] == "US":
		v, _ := strconv.Atoi(symbol[2:])
		return 30000 + uint32(v)
	case len(symbol) > 2 && symbol[:2] == "HK":
		v, _ := strconv.Atoi(symbol[2:])
		return 20000 + uint32(v)
	case len(symbol) == 6 && symbol[:3] == "000":
		return 31000 + uint32(n)
	case len(symbol) == 6 && symbol[:3] == "399":
		return uint32(n) - 399000 + 30000
	case len(symbol) == 6 && symbol[:3] == "899":
		return uint32(n) - 899000 + 32000
	case len(symbol) == 6 && symbol[:2] == "88":
		return uint32(n) - 880000 + 20000
	default:
		return uint32(n)
	}
}

// ---- frame builders / reader (ported from millken proto.go / handshake.go) ----

func pingPacket(seq uint16) []byte {
	buf := make([]byte, 12)
	buf[0] = 0x0c
	buf[1] = 0x00
	binary.BigEndian.PutUint16(buf[2:4], 0x0000)
	binary.LittleEndian.PutUint16(buf[4:6], seq)
	binary.LittleEndian.PutUint16(buf[6:8], 0x0002)
	binary.LittleEndian.PutUint16(buf[8:10], 0x0002)
	binary.LittleEndian.PutUint16(buf[10:12], cmdSPPing)
	return buf
}

func connectAuthPacket(seq uint16) []byte {
	p := make([]byte, 13)
	p[0] = 0x0c
	p[1] = 0x02
	binary.BigEndian.PutUint16(p[2:4], 0x1894)
	binary.BigEndian.PutUint16(p[4:6], seq)
	binary.LittleEndian.PutUint16(p[6:8], 0x0003)
	binary.LittleEndian.PutUint16(p[8:10], 0x0003)
	binary.LittleEndian.PutUint16(p[10:12], cmdSPConnectAuth)
	p[12] = 0x01
	return p
}

func stage2Packet(seq uint16) []byte {
	payload := []byte{
		0x74, 0x64, 0x78, 0x6c, 0x65, 0x76, 0x65, 0x6c,
		0x00, 0x00, 0x00, 0xe1, 0x7a, 0xf4, 0x40, 0x4c,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
	}
	p := make([]byte, 12+len(payload))
	p[0] = 0x0c
	p[1] = 0x03
	binary.BigEndian.PutUint16(p[2:4], 0x1899)
	binary.BigEndian.PutUint16(p[4:6], seq)
	binary.LittleEndian.PutUint16(p[6:8], uint16(len(payload)+2))
	binary.LittleEndian.PutUint16(p[8:10], uint16(len(payload)+2))
	binary.LittleEndian.PutUint16(p[10:12], cmdSPStage2)
	copy(p[12:], payload)
	return p
}

// buildSPFrame builds a head=0x01 SP frame: [head][cust4][ctrl][len][len][msgID][body].
func buildSPFrame(head byte, msgID uint16, body []byte) []byte {
	inner := make([]byte, 2+len(body))
	binary.LittleEndian.PutUint16(inner[0:2], msgID)
	copy(inner[2:], body)
	header := make([]byte, 10)
	header[0] = head
	header[5] = 1
	binary.LittleEndian.PutUint16(header[6:8], uint16(len(inner)))
	binary.LittleEndian.PutUint16(header[8:10], uint16(len(inner)))
	return append(header, inner...)
}

// readSPFrame reads one frame and returns its cmd id and decoded body. It
// handles the standard 0xB1CB7400 response frame, 0x0c/0x0b direct frames, and
// 0x01 SP frames.
func readSPFrame(r *bufio.Reader) (uint16, []byte, error) {
	magic, err := r.Peek(4)
	if err != nil {
		return 0, nil, err
	}
	switch {
	case bytes.Equal(magic, []byte{0xb1, 0xcb, 0x74, 0x00}):
		header := make([]byte, 16)
		if _, err := io.ReadFull(r, header); err != nil {
			return 0, nil, err
		}
		cmd := binary.LittleEndian.Uint16(header[10:12])
		zipLen := int(binary.LittleEndian.Uint16(header[12:14]))
		rawLen := int(binary.LittleEndian.Uint16(header[14:16]))
		payloadLen := zipLen
		if payloadLen == 0 {
			payloadLen = rawLen
		}
		payload := make([]byte, payloadLen)
		if payloadLen > 0 {
			if _, err := io.ReadFull(r, payload); err != nil {
				return 0, nil, err
			}
		}
		if zipLen > 0 && zipLen != rawLen {
			if decoded, err := spInflate(payload); err == nil {
				payload = decoded
			}
		}
		return cmd, payload, nil
	case magic[0] == 0x0c || magic[0] == 0x0b:
		head, err := r.Peek(12)
		if err != nil {
			return 0, nil, err
		}
		bodyLen := int(binary.LittleEndian.Uint16(head[8:10]))
		frame := make([]byte, 12+bodyLen)
		if _, err := io.ReadFull(r, frame); err != nil {
			return 0, nil, err
		}
		return binary.LittleEndian.Uint16(frame[10:12]), frame[12:], nil
	case magic[0] == 0x01:
		head, err := r.Peek(10)
		if err != nil {
			return 0, nil, err
		}
		bodyLen := int(binary.LittleEndian.Uint16(head[6:8]))
		frame := make([]byte, 10+bodyLen)
		if _, err := io.ReadFull(r, frame); err != nil {
			return 0, nil, err
		}
		if len(frame) < 12 {
			return 0, nil, fmt.Errorf("SP frame too short")
		}
		return binary.LittleEndian.Uint16(frame[10:12]), frame[12:], nil
	default:
		_, _ = r.Discard(1)
		return readSPFrame(r)
	}
}

func spInflate(data []byte) ([]byte, error) {
	zr, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}
