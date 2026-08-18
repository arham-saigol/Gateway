package web_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"arham-gateway/web"
)

func TestAssetHandler(t *testing.T) {
	handler := web.AssetHandler()

	// Test GET /
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rec.Code)
	}

	// Test GET /non-existent-route (SPA routing fallback to index.html)
	reqSPA := httptest.NewRequest(http.MethodGet, "/dashboard/keys", nil)
	recSPA := httptest.NewRecorder()
	handler.ServeHTTP(recSPA, reqSPA)

	if recSPA.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SPA fallback, got %d", recSPA.Code)
	}
	if recSPA.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("expected text/html charset=utf-8, got %s", recSPA.Header().Get("Content-Type"))
	}
}
