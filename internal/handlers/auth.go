package handlers

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"construct/domains/internal/database"
	"construct/domains/internal/middleware"
	"construct/domains/internal/models"
)

var (
	pendingStatesMu sync.Mutex
	pendingStates   = make(map[string]time.Time)
)

func generateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSessionToken() string {
	b := make([]byte, 48)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// pendingRedirects stores the post-login redirect URL per state
var (
	pendingRedirectsMu sync.Mutex
	pendingRedirects   = make(map[string]string)
)

// LoginRedirect redirects to the OAuth provider's authorize endpoint
func LoginRedirect(w http.ResponseWriter, r *http.Request) {
	state := generateState()

	pendingStatesMu.Lock()
	pendingStates[state] = time.Now().Add(10 * time.Minute)
	now := time.Now()
	for k, exp := range pendingStates {
		if now.After(exp) {
			delete(pendingStates, k)
		}
	}
	pendingStatesMu.Unlock()

	// Store post-login redirect if provided
	if redirect := r.URL.Query().Get("redirect"); redirect != "" {
		pendingRedirectsMu.Lock()
		pendingRedirects[state] = redirect
		pendingRedirectsMu.Unlock()
	}

	params := url.Values{
		"client_id":     {Cfg.OAuthClientID},
		"redirect_uri":  {Cfg.OAuthRedirectURI},
		"response_type": {"code"},
		"scope":         {"profile email"},
		"state":         {state},
	}

	authorizeURL := Cfg.OAuthURL + "/oauth/authorize?" + params.Encode()
	w.Header().Set("Location", authorizeURL)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
}

// RegisterRedirect redirects to the OAuth provider's register endpoint
func RegisterRedirect(w http.ResponseWriter, r *http.Request) {
	state := generateState()

	pendingStatesMu.Lock()
	pendingStates[state] = time.Now().Add(10 * time.Minute)
	now := time.Now()
	for k, exp := range pendingStates {
		if now.After(exp) {
			delete(pendingStates, k)
		}
	}
	pendingStatesMu.Unlock()

	params := url.Values{
		"client_id":     {Cfg.OAuthClientID},
		"redirect_uri":  {Cfg.OAuthRedirectURI},
		"response_type": {"code"},
		"scope":         {"profile email"},
		"state":         {state},
	}

	registerURL := Cfg.OAuthURL + "/register?" + params.Encode()
	w.Header().Set("Location", registerURL)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
}

// AuthCallback handles the OAuth callback
func AuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errParam := r.URL.Query().Get("error")

	if errParam != "" {
		redirectWithError(w, "OAuth error: "+errParam)
		return
	}
	if code == "" {
		redirectWithError(w, "Missing authorization code")
		return
	}

	pendingStatesMu.Lock()
	expiry, exists := pendingStates[state]
	if exists {
		delete(pendingStates, state)
	}
	pendingStatesMu.Unlock()

	if !exists {
		redirectWithError(w, "Invalid state parameter")
		return
	}
	if time.Now().After(expiry) {
		redirectWithError(w, "State expired")
		return
	}

	// Exchange code for token
	log.Println("[auth] Exchanging code...")
	tokenData, err := exchangeCode(code)
	if err != nil {
		log.Printf("[auth] Token exchange error: %v", err)
		redirectWithError(w, "Failed to exchange authorization code")
		return
	}

	accessToken, ok := tokenData["access_token"].(string)
	if !ok || accessToken == "" {
		redirectWithError(w, "No access token received")
		return
	}

	// Fetch user info
	log.Println("[auth] Fetching user info...")
	userInfo, err := fetchUserInfo(accessToken)
	if err != nil {
		log.Printf("[auth] User info error: %v", err)
		redirectWithError(w, "Failed to fetch user profile")
		return
	}

	userIDRaw, ok := userInfo["id"]
	if !ok {
		redirectWithError(w, "No user ID in profile")
		return
	}

	var userID uint
	switch v := userIDRaw.(type) {
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	default:
		redirectWithError(w, "Invalid user ID format")
		return
	}

	userUUID, _ := userInfo["uuid"].(string)
	log.Printf("[auth] User: %v (id: %d, uuid: %s)", userInfo["email"], userID, userUUID)

	// Create session
	sessionToken := generateSessionToken()
	userAgent := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)

	session := models.Session{
		Token:     sessionToken,
		UserID:    userID,
		UserUUID:  userUUID,
		UserAgent: &userAgent,
		IPAddress: &ipAddress,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}
	database.DB.Create(&session)

	// Delete expired sessions
	database.DB.Where("expires_at < ?", time.Now()).Delete(&models.Session{})

	// Check for post-login redirect
	redirectTo := Cfg.AppURL + "/dashboard"
	pendingRedirectsMu.Lock()
	if r, ok := pendingRedirects[state]; ok {
		redirectTo = Cfg.AppURL + r
		delete(pendingRedirects, state)
	}
	pendingRedirectsMu.Unlock()

	// Set cookie and redirect
	w.Header().Set("Location", redirectTo)
	w.Header().Set("Set-Cookie", middleware.SessionCookie(Cfg, sessionToken, false))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
}

// AuthMe returns current authenticated user info
func AuthMe(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session == nil {
		WriteJSON(w, 200, map[string]any{"authenticated": false})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"authenticated": true,
		"user": map[string]any{
			"id": session.UserID,
		},
	})
}

// Logout clears the session and cookie
func Logout(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session != nil {
		database.DB.Delete(session)
	}

	w.Header().Set("Location", Cfg.AppURL)
	w.Header().Set("Set-Cookie", middleware.SessionCookie(Cfg, "", true))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
}

func redirectWithError(w http.ResponseWriter, message string) {
	encoded := url.QueryEscape(message)
	w.Header().Set("Location", "/?error="+encoded)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
}

func exchangeCode(code string) (map[string]any, error) {
	tokenURL := Cfg.OAuthURL + "/oauth/token"

	body := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     Cfg.OAuthClientID,
		"client_secret": Cfg.OAuthClientSecret,
		"redirect_uri":  Cfg.OAuthRedirectURI,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", tokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token exchange failed: %d %v", resp.StatusCode, result)
	}

	return result, nil
}

func fetchUserInfo(accessToken string) (map[string]any, error) {
	userInfoURL := Cfg.OAuthURL + "/api/me"

	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("request error: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("user info failed: %d %v", resp.StatusCode, result)
	}

	return result, nil
}
