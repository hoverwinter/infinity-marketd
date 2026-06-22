package tdx

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestServerCandidateLists(t *testing.T) {
	if len(SPServerCandidates()) == 0 {
		t.Fatal("expected SP candidates")
	}
	if len(FundServerCandidates()) == 0 {
		t.Fatal("expected fund candidates")
	}
}

func TestProbeSPServers(t *testing.T) {
	ln := startSPProbeServer(t)
	defer ln.Close()

	results := ProbeSPServers(nil, []string{ln.Addr().String()}, time.Second)
	if len(results) != 1 || !results[0].Success || !results[0].Preferred {
		t.Fatalf("unexpected results: %+v", results)
	}
	if BestSPServer(results) != ln.Addr().String() {
		t.Fatalf("best SP = %q", BestSPServer(results))
	}
}

func TestProbeFundServers(t *testing.T) {
	ln := startFundProbeServer(t)
	defer ln.Close()

	results := ProbeFundServers(nil, []string{ln.Addr().String()}, time.Second)
	if len(results) != 1 || !results[0].Success || !results[0].Preferred {
		t.Fatalf("unexpected results: %+v", results)
	}
	if BestFundServer(results) != ln.Addr().String() {
		t.Fatalf("best fund = %q", BestFundServer(results))
	}
}

func TestProbeProtocolFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	results := ProbeSPServers(nil, []string{addr}, 50*time.Millisecond)
	if len(results) != 1 || results[0].Success || results[0].Error == "" {
		t.Fatalf("expected failure result, got %+v", results)
	}
}

func startSPProbeServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		read := func(n int) bool {
			_, err := io.ReadFull(conn, make([]byte, n))
			return err == nil
		}
		writeResp := func(cmd uint16) {
			h := make([]byte, 16)
			binary.BigEndian.PutUint32(h[0:4], 0xB1CB7400)
			binary.LittleEndian.PutUint16(h[10:12], cmd)
			_, _ = conn.Write(h)
		}
		if !read(12) {
			return
		}
		writeResp(cmdSPPing)
		if !read(13) {
			return
		}
		writeResp(cmdSPConnectAuth)
		if !read(42) {
			return
		}
		writeResp(cmdSPStage2)
		if !read(92) {
			return
		}
		_, _ = conn.Write(buildSPFrame(0x01, cmdSPLogin, nil))
	}()
	return ln
}

func startFundProbeServer(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		read := func(n int) bool {
			_, err := io.ReadFull(conn, make([]byte, n))
			return err == nil
		}
		if !read(92) {
			return
		}
		_, _ = conn.Write(buildSPFrame(0x01, cmdSPLogin, nil))
		if !read(12) {
			return
		}
		_, _ = conn.Write(buildSPFrame(0x01, cmdFundBootstrap, nil))
	}()
	return ln
}
