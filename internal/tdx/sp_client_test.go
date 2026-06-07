package tdx

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"math"
	"net"
	"testing"
	"time"
)

func TestExchangeBoardCode(t *testing.T) {
	cases := map[string]uint32{
		"880472": 472 + 20000, // 88x -> 880000 offset + 20000
		"399001": 1 + 30000,   // 399x
		"000300": 31000 + 300, // 000x
		"600519": 600519,      // plain
	}
	for in, want := range cases {
		if got := ExchangeBoardCode(in); got != want {
			t.Fatalf("ExchangeBoardCode(%q)=%d want %d", in, got, want)
		}
	}
}

func TestBuildSPFrameStructure(t *testing.T) {
	frame := buildSPFrame(0x01, cmdBoardMembers, []byte{0xaa, 0xbb})
	if frame[0] != 0x01 || frame[5] != 1 {
		t.Fatalf("head/control wrong: %x", frame[:6])
	}
	if l := binary.LittleEndian.Uint16(frame[6:8]); l != 4 { // msgID(2)+body(2)
		t.Fatalf("len = %d", l)
	}
	if msg := binary.LittleEndian.Uint16(frame[10:12]); msg != cmdBoardMembers {
		t.Fatalf("msgID = %#x", msg)
	}
	if !bytes.Equal(frame[12:], []byte{0xaa, 0xbb}) {
		t.Fatalf("body = %x", frame[12:])
	}
}

func TestReadSPFrameVariants(t *testing.T) {
	// 0x01 SP frame
	sp := buildSPFrame(0x01, 0x122C, []byte{0x01, 0x02, 0x03})
	// 0xB1CB7400 response frame (cmd 0x0015, empty body)
	resp := make([]byte, 16)
	binary.BigEndian.PutUint32(resp[0:4], 0xB1CB7400)
	binary.LittleEndian.PutUint16(resp[10:12], 0x0015)
	r := bufio.NewReader(bytes.NewReader(append(append([]byte{}, sp...), resp...)))

	cmd, body, err := readSPFrame(r)
	if err != nil || cmd != 0x122C || !bytes.Equal(body, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("sp frame: cmd=%#x body=%x err=%v", cmd, body, err)
	}
	cmd, body, err = readSPFrame(r)
	if err != nil || cmd != 0x0015 || len(body) != 0 {
		t.Fatalf("resp frame: cmd=%#x body=%x err=%v", cmd, body, err)
	}
}

// craftBoardMembersBody builds a 0x122C payload with one row using spBoardBitmap.
func craftBoardMembersBody(t *testing.T) []byte {
	t.Helper()
	activeBits := spActiveBits(spBoardBitmap)
	rowLen := 68 + 4*len(activeBits)

	body := make([]byte, 26)
	copy(body[0:20], spBoardBitmap)
	binary.LittleEndian.PutUint16(body[24:26], 1) // row count

	row := make([]byte, rowLen)
	binary.LittleEndian.PutUint16(row[0:2], 1) // market sh
	copy(row[2:], []byte("600519"))
	copy(row[24:], []byte("浦发银行")) // GBK? stored as raw bytes; decode is best-effort

	idx := func(bit int) int {
		for i, b := range activeBits {
			if b == bit {
				return i
			}
		}
		t.Fatalf("bit 0x%02x not active", bit)
		return -1
	}
	// pre_close (bit 0x00, float32) and volume (bit 0x05, uint32)
	binary.LittleEndian.PutUint32(row[68+4*idx(0x00):], math.Float32bits(12.34))
	binary.LittleEndian.PutUint32(row[68+4*idx(0x05):], 999)

	return append(body, row...)
}

func TestDecodeBoardMembers(t *testing.T) {
	items, err := DecodeBoardMembers(craftBoardMembersBody(t))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 row, got %d", len(items))
	}
	it := items[0]
	if it.Market != "sh" || it.MarketCode != 1 || it.Symbol != "600519" {
		t.Fatalf("identity wrong: %+v", it)
	}
	if it.Fields["pre_close"] != float64(float32(12.34)) {
		t.Fatalf("pre_close = %v", it.Fields["pre_close"])
	}
	if it.Fields["volume"] != 999 {
		t.Fatalf("volume = %v", it.Fields["volume"])
	}
}

// TestSPSessionFullPath drives the bootstrap + login + one board request over a
// scripted in-memory server, proving the handshake loop and pagination wiring.
func TestSPSessionFullPath(t *testing.T) {
	cli, srv := net.Pipe()
	board := craftBoardMembersBody(t)

	go func() {
		defer srv.Close()
		_ = srv.SetDeadline(time.Now().Add(2 * time.Second))
		read := func(n int) bool {
			_, err := io.ReadFull(srv, make([]byte, n))
			return err == nil
		}
		writeResp := func(cmd uint16) {
			h := make([]byte, 16)
			binary.BigEndian.PutUint32(h[0:4], 0xB1CB7400)
			binary.LittleEndian.PutUint16(h[10:12], cmd)
			_, _ = srv.Write(h)
		}
		if !read(12) { // ping
			return
		}
		writeResp(cmdSPPing)
		if !read(13) { // connect-auth
			return
		}
		writeResp(cmdSPConnectAuth)
		if !read(42) { // stage2 (12 + 30 payload)
			return
		}
		writeResp(cmdSPStage2)
		if !read(92) { // sp login (10 + 2 + 80)
			return
		}
		_, _ = srv.Write(buildSPFrame(0x01, cmdSPLogin, nil))
		if !read(55) { // board request (10 + 2 + 43)
			return
		}
		_, _ = srv.Write(buildSPFrame(0x01, cmdBoardMembers, board))
	}()

	session, err := spHandshake(cli, "pipe", 2*time.Second)
	if err != nil {
		t.Fatalf("handshake: %v", err)
	}
	defer session.Close()
	items, err := session.BoardMembers(ExchangeBoardCode("600519"), 0, 1, 0)
	if err != nil {
		t.Fatalf("board members: %v", err)
	}
	if len(items) != 1 || items[0].Symbol != "600519" {
		t.Fatalf("unexpected items: %+v", items)
	}
}
