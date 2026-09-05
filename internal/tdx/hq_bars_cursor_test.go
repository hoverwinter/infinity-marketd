package tdx

import (
	"encoding/binary"
	"testing"
)

func TestHQBarsCursorAcrossRecords(t *testing.T) {
	for _, index := range []bool{false, true} {
		body := []byte{3, 0}
		for i := 0; i < 3; i++ {
			body = binary.LittleEndian.AppendUint32(body, uint32(20260901+i))
			openDiff := 10
			if i == 0 {
				openDiff = 10000
			}
			for _, v := range []int{openDiff, 100, 200, -100} {
				body = append(body, encodeTDXVarInt(v)...)
			}
			body = append(body, make([]byte, 8)...)
			if index {
				body = binary.LittleEndian.AppendUint16(body, uint16(100+i))
				body = binary.LittleEndian.AppendUint16(body, uint16(200+i))
			}
		}
		req := HQBarsRequest{Category: HQKLineDayAlt, Market: "sh", Symbol: "880005"}
		bars, err := DecodeHQBarsResponse(req, index, body)
		if err != nil {
			t.Fatal(err)
		}
		for i, bar := range bars {
			expectedOpen := float64(10000+i*110) / 1000
			if bar.Day != 1+i || bar.Open != expectedOpen {
				t.Fatalf("index=%v row %d = %+v", index, i, bar)
			}
			if index && (bar.UpCount != uint16(100+i) || bar.DownCount != uint16(200+i)) {
				t.Fatalf("bad counts %+v", bar)
			}
		}
		if _, err := DecodeHQBarsResponse(req, index, body[:len(body)-1]); err == nil {
			t.Fatal("truncated last record accepted")
		}
	}
}
