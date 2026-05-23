package pdfgen

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConvertHTML_Success(t *testing.T) {
	fakePDF := []byte("%PDF-1.4 fake content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/forms/chromium/convert/html" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if ct == "" || len(ct) < 10 {
			t.Error("expected multipart content-type")
		}
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, _, err := r.FormFile("files")
		if err != nil {
			t.Fatalf("expected files field: %v", err)
		}
		file.Close()
		if r.FormValue("printBackground") != "true" {
			t.Error("expected printBackground=true")
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(fakePDF)
	}))
	defer srv.Close()

	client := NewGotenbergClient(srv.URL, nil)
	result, err := client.ConvertHTML([]byte("<html><body>Hello</body></html>"), DefaultPDFOptions())
	if err != nil {
		t.Fatalf("ConvertHTML error: %v", err)
	}
	if string(result) != string(fakePDF) {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestConvertHTML_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("chromium crashed"))
	}))
	defer srv.Close()

	client := NewGotenbergClient(srv.URL, nil)
	_, err := client.ConvertHTML([]byte("<html></html>"), DefaultPDFOptions())
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
