package securitymaster

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileSourceFetchesNormalizedCSVRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "securities.csv")
	raw := "market,symbol,current_name,status,listing_date,aliases,source\n" +
		"sh,600519,贵州茅台,listed,20010827,茅台|Kweichow,akshare\n" +
		"bj,920001,北证测试,listed,20260610,,mootdx\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := (FileSource{Path: path, Source: SourceFile}).Fetch(context.Background(), []string{"sh"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %#v", rows)
	}
	if rows[0].Market != "sh" || rows[0].Source != "akshare" || len(rows[0].Aliases) != 2 {
		t.Fatalf("row = %#v", rows[0])
	}
}
