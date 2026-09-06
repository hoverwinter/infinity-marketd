package ths

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/marketdata"
)

var (
	sixDigits   = regexp.MustCompile(`^[0-9]{6}$`)
	divTag      = regexp.MustCompile(`(?is)</?div\b[^>]*>`)
	attributes  = regexp.MustCompile(`(?is)([a-z_:][a-z0-9_:.-]*)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	anchor      = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a\s*>`)
	input       = regexp.MustCompile(`(?is)<input\b[^>]*>`)
	tags        = regexp.MustCompile(`(?s)<[^>]*>`)
	boardHeader = regexp.MustCompile(`(?is)<h3\b[^>]*>\s*([^<]+)<span\b[^>]*>\s*([0-9]{6})\s*</span>\s*</h3>`)
)

func (*Client) BoardKinds() []string { return []string{"industry", "concept"} }

func boardPath(kind string) (string, error) {
	switch kind {
	case "industry":
		return "thshy", nil
	case "concept":
		return "gn", nil
	default:
		return "", fmt.Errorf("%w: THS board kind must be industry or concept", marketdata.ErrUnsupported)
	}
}

func (c *Client) Boards(ctx context.Context, kind string) (marketdata.BoardsResult, error) {
	kind = strings.TrimSpace(kind)
	path, err := boardPath(kind)
	if err != nil {
		return marketdata.BoardsResult{}, err
	}
	seed := "881270"
	if kind == "concept" {
		seed = "301558"
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	page, err := c.page(ctx, "/"+path+"/detail/code/"+seed+"/")
	if err != nil {
		return marketdata.BoardsResult{}, err
	}
	boards, err := parseBoards(page, kind)
	if err != nil {
		return marketdata.BoardsResult{}, err
	}
	return marketdata.BoardsResult{Provider: c.ID(), Kind: kind, Scope: "current_page_catalog", Boards: boards}, nil
}

func (c *Client) ResolveBoard(ctx context.Context, kind, code string) (marketdata.BoardResult, error) {
	kind, code = strings.TrimSpace(kind), strings.TrimSpace(code)
	path, err := boardPath(kind)
	if err != nil {
		return marketdata.BoardResult{}, err
	}
	if !sixDigits.MatchString(code) {
		return marketdata.BoardResult{}, fmt.Errorf("%w: THS board code must be six digits", marketdata.ErrInvalid)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	page, err := c.page(ctx, "/"+path+"/detail/code/"+code+"/")
	if err != nil {
		return marketdata.BoardResult{}, err
	}
	board, err := parseBoard(page, kind, code)
	if err != nil {
		return marketdata.BoardResult{}, err
	}
	return marketdata.BoardResult{Provider: c.ID(), Board: board}, nil
}

func attr(tag, key string) string {
	for _, m := range attributes.FindAllStringSubmatch(tag, -1) {
		if strings.EqualFold(m[1], key) {
			return html.UnescapeString(m[2] + m[3] + m[4])
		}
	}
	return ""
}

// Only the specific THS div subtree is needed. Balance nested divs rather than
// matching arbitrary nested HTML with one regex or scraping unrelated anchors.
func divContent(page, class string) (string, error) {
	start, depth := -1, 0
	for _, loc := range divTag.FindAllStringIndex(page, -1) {
		tag := page[loc[0]:loc[1]]
		closing := strings.HasPrefix(tag, "</")
		if start < 0 {
			if closing {
				continue
			}
			for _, c := range strings.Fields(attr(tag, "class")) {
				if c == class {
					start, depth = loc[1], 1
					break
				}
			}
			continue
		}
		if closing {
			depth--
		} else {
			depth++
		}
		if depth == 0 {
			return page[start:loc[0]], nil
		}
	}
	return "", fmt.Errorf("%w: THS %s section missing or incomplete (possibly a challenge page)", marketdata.ErrPayload, class)
}

func parseBoards(page, kind string) ([]marketdata.Board, error) {
	path, err := boardPath(kind)
	if err != nil {
		return nil, err
	}
	section, err := divContent(page, "cate_inner")
	if err != nil {
		return nil, err
	}
	pattern := regexp.MustCompile(`/` + path + `/detail/code/([0-9]{6})/?$`)
	seen := map[string]marketdata.Board{}
	for _, m := range anchor.FindAllStringSubmatch(section, -1) {
		id := pattern.FindStringSubmatch(attr(m[1], "href"))
		if id == nil {
			continue
		}
		name := strings.TrimSpace(html.UnescapeString(tags.ReplaceAllString(m[2], "")))
		if name == "" {
			return nil, fmt.Errorf("%w: empty THS board name", marketdata.ErrPayload)
		}
		b := marketdata.Board{Kind: kind, Code: id[1], Name: name}
		if prior, ok := seen[b.Code]; ok && prior.Name != b.Name {
			return nil, fmt.Errorf("%w: conflicting THS board %s", marketdata.ErrPayload, b.Code)
		}
		seen[b.Code] = b
	}
	if len(seen) == 0 {
		return nil, fmt.Errorf("%w: empty THS board catalog", marketdata.ErrPayload)
	}
	rows := make([]marketdata.Board, 0, len(seen))
	for _, b := range seen {
		rows = append(rows, b)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
	return rows, nil
}

func parseBoard(page, kind, code string) (marketdata.Board, error) {
	boards, err := parseBoards(page, kind)
	if err != nil {
		return marketdata.Board{}, err
	}
	var board marketdata.Board
	for _, b := range boards {
		if b.Code == code {
			board = b
			break
		}
	}
	if board.Code == "" {
		return board, fmt.Errorf("%w: THS %s board %s absent from page catalog", marketdata.ErrNotFound, kind, code)
	}
	symbol, count := "", 0
	for _, tag := range input.FindAllString(page, -1) {
		if attr(tag, "id") == "clid" {
			symbol = attr(tag, "value")
			count++
		}
	}
	hq, err := divContent(page, "board-hq")
	if err != nil {
		return marketdata.Board{}, err
	}
	header := boardHeader.FindStringSubmatch(hq)
	if count != 1 || !sixDigits.MatchString(symbol) || header == nil || header[2] != symbol || strings.TrimSpace(html.UnescapeString(header[1])) != board.Name || (kind == "industry" && symbol != code) {
		return marketdata.Board{}, fmt.Errorf("%w: THS board page identity mismatch", marketdata.ErrPayload)
	}
	board.Instrument = &marketdata.Instrument{Kind: "index", Market: "board", Symbol: symbol}
	return board, nil
}
