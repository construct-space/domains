package middleware

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"construct/domains/internal/config"
	"construct/domains/internal/database"
	"construct/domains/internal/models"
)

// Auth accepts the unified my.lisaos.dev gateway identity first, then
// falls back to the legacy domains session cookie for direct/old flows.
// Sets X-User-ID and X-User-UUID headers for downstream handlers.
func Auth(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if trustGatewayIdentity(cfg, r) {
				next.ServeHTTP(w, r)
				return
			}

			session := GetSession(r)
			if session == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}

			r.Header.Set("X-User-ID", strconv.FormatUint(uint64(session.UserID), 10))
			if session.UserUUID != "" {
				r.Header.Set("X-User-UUID", session.UserUUID)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func trustGatewayIdentity(cfg *config.Config, r *http.Request) bool {
	if cfg == nil || cfg.InternalSharedSecret == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(cfg.InternalSharedSecret)) != 1 {
		return false
	}

	rowID := strings.TrimSpace(r.Header.Get("X-Auth-User-Row-ID"))
	if rowID == "" {
		return false
	}
	if _, err := strconv.ParseUint(rowID, 10, 64); err != nil {
		return false
	}

	r.Header.Set("X-User-ID", rowID)
	if userUUID := strings.TrimSpace(r.Header.Get("X-Auth-User-ID")); userUUID != "" {
		r.Header.Set("X-User-UUID", userUUID)
	}
	return true
}

// GetSession returns the session from the cookie, or nil if invalid/expired.
func GetSession(r *http.Request) *models.Session {
	token := ParseCookie(r.Header.Get("Cookie"), "session")
	if token == "" {
		return nil
	}

	var session models.Session
	if err := database.DB.Where("token = ?", token).First(&session).Error; err != nil {
		return nil
	}
	if session.IsExpired() {
		return nil
	}
	return &session
}
