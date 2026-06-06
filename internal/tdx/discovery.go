package tdx

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Period string

const (
	PeriodDay Period = "1d"
	Period1m  Period = "1m"
	Period5m  Period = "5m"
)

func DiscoverFile(root string, period Period, market string, code string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("tdx root is required")
	}
	if market == "" {
		market = InferMarketFromCode(code)
	}
	root = filepath.Clean(root)

	var candidates []string
	switch period {
	case PeriodDay:
		candidates = []string{
			filepath.Join(root, "vipdoc", market, "lday", market+code+".day"),
			filepath.Join(root, market, "lday", market+code+".day"),
			filepath.Join(root, market+code+".day"),
			filepath.Join(root, code+".day"),
		}
	case Period1m:
		candidates = []string{
			filepath.Join(root, "vipdoc", market, "minline", market+code+".lc1"),
			filepath.Join(root, "vipdoc", market, "minline", market+code+".1"),
			filepath.Join(root, market, "minline", market+code+".lc1"),
			filepath.Join(root, market, "minline", market+code+".1"),
			filepath.Join(root, market+code+".lc1"),
			filepath.Join(root, market+code+".1"),
		}
	case Period5m:
		candidates = []string{
			filepath.Join(root, "vipdoc", market, "fzline", market+code+".lc5"),
			filepath.Join(root, "vipdoc", market, "fzline", market+code+".5"),
			filepath.Join(root, market, "fzline", market+code+".lc5"),
			filepath.Join(root, market, "fzline", market+code+".5"),
			filepath.Join(root, market+code+".lc5"),
			filepath.Join(root, market+code+".5"),
		}
	default:
		return "", fmt.Errorf("unsupported period %q", period)
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no %s file found for %s%s under %s", period, market, code, root)
}

func DiscoverFiles(root string, period Period, market string) ([]string, error) {
	if root == "" {
		return nil, fmt.Errorf("tdx root is required")
	}
	root = filepath.Clean(root)
	market = strings.ToLower(strings.TrimSpace(market))
	if market != "" && market != "sh" && market != "sz" && market != "bj" {
		return nil, fmt.Errorf("unsupported market %q", market)
	}

	allowedExts, err := periodExtensions(period)
	if err != nil {
		return nil, err
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !allowedExts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		fileMarket, _, err := ParseMarketSymbol(path, market, "")
		if err != nil {
			return nil
		}
		if market != "" && fileMarket != market {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return nil, fmt.Errorf("no %s files found under %s", period, root)
	}
	return files, nil
}

func periodExtensions(period Period) (map[string]bool, error) {
	switch period {
	case PeriodDay:
		return map[string]bool{".day": true}, nil
	case Period1m:
		return map[string]bool{".lc1": true, ".1": true}, nil
	case Period5m:
		return map[string]bool{".lc5": true, ".5": true}, nil
	default:
		return nil, fmt.Errorf("unsupported period %q", period)
	}
}
