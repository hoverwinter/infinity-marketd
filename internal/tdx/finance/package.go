package finance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

func ParseFinancialPackage(path string, loc *time.Location, dictionary map[uint16]model.FinancialItemDictionaryEntry) (FinancialZipResult, error) {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return ParseFinancialZip(path, loc, dictionary)
	case gpcwDATPattern.MatchString(lower):
		raw, err := os.ReadFile(path)
		if err != nil {
			return FinancialZipResult{}, err
		}
		parsed := ParseFinancialDAT(raw, path, loc, dictionary)
		return FinancialZipResult{
			Entries:         []FinancialZipEntry{{Name: base, InputPath: path, Rows: parsed.Rows, Issues: parsed.Issues}},
			FilesDiscovered: 1,
			Format:          parsed.Format,
		}, nil
	default:
		return FinancialZipResult{}, fmt.Errorf("unsupported financial package %s: expected gpcwYYYYMMDD.zip, gpcwYYYYMMDD.dat, or tdxfin.zip", path)
	}
}
