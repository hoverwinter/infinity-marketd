package eastmoney

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

const maxCatalogPages = 50
const catalogPageSize = 100

var boardCode = regexp.MustCompile(`^BK[0-9]{4,6}$`)
var _ marketdata.BoardProvider = (*Client)(nil)

func (*Client) BoardKinds() []string { return []string{"industry", "concept"} }

func categoryFilter(kind string) (string, error) {
	switch kind {
	case "industry":
		return "m:90 t:2 f:!50", nil
	case "concept":
		return "m:90 t:3 f:!50", nil
	default:
		return "", fmt.Errorf("%w: Eastmoney board kind must be industry or concept", marketdata.ErrUnsupported)
	}
}

type catalogItem struct {
	Code   string `json:"f12"`
	Market *int   `json:"f13"`
	Name   string `json:"f14"`
}

func (c *Client) Boards(ctx context.Context, kind string) (marketdata.BoardsResult, error) {
	kind = strings.TrimSpace(kind)
	filter, err := categoryFilter(kind)
	if err != nil {
		return marketdata.BoardsResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	params := url.Values{"pn": {"1"}, "pz": {strconv.Itoa(catalogPageSize)}, "po": {"0"}, "np": {"1"}, "ut": {"bd1d9ddb04089700cf9c27f6f7426281"}, "fltt": {"2"}, "invt": {"2"}, "fid": {"f12"}, "fs": {filter}, "fields": {"f12,f13,f14"}}
	rows := []marketdata.Board{}
	seen := map[string]bool{}
	total, pageSize := -1, 0
	for page := 1; page <= maxCatalogPages; page++ {
		params.Set("pn", strconv.Itoa(page))
		var data struct {
			Total *int           `json:"total"`
			Diff  *[]catalogItem `json:"diff"`
		}
		if err := c.get(ctx, c.quoteURL, "/api/qt/clist/get", params, &data); err != nil {
			return marketdata.BoardsResult{}, err
		}
		if data.Total == nil || *data.Total < 0 || data.Diff == nil || len(*data.Diff) > catalogPageSize {
			return marketdata.BoardsResult{}, fmt.Errorf("%w: malformed Eastmoney catalog page", marketdata.ErrPayload)
		}
		if page == 1 {
			total, pageSize = *data.Total, len(*data.Diff)
			if total > 0 && pageSize == 0 {
				return marketdata.BoardsResult{}, fmt.Errorf("%w: empty first catalog page with positive total", marketdata.ErrPayload)
			}
			if pageSize > 0 && (total > pageSize*maxCatalogPages) {
				return marketdata.BoardsResult{}, fmt.Errorf("%w: Eastmoney catalog requires more than %d pages", marketdata.ErrLimit, maxCatalogPages)
			}
		} else if *data.Total != total {
			return marketdata.BoardsResult{}, fmt.Errorf("%w: Eastmoney catalog total changed during pagination", marketdata.ErrPayload)
		}
		expected := min(pageSize, total-len(rows))
		if len(*data.Diff) != expected {
			return marketdata.BoardsResult{}, fmt.Errorf("%w: incomplete Eastmoney catalog page %d", marketdata.ErrPayload, page)
		}
		for _, item := range *data.Diff {
			name := strings.TrimSpace(item.Name)
			if !boardCode.MatchString(item.Code) || item.Market == nil || *item.Market != 90 || name == "" || name == "-" || seen[item.Code] {
				return marketdata.BoardsResult{}, fmt.Errorf("%w: invalid or repeated Eastmoney board identity", marketdata.ErrPayload)
			}
			seen[item.Code] = true
			rows = append(rows, marketdata.Board{Kind: kind, Code: item.Code, Name: name})
		}
		if len(rows) == total {
			sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
			return marketdata.BoardsResult{Provider: c.ID(), Kind: kind, Scope: "current_provider_catalog", Boards: rows}, nil
		}
	}
	return marketdata.BoardsResult{}, fmt.Errorf("%w: Eastmoney catalog scan incomplete", marketdata.ErrLimit)
}

func (c *Client) ResolveBoard(ctx context.Context, kind, code string) (marketdata.BoardResult, error) {
	code = strings.TrimSpace(code)
	if !boardCode.MatchString(code) {
		return marketdata.BoardResult{}, fmt.Errorf("%w: Eastmoney board code must be BK followed by 4-6 digits", marketdata.ErrInvalid)
	}
	result, err := c.Boards(ctx, kind)
	if err != nil {
		return marketdata.BoardResult{}, err
	}
	for _, board := range result.Boards {
		if board.Code == code {
			board.Instrument = &marketdata.Instrument{Kind: "index", Market: "board", Symbol: code}
			return marketdata.BoardResult{Provider: c.ID(), Board: board}, nil
		}
	}
	return marketdata.BoardResult{}, fmt.Errorf("%w: Eastmoney %s board %s", marketdata.ErrNotFound, result.Kind, code)
}
