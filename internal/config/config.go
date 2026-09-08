package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port              string
	AppURL            string
	AllowedOrigins    []string
	DBDriver          string
	DBHost            string
	DBPort            string
	DBName            string
	DBUser            string
	DBPass            string
	PorkbunAPIKey     string
	PorkbunSecret     string
	OAuthURL          string
	OAuthClientID     string
	OAuthClientSecret string
	OAuthRedirectURI  string
	PolarToken        string
	DefaultIP         string
	// Shared secret used by oracle (and anyone else on the internal mesh)
	// to authenticate server-to-server calls into /api/admin/*. Must match
	// the INTERNAL_SHARED_SECRET set on the caller's side.
	InternalSharedSecret string
}

func Load() *Config {
	loadEnvFile(".env")

	origins := strings.Split(env("ALLOWED_ORIGINS", "http://localhost:8001,http://localhost:3050,tauri://localhost"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return &Config{
		Port:              env("PORT", "8001"),
		AppURL:            env("APP_URL", "http://localhost:8001"),
		AllowedOrigins:    origins,
		DBDriver:          env("DB_DRIVER", "mysql"),
		DBHost:            env("DB_HOST", "localhost"),
		DBPort:            env("DB_PORT", "3306"),
		DBName:            env("DB_NAME", "construct_domains"),
		DBUser:            env("DB_USER", "root"),
		DBPass:            env("DB_PASS", ""),
		PorkbunAPIKey:     env("PORKBUN_API_KEY", ""),
		PorkbunSecret:     env("PORKBUN_SECRET_KEY", ""),
		OAuthURL:          env("OAUTH_URL", "https://accounts.lisaos.dev"),
		OAuthClientID:     env("OAUTH_CLIENT_ID", ""),
		OAuthClientSecret: env("OAUTH_CLIENT_SECRET", ""),
		OAuthRedirectURI:  env("OAUTH_REDIRECT_URI", "http://localhost:8001/api/auth/callback"),
		PolarToken:        env("POLAR_TOKEN", ""),
		DefaultIP:         env("DEFAULT_IP", "23.88.112.108"),
		InternalSharedSecret: env("INTERNAL_SHARED_SECRET", ""),
	}
}

func (c *Config) IsSecure() bool {
	return strings.HasPrefix(c.AppURL, "https")
}

func SessionCookie(cfg *Config, token string, clear bool) string {
	maxAge := 30 * 24 * 60 * 60
	value := token
	if clear {
		maxAge = 0
		value = ""
	}
	cookie := fmt.Sprintf("session=%s; Path=/; HttpOnly; SameSite=Lax; Max-Age=%d", value, maxAge)
	if cfg.IsSecure() {
		cookie += "; Secure"
	}
	return cookie
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
