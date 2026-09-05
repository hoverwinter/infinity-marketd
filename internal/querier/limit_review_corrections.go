package querier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hoverwinter/infinity-marketd/internal/model"
)

type LimitCorrectionImportResult struct {
	RunID       string               `json:"run_id"`
	Events      uint64               `json:"events"`
	RowsWritten uint64               `json:"rows_written"`
	RowsSkipped uint64               `json:"rows_skipped"`
	Issues      []model.QualityIssue `json:"issues"`
	DryRun      bool                 `json:"dry_run"`
}

type LimitCorrectionImporter func(context.Context, []byte, bool) (LimitCorrectionImportResult, error)

// Only the explicit console write-plane wiring installs this handler.
func (s *Server) WithLimitCorrectionImporter(token string, importer LimitCorrectionImporter) *Server {
	if strings.TrimSpace(token) == "" || importer == nil {
		return s
	}
	tokenHash := sha256.Sum256([]byte(token))
	var writes sync.Mutex
	s.limitCorrectionHandler = func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		providedHash := sha256.Sum256([]byte(provided))
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || subtle.ConstantTimeCompare(providedHash[:], tokenHash[:]) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, fmt.Errorf("review write authentication required"))
			return
		}
		if r.Header.Get("Origin") != "" {
			writeError(w, http.StatusForbidden, fmt.Errorf("use a server-side gateway for correction imports"))
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, fmt.Errorf("application/json is required"))
			return
		}
		dryRun := true
		for key, values := range r.URL.Query() {
			if key != "dry_run" || len(values) != 1 || (values[0] != "true" && values[0] != "false") {
				writeError(w, http.StatusBadRequest, validationError("only dry_run=true|false is accepted"))
				return
			}
			dryRun = values[0] == "true"
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		defer r.Body.Close()
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			var status = http.StatusBadRequest
			if _, ok := err.(*http.MaxBytesError); ok {
				status = http.StatusRequestEntityTooLarge
			}
			writeError(w, status, err)
			return
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil || compact.Len() == 0 || compact.Bytes()[0] != '{' {
			writeError(w, http.StatusBadRequest, validationError("one correction JSON object is required"))
			return
		}
		if !writes.TryLock() {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusConflict, fmt.Errorf("a correction import is in progress"))
			return
		}
		defer writes.Unlock()
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		result, err := importer(ctx, compact.Bytes(), dryRun)
		if result.Issues == nil {
			result.Issues = []model.QualityIssue{}
		}
		if err != nil {
			writeJSON(w, statusForError(err), map[string]any{"error": err.Error(), "result": result})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
	return s
}
