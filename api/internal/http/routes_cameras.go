package httpx

import (
	"net/http"
)

func (s *server) listCameras(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items":   []any{},
		"message": "Camera integration is planned. Contact support for Rhombus/ONVIF integration status.",
	})
}

func (s *server) createCamera(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "Camera integration is not yet available.")
}

func (s *server) getCamera(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "Camera integration is not yet available.")
}

func (s *server) deleteCamera(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "Camera integration is not yet available.")
}
