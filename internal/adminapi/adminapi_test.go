package adminapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"arham-gateway/internal/adminapi"
	"arham-gateway/internal/auth"
	"arham-gateway/internal/config"
	"arham-gateway/internal/crypto"
	"arham-gateway/internal/database"
)

func setupAdminTestEnv(t *testing.T) (*database.DB, []byte, string, http.Handler) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "gateway.db")

	db, err := database.Open(dbPath, 5000)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	_ = db.Migrate()
	_ = db.SeedDefaults()

	masterKey, _ := crypto.GenerateRandomBytes(32)

	// Set admin password
	adminPass := "SuperSecretAdmin123!"
	hashed, _ := auth.HashPassword(adminPass)
	_ = db.SetAdminPasswordHash(hashed)

	cfg := config.DefaultConfig()
	handler := adminapi.NewHandler(db, masterKey, &cfg)

	return db, masterKey, adminPass, handler
}

func loginAdmin(t *testing.T, handler http.Handler, password string) (*http.Cookie, string) {
	loginBody, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		CSRFToken string `json:"csrf_token"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&resp)

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "arham_session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatalf("session cookie not set on login")
	}

	return sessionCookie, resp.CSRFToken
}

func TestAdminAuthLifecycle(t *testing.T) {
	_, _, adminPass, handler := setupAdminTestEnv(t)

	// 1. Wrong password
	badBody, _ := json.Marshal(map[string]string{"password": "WrongPassword"})
	reqBad := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
	recBad := httptest.NewRecorder()
	handler.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 on wrong password, got %d", recBad.Code)
	}

	// 2. Successful login
	cookie, csrf := loginAdmin(t, handler, adminPass)

	// 3. Authenticated /api/auth/me
	reqMe := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqMe.AddCookie(cookie)
	recMe := httptest.NewRecorder()
	handler.ServeHTTP(recMe, reqMe)
	if recMe.Code != http.StatusOK {
		t.Errorf("expected 200 on /api/auth/me, got %d", recMe.Code)
	}

	// 4. Logout
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	reqLogout.AddCookie(cookie)
	reqLogout.Header.Set("X-CSRF-Token", csrf)
	recLogout := httptest.NewRecorder()
	handler.ServeHTTP(recLogout, reqLogout)
	if recLogout.Code != http.StatusOK {
		t.Errorf("expected 200 on logout, got %d", recLogout.Code)
	}

	// 5. Subsequent request with old session should fail
	reqAfter := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqAfter.AddCookie(cookie)
	recAfter := httptest.NewRecorder()
	handler.ServeHTTP(recAfter, reqAfter)
	if recAfter.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 after session revocation, got %d", recAfter.Code)
	}
}

func TestAdminLoginRejectsOversizedBody(t *testing.T) {
	_, _, _, handler := setupAdminTestEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"password":"`+strings.Repeat("x", 5000)+`"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized login body, got %d", rec.Code)
	}
}

func TestProviderKeyAndGatewayKeyManagement(t *testing.T) {
	_, _, adminPass, handler := setupAdminTestEnv(t)
	cookie, csrf := loginAdmin(t, handler, adminPass)

	// 1. Create a Provider Key
	keyBody, _ := json.Marshal(map[string]any{
		"provider_id":                "fireworks",
		"display_name":               "Production Fireworks",
		"secret":                     "fw_test_secret_12345",
		"starting_balance_micro_usd": 6000000,
	})
	reqKey := httptest.NewRequest(http.MethodPost, "/api/providers/keys", bytes.NewReader(keyBody))
	reqKey.AddCookie(cookie)
	reqKey.Header.Set("X-CSRF-Token", csrf)
	recKey := httptest.NewRecorder()
	handler.ServeHTTP(recKey, reqKey)

	if recKey.Code != http.StatusOK {
		t.Fatalf("expected 200 creating provider key, got %d: %s", recKey.Code, recKey.Body.String())
	}

	// Verify key secret is never exposed in response
	if bytes.Contains(recKey.Body.Bytes(), []byte("fw_test_secret_12345")) {
		t.Fatalf("provider key secret leaked in response body!")
	}

	// 2. Create a Gateway Key
	gwBody, _ := json.Marshal(map[string]string{
		"name": "developer-laptop",
	})
	reqGw := httptest.NewRequest(http.MethodPost, "/api/gateway-keys", bytes.NewReader(gwBody))
	reqGw.AddCookie(cookie)
	reqGw.Header.Set("X-CSRF-Token", csrf)
	recGw := httptest.NewRecorder()
	handler.ServeHTTP(recGw, reqGw)

	if recGw.Code != http.StatusOK {
		t.Fatalf("expected 200 creating gateway key, got %d: %s", recGw.Code, recGw.Body.String())
	}

	var gwResp struct {
		ID     string `json:"id"`
		Key    string `json:"key"`
		Prefix string `json:"prefix"`
	}
	if err := json.NewDecoder(recGw.Body).Decode(&gwResp); err != nil {
		t.Fatalf("failed to decode gateway key response: %v", err)
	}

	if !strings.HasPrefix(gwResp.Key, "arham_") {
		t.Errorf("expected generated key to have arham_ prefix, got %s", gwResp.Key)
	}
}

func TestRouteReorderingAndRateUpdates(t *testing.T) {
	_, _, adminPass, handler := setupAdminTestEnv(t)
	cookie, csrf := loginAdmin(t, handler, adminPass)

	// 1. Reorder routes for deepseek-v4-flash
	// Reorder: 1. Baseten, 2. Novita, 3. SiliconFlow, 4. Fireworks
	newOrder := []string{"map-base-flash", "map-nov-flash", "map-sf-flash", "map-fw-flash"}
	orderBody, _ := json.Marshal(map[string]any{
		"mapping_ids": newOrder,
	})
	reqReorder := httptest.NewRequest(http.MethodPost, "/api/models/deepseek-v4-flash/routes", bytes.NewReader(orderBody))
	reqReorder.AddCookie(cookie)
	reqReorder.Header.Set("X-CSRF-Token", csrf)
	recReorder := httptest.NewRecorder()
	handler.ServeHTTP(recReorder, reqReorder)

	if recReorder.Code != http.StatusOK {
		t.Fatalf("failed to reorder routes: %d %s", recReorder.Code, recReorder.Body.String())
	}

	// 2. Update rates for map-base-flash
	ratesBody, _ := json.Marshal(map[string]any{
		"input_rate":  150000,
		"cached_rate": 15000,
		"output_rate": 300000,
	})
	reqRates := httptest.NewRequest(http.MethodPost, "/api/mappings/map-base-flash/rates", bytes.NewReader(ratesBody))
	reqRates.AddCookie(cookie)
	reqRates.Header.Set("X-CSRF-Token", csrf)
	recRates := httptest.NewRecorder()
	handler.ServeHTTP(recRates, reqRates)

	if recRates.Code != http.StatusOK {
		t.Fatalf("failed to update mapping rates: %d %s", recRates.Code, recRates.Body.String())
	}
}

func TestAdminLoginRateLimiting(t *testing.T) {
	_, _, _, handler := setupAdminTestEnv(t)

	badBody, _ := json.Marshal(map[string]string{"password": "WrongPassword"})

	// First 4 failed attempts should return 401
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
		req.RemoteAddr = "192.0.2.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i+1, rec.Code)
		}
	}

	// 5th failed attempt records threshold and returns 401
	req5 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
	req5.RemoteAddr = "192.0.2.1:1234"
	rec5 := httptest.NewRecorder()
	handler.ServeHTTP(rec5, req5)
	if rec5.Code != http.StatusUnauthorized {
		t.Fatalf("5th attempt: expected 401, got %d", rec5.Code)
	}

	// 6th attempt from same IP should be blocked with 429
	req6 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
	req6.RemoteAddr = "192.0.2.1:1234"
	rec6 := httptest.NewRecorder()
	handler.ServeHTTP(rec6, req6)
	if rec6.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: expected 429 Too Many Requests, got %d", rec6.Code)
	}

	// Attempt from another IP should still be allowed (returns 401)
	reqOther := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
	reqOther.RemoteAddr = "192.0.2.2:1234"
	recOther := httptest.NewRecorder()
	handler.ServeHTTP(recOther, reqOther)
	if recOther.Code != http.StatusUnauthorized {
		t.Fatalf("other IP attempt: expected 401, got %d", recOther.Code)
	}
}

func TestAdminLoginTrustedProxySpoofing(t *testing.T) {
	_, _, _, handler := setupAdminTestEnv(t)

	badBody, _ := json.Marshal(map[string]string{"password": "WrongPassword"})

	// Directly connecting untrusted IP trying to rotate spoofed headers
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
		req.RemoteAddr = "198.51.100.1:5555"
		req.Header.Set("X-Forwarded-For", "10.0.0."+string(rune('1'+i)))
		req.Header.Set("CF-Connecting-IP", "10.0.0."+string(rune('1'+i)))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("untrusted attempt %d: expected 401, got %d", i+1, rec.Code)
		}
	}

	// 6th attempt from the same untrusted RemoteAddr should be rate-limited despite rotated headers
	reqBlocked := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(badBody))
	reqBlocked.RemoteAddr = "198.51.100.1:5555"
	reqBlocked.Header.Set("X-Forwarded-For", "10.0.0.99")
	recBlocked := httptest.NewRecorder()
	handler.ServeHTTP(recBlocked, reqBlocked)
	if recBlocked.Code != http.StatusTooManyRequests {
		t.Fatalf("expected spoofed header attack to be blocked with 429, got %d", recBlocked.Code)
	}
}

func TestProviderKeyNotFoundAndValidation(t *testing.T) {
	_, _, adminPass, handler := setupAdminTestEnv(t)
	cookie, csrf := loginAdmin(t, handler, adminPass)

	// 1. Create key for non-existent provider should return 400
	badKeyBody, _ := json.Marshal(map[string]any{
		"provider_id":  "nonexistent-provider",
		"display_name": "Test Key",
		"secret":       "secret-123",
	})
	reqBadProv := httptest.NewRequest(http.MethodPost, "/api/providers/keys", bytes.NewReader(badKeyBody))
	reqBadProv.AddCookie(cookie)
	reqBadProv.Header.Set("X-CSRF-Token", csrf)
	recBadProv := httptest.NewRecorder()
	handler.ServeHTTP(recBadProv, reqBadProv)
	if recBadProv.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown provider, got %d: %s", recBadProv.Code, recBadProv.Body.String())
	}

	// 2. Update status of non-existent key should return 404
	statusBody, _ := json.Marshal(map[string]string{"status": "disabled"})
	reqStatus := httptest.NewRequest(http.MethodPost, "/api/providers/keys/pkey-unknown/status", bytes.NewReader(statusBody))
	reqStatus.AddCookie(cookie)
	reqStatus.Header.Set("X-CSRF-Token", csrf)
	recStatus := httptest.NewRecorder()
	handler.ServeHTTP(recStatus, reqStatus)
	if recStatus.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown provider key status update, got %d: %s", recStatus.Code, recStatus.Body.String())
	}

	// 3. Add balance adjustment for non-existent key should return 404
	adjBody, _ := json.Marshal(map[string]any{
		"amount_micro_usd": 1000000,
		"note":             "Credit top-up",
	})
	reqAdj := httptest.NewRequest(http.MethodPost, "/api/providers/keys/pkey-unknown/adjust", bytes.NewReader(adjBody))
	reqAdj.AddCookie(cookie)
	reqAdj.Header.Set("X-CSRF-Token", csrf)
	recAdj := httptest.NewRecorder()
	handler.ServeHTTP(recAdj, reqAdj)
	if recAdj.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown provider key adjustment, got %d: %s", recAdj.Code, recAdj.Body.String())
	}
}
