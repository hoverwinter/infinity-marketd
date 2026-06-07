package finance

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DefaultFinancialRemoteBaseURL = "https://data.tdx.com.cn/tdxfin/"

var remoteGPCWZIPPattern = regexp.MustCompile(`^gpcw(\d{8})\.zip$`)

type RemoteFinancialFile struct {
	Filename   string `json:"filename"`
	MD5        string `json:"md5"`
	Size       int64  `json:"size"`
	ReportDate string `json:"report_date,omitempty"`
}

type RemoteFinancialFetchResult struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Skipped  bool   `json:"skipped"`
}

type RemoteClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

func ParseRemoteFinancialManifest(raw []byte) ([]RemoteFinancialFile, error) {
	var out []RemoteFinancialFile
	for lineNo, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) != 3 {
			return nil, fmt.Errorf("manifest line %d must have filename,md5,size", lineNo+1)
		}
		filename := strings.TrimSpace(parts[0])
		if err := ValidateRemoteFinancialFilename(filename); err != nil {
			return nil, fmt.Errorf("manifest line %d filename: %w", lineNo+1, err)
		}
		hash := strings.ToLower(strings.TrimSpace(parts[1]))
		if len(hash) != 32 || !isHex(hash) {
			return nil, fmt.Errorf("manifest line %d has invalid md5 %q", lineNo+1, parts[1])
		}
		size, err := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
		if err != nil || size < 0 {
			return nil, fmt.Errorf("manifest line %d has invalid size %q", lineNo+1, parts[2])
		}
		reportDate := ""
		if matches := remoteGPCWZIPPattern.FindStringSubmatch(filename); matches != nil {
			reportDate = matches[1]
		}
		out = append(out, RemoteFinancialFile{Filename: filename, MD5: hash, Size: size, ReportDate: reportDate})
	}
	return out, nil
}

func ValidateRemoteFinancialFilename(filename string) error {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if filepath.IsAbs(filename) || strings.Contains(filename, "/") || strings.Contains(filename, `\`) || strings.Contains(filename, "..") {
		return fmt.Errorf("unsafe filename %q", filename)
	}
	if !remoteGPCWZIPPattern.MatchString(filename) {
		return fmt.Errorf("filename must match gpcwYYYYMMDD.zip")
	}
	return nil
}

func FindRemoteFinancialFile(files []RemoteFinancialFile, filename string) (RemoteFinancialFile, bool) {
	filename = strings.TrimSpace(filename)
	for _, file := range files {
		if file.Filename == filename {
			return file, true
		}
	}
	return RemoteFinancialFile{}, false
}

func (c RemoteClient) List(ctx context.Context) ([]RemoteFinancialFile, error) {
	raw, err := c.getBytes(ctx, "gpcw.txt")
	if err != nil {
		return nil, err
	}
	return ParseRemoteFinancialManifest(raw)
}

func (c RemoteClient) Fetch(ctx context.Context, file RemoteFinancialFile, dir string) (RemoteFinancialFetchResult, error) {
	if err := ValidateRemoteFinancialFilename(file.Filename); err != nil {
		return RemoteFinancialFetchResult{}, err
	}
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return RemoteFinancialFetchResult{}, err
	}
	target := filepath.Join(dir, file.Filename)
	if file.Size > 0 && file.MD5 != "" {
		if err := VerifyRemoteFinancialFile(target, file); err == nil {
			return RemoteFinancialFetchResult{Filename: file.Filename, Path: target, Bytes: file.Size, Skipped: true}, nil
		}
	}

	downloadURL, err := c.resolveURL(file.Filename)
	if err != nil {
		return RemoteFinancialFetchResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return RemoteFinancialFetchResult{}, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return RemoteFinancialFetchResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RemoteFinancialFetchResult{}, fmt.Errorf("GET %s returned %s", downloadURL, resp.Status)
	}

	tmp, err := os.CreateTemp(dir, "."+file.Filename+".*.tmp")
	if err != nil {
		return RemoteFinancialFetchResult{}, err
	}
	tmpPath := tmp.Name()
	written, copyErr := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpPath)
		return RemoteFinancialFetchResult{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return RemoteFinancialFetchResult{}, closeErr
	}
	if file.Size > 0 || file.MD5 != "" {
		if err := VerifyRemoteFinancialFile(tmpPath, file); err != nil {
			_ = os.Remove(tmpPath)
			return RemoteFinancialFetchResult{}, err
		}
	}
	if err := os.Rename(tmpPath, target); err != nil {
		_ = os.Remove(tmpPath)
		return RemoteFinancialFetchResult{}, err
	}
	return RemoteFinancialFetchResult{Filename: file.Filename, Path: target, Bytes: written}, nil
}

func VerifyRemoteFinancialFile(path string, file RemoteFinancialFile) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if file.Size > 0 && info.Size() != file.Size {
		return fmt.Errorf("%s size %d != manifest size %d", path, info.Size(), file.Size)
	}
	if file.MD5 != "" {
		sum, err := fileMD5(path)
		if err != nil {
			return err
		}
		if sum != strings.ToLower(file.MD5) {
			return fmt.Errorf("%s md5 %s != manifest md5 %s", path, sum, strings.ToLower(file.MD5))
		}
	}
	return nil
}

func (c RemoteClient) getBytes(ctx context.Context, remotePath string) ([]byte, error) {
	rawURL, err := c.resolveURL(remotePath)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s returned %s", rawURL, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func (c RemoteClient) resolveURL(remotePath string) (string, error) {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		base = DefaultFinancialRemoteBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("base URL must include scheme and host")
	}
	ref := &url.URL{Path: strings.TrimLeft(remotePath, "/")}
	return parsed.ResolveReference(ref).String(), nil
}

func (c RemoteClient) client() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func fileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func isHex(value string) bool {
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}
