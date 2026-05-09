package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

const maxBodyBytes = 1 << 20

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    string `json:"code"`
	Status  string `json:"status"`
}

func writeError(w http.ResponseWriter, status int, message string) {
	nextMessage := strings.TrimSpace(message)
	if nextMessage == "" {
		nextMessage = http.StatusText(status)
	}
	writeJSON(w, status, errorResponse{
		Error:   nextMessage,
		Message: nextMessage,
		Code:    httpStatusErrorCode(status),
		Status:  strconv.Itoa(status),
	})
}

func writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "internal error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

// parsePagination extracts offset/limit from query params with defaults.
func parsePagination(r *http.Request, total int) (offset, limit int) {
	offset, _ = strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return offset, limit
}

func httpStatusErrorCode(status int) string {
	text := strings.ToLower(strings.TrimSpace(http.StatusText(status)))
	if text == "" {
		return "http_error"
	}
	replacer := strings.NewReplacer(" ", "_", "-", "_")
	return replacer.Replace(text)
}
