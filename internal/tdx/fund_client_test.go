package tdx

import (
	"bufio"
	"encoding/binary"
	"io"
	"math"
	"net"
	"testing"
	"time"
)

func TestFundNormalizeCode(t *testing.T) {
	if c, err := fundNormalizeCode("sh510050"); err != nil || c != "510050" {
		t.Fatalf("sh510050 -> %q %v", c, err)
	}
	if c, err := fundNormalizeCode("159915"); err != nil || c != "159915" {
		t.Fatalf("159915 -> %q %v", c, err)
	}
	if _, err := fundNormalizeCode("920001"); err == nil {
		t.Fatalf("expected bj rejection")
	}
	if _, err := fundNormalizeCode("abc"); err == nil {
		t.Fatalf("expected invalid code error")
	}
}

func TestDecodeFundDetail(t *testing.T) {
	body := make([]byte, 38)
	body[0] = 0x21
	copy(body[1:], []byte("159915"))
	binary.LittleEndian.PutUint16(body[36:38], 2)
	for _, id := range []uint32{100, 200} {
		row := make([]byte, 16)
		binary.LittleEndian.PutUint32(row[0:4], id)
		for j := 0; j < 6; j++ {
			binary.LittleEndian.PutUint16(row[4+j*2:], uint16(int(id)+j))
		}
		body = append(body, row...)
	}
	d, err := DecodeFundDetail(body)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if d.Category != 0x21 || d.Code != "159915" || len(d.Items) != 2 {
		t.Fatalf("detail wrong: %+v", d)
	}
	if d.Items[1].ID != 200 || d.Items[1].Values[0] != 200 || d.Items[1].Values[5] != 205 {
		t.Fatalf("item wrong: %+v", d.Items[1])
	}
}

func TestDecodeFundKlines(t *testing.T) {
	body := make([]byte, 42)
	binary.LittleEndian.PutUint16(body[24:26], 0) // use passed period
	binary.LittleEndian.PutUint16(body[40:42], 1)
	rec := make([]byte, 32)
	binary.LittleEndian.PutUint32(rec[0:4], 20260605)
	binary.LittleEndian.PutUint32(rec[4:8], math.Float32bits(1.5))
	binary.LittleEndian.PutUint32(rec[8:12], math.Float32bits(1.6))
	binary.LittleEndian.PutUint32(rec[12:16], math.Float32bits(1.4))
	binary.LittleEndian.PutUint32(rec[16:20], math.Float32bits(1.55))
	binary.LittleEndian.PutUint32(rec[20:24], math.Float32bits(1000))
	binary.LittleEndian.PutUint32(rec[24:28], 500)
	body = append(body, rec...)

	bars, err := DecodeFundKlines(body, fundPeriodDay)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bars) != 1 {
		t.Fatalf("expected 1 bar, got %d", len(bars))
	}
	b := bars[0]
	if b.Time != "2026-06-05" || b.Volume != 500 {
		t.Fatalf("bar time/volume wrong: %+v", b)
	}
	if b.Open != float64(float32(1.5)) || b.Close != float64(float32(1.55)) || b.Amount != float64(float32(1000)) {
		t.Fatalf("bar prices wrong: %+v", b)
	}
}

func TestFundSessionFullPath(t *testing.T) {
	cli, srv := net.Pipe()

	detailBody := make([]byte, 38)
	detailBody[0] = 0x21
	copy(detailBody[1:], []byte("159915"))
	binary.LittleEndian.PutUint16(detailBody[36:38], 0) // zero items

	go func() {
		defer srv.Close()
		_ = srv.SetDeadline(time.Now().Add(2 * time.Second))
		read := func(n int) bool { _, err := io.ReadFull(srv, make([]byte, n)); return err == nil }
		if !read(92) { // sp login (10+2+80)
			return
		}
		_, _ = srv.Write(buildSPFrame(0x01, cmdSPLogin, nil))
		if !read(12) { // fund bootstrap (10+2)
			return
		}
		_, _ = srv.Write(buildSPFrame(0x01, cmdFundBootstrap, nil))
		if !read(50) { // fund detail request (10+2+38)
			return
		}
		_, _ = srv.Write(buildSPFrame(0x01, cmdFundDetail, detailBody))
	}()

	s := &SPSession{server: "pipe", conn: cli, r: bufio.NewReader(cli), timeout: 2 * time.Second}
	// SP login + fund bootstrap inline (mirrors OpenFundSession over the pipe).
	if _, err := s.exchange(buildSPFrame(0x01, cmdSPLogin, spLoginBody), cmdSPLogin); err != nil {
		t.Fatalf("sp login: %v", err)
	}
	if _, err := s.exchange(buildSPFrame(0x01, cmdFundBootstrap, nil), cmdFundBootstrap); err != nil {
		t.Fatalf("fund bootstrap: %v", err)
	}
	d, err := s.FundDetail("159915", 0)
	if err != nil {
		t.Fatalf("fund detail: %v", err)
	}
	if d.Code != "159915" {
		t.Fatalf("unexpected detail: %+v", d)
	}
}
