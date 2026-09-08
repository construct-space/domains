package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
)

// AdminAuth accepts requests carrying a matching X-Internal-Secret header.
// The secret is read from INTERNAL_SHARED_SECRET at call time (after config.Load
// has already pushed values from .env into the process env). Used to gate the
// /api/admin/* routes consumed by oracle + other internal services.
func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := os.Getenv("INTERNAL_SHARED_SECRET")
		if secret == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Secret")), []byte(secret)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
