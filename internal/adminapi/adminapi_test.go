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
