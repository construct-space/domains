package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"construct/domains/internal/config"
)

func TestAuthTrustsGatewayIdentity(t *testing.T) {
	cfg := &config.Config{InternalSharedSecret: "shared"}
	called := false
	handler := Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := r.Header.Get("X-User-ID"); got != "42" {
			t.Fatalf("X-User-ID = %q, want 42", got)
		}
		if got := r.Header.Get("X-User-UUID"); got != "user-uuid" {
			t.Fatalf("X-User-UUID = %q, want user-uuid", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/domains", nil)
	req.Header.Set("X-Internal-Secret", "shared")
	req.Header.Set("X-Auth-User-Row-ID", "42")
	req.Header.Set("X-Auth-User-ID", "user-uuid")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("handler was not called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestAuthRejectsGatewayIdentityWithoutNumericRowID(t *testing.T) {
	cfg := &config.Config{InternalSharedSecret: "shared"}
	handler := Auth(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/domains", nil)
	req.Header.Set("X-Internal-Secret", "shared")
	req.Header.Set("X-Auth-User-Row-ID", "user-uuid")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}
