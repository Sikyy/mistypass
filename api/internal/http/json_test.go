package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorUsesStructuredSchema(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	writeError(recorder, http.StatusUnprocessableEntity, "apple wallet cards must be enrolled by resident users")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", recorder.Code)
	}
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Code    string `json:"code"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode structured error response: %v", err)
	}
	if payload.Error != "apple wallet cards must be enrolled by resident users" ||
		payload.Message != payload.Error ||
		payload.Code != "unprocessable_entity" ||
		payload.Status != "422" {
		t.Fatalf("unexpected structured error response: %+v", payload)
	}
}
