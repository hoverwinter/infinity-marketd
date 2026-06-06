package querier

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Server struct {
	repo Repository
}

func NewServer(repo Repository) *Server {
	return &Server{repo: repo}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/bars", s.handleBars)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.repo.Health(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(w, http.StatusOK, Health{Status: "ok"})
}

func (s *Server) handleBars(w http.ResponseWriter, r *http.Request) {
	query := barQueryFromRequest(r)
	result, err := s.repo.Bars(r.Context(), query)
	if err != nil {
		writeError(w, statusForError(err), err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func barQueryFromRequest(r *http.Request) BarQuery {
	values := r.URL.Query()
	limit, _ := strconv.Atoi(values.Get("limit"))
	return BarQuery{
		Market: strings.TrimSpace(values.Get("market")),
		Symbol: strings.TrimSpace(values.Get("symbol")),
		Period: strings.TrimSpace(values.Get("period")),
		Since:  strings.TrimSpace(values.Get("since")),
		Until:  strings.TrimSpace(values.Get("until")),
		Limit:  limit,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusForError(err error) int {
	if IsValidationError(err) {
		return http.StatusBadRequest
	}
	return http.StatusInternalServerError
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func IsValidationError(err error) bool {
	var validationErr ValidationError
	return errors.As(err, &validationErr)
}

func validationError(format string, args ...any) error {
	return ValidationError{Message: fmt.Sprintf(format, args...)}
}
