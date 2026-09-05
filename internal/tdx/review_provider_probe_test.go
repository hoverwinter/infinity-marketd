package tdx

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestReviewProviderIdentityProbe(t *testing.T) {
	server := os.Getenv("MARKETD_REVIEW_TDX_PROBE")
	if server == "" {
		t.Skip("set MARKETD_REVIEW_TDX_PROBE to probe public TDX identities")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rows, err := FetchSecurityList(ctx, "sh", QuoteClientOptions{Servers: []string{server}, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if strings.Contains(row.Name, "昨") || strings.Contains(row.Name, "涨停") || row.Symbol == "880005" || row.Symbol == "880006" {
			t.Logf("%+v", row)
		}
	}
}
