package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"construct/domains/internal/config"
	"construct/domains/internal/database"
	"construct/domains/internal/models"
	"construct/domains/internal/polar"
	"construct/domains/internal/porkbun"
)

var Cfg *config.Config
var Porkbun *porkbun.Client
var Polar *polar.Client

const maxRequestBodyBytes = 1 << 20

func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func parseBody(r *http.Request) (map[string]any, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if len(body) > maxRequestBodyBytes {
		return nil, fmt.Errorf("request body too large")
	}

	if len(body) == 0 {
		return make(map[string]any), nil
	}

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getUserID(r *http.Request) uint {
	idStr := r.Header.Get("X-User-ID")
	if idStr == "" {
		return 0
	}
	id, _ := strconv.ParseUint(idStr, 10, 64)
	return uint(id)
}

func getUserUUID(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-User-UUID"))
}

func requireOwnedDomain(w http.ResponseWriter, r *http.Request, domainName string) (*models.Domain, bool) {
	userUUID := getUserUUID(r)
	if userUUID == "" || domainName == "" {
		WriteJSON(w, http.StatusNotFound, map[string]any{"error": "domain not found"})
		return nil, false
	}

	// Legacy fallback: rows pre-UUID-migration may have user_uuid empty;
	// match by user_id in that case. Backfill on hit so the OR collapses
	// over time.
	userID := getUserID(r)
	var domain models.Domain
	if err := database.DB.Where(
		"domain = ? AND (user_uuid = ? OR ((user_uuid = '' OR user_uuid IS NULL) AND user_id = ?))",
		domainName, userUUID, userID,
	).First(&domain).Error; err != nil {
		WriteJSON(w, http.StatusNotFound, map[string]any{"error": "domain not found"})
		return nil, false
	}
	if domain.UserUUID == "" {
		database.DB.Model(&domain).Update("user_uuid", userUUID)
		domain.UserUUID = userUUID
	}
	return &domain, true
}

func getClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.Split(fwd, ",")[0]
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	return r.RemoteAddr
}
