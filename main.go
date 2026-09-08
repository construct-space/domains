package main

import (
	"log"
	"net/http"
	"time"

	"construct/domains/internal/config"
	"construct/domains/internal/database"
	"construct/domains/internal/handlers"
	"construct/domains/internal/middleware"
	"construct/domains/internal/polar"
	"construct/domains/internal/porkbun"
)

// API-only service as of 2026-04-22. The embedded Vue SPA (formerly served
// here from frontend/dist) moved to two separate repos: the public landing
// now lives in construct-space/domains-web (deployed to domains.lisaos.dev),
// and the authenticated tenant dashboard is a space in construct-space/my-web
// (my.lisaos.dev/domains). This binary only speaks JSON now.

func main() {
	cfg := config.Load()
	handlers.Cfg = cfg
	handlers.Porkbun = porkbun.New(cfg.PorkbunAPIKey, cfg.PorkbunSecret)
	handlers.Polar = polar.New(cfg.PolarToken)

	database.Init(cfg)

	mux := http.NewServeMux()

	auth := middleware.Auth(cfg)

	// Public — domain search & pricing
	mux.HandleFunc("POST /api/search", handlers.SearchDomain)
	mux.HandleFunc("GET /api/pricing", handlers.GetPricing)

	// OAuth flow
	mux.HandleFunc("GET /api/auth/login", handlers.LoginRedirect)
	mux.HandleFunc("GET /api/auth/register", handlers.RegisterRedirect)
	mux.HandleFunc("GET /api/auth/callback", handlers.AuthCallback)
	mux.HandleFunc("GET /api/auth/me", handlers.AuthMe)
	mux.HandleFunc("GET /api/auth/logout", handlers.Logout)

	// Checkout & payments (authenticated)
	mux.Handle("POST /api/checkout", auth(http.HandlerFunc(handlers.CreateCheckout)))
	mux.Handle("GET /api/checkout/{id}", auth(http.HandlerFunc(handlers.GetCheckoutStatus)))
	mux.Handle("GET /api/orders", auth(http.HandlerFunc(handlers.ListOrders)))
	mux.Handle("GET /api/orders/{id}", auth(http.HandlerFunc(handlers.GetOrder)))

	// Authenticated — domain management
	mux.Handle("GET /api/domains", auth(http.HandlerFunc(handlers.ListDomains)))
	mux.Handle("GET /api/domains/{domain}", auth(http.HandlerFunc(handlers.GetDomain)))
	mux.Handle("PUT /api/domains/{domain}/auto-renew", auth(http.HandlerFunc(handlers.UpdateAutoRenew)))
	mux.Handle("PUT /api/domains/{domain}/nameservers", auth(http.HandlerFunc(handlers.UpdateNameservers)))
	mux.Handle("GET /api/domains/{domain}/forwarding", auth(http.HandlerFunc(handlers.GetURLForwarding)))
	mux.Handle("GET /api/domains/{domain}/ssl", auth(http.HandlerFunc(handlers.GetSSLBundle)))

	// Domain setup (set default A records)
	mux.Handle("POST /api/domains/{domain}/setup", auth(http.HandlerFunc(handlers.SetupDomain)))

	// Domain redirect / alias
	mux.Handle("POST /api/domains/{domain}/redirect", auth(http.HandlerFunc(handlers.CreateRedirect)))
	mux.Handle("GET /api/domains/{domain}/redirect", auth(http.HandlerFunc(handlers.GetRedirect)))
	mux.Handle("DELETE /api/domains/{domain}/redirect", auth(http.HandlerFunc(handlers.DeleteRedirect)))

	// Public lookup for Construct server to resolve redirects
	mux.HandleFunc("GET /api/redirect/lookup", handlers.LookupRedirect)

	// Authenticated — DNS management
	mux.Handle("GET /api/domains/{domain}/dns", auth(http.HandlerFunc(handlers.ListDNSRecords)))
	mux.Handle("POST /api/domains/{domain}/dns", auth(http.HandlerFunc(handlers.CreateDNSRecord)))
	mux.Handle("POST /api/domains/{domain}/dns/bulk", auth(http.HandlerFunc(handlers.BulkCreateDNS)))
	mux.Handle("PUT /api/domains/{domain}/dns/{id}", auth(http.HandlerFunc(handlers.EditDNSRecord)))
	mux.Handle("DELETE /api/domains/{domain}/dns/{id}", auth(http.HandlerFunc(handlers.DeleteDNSRecord)))

	// Admin — server-to-server, X-Internal-Secret gated. Oracle proxies these.
	adminAuth := middleware.AdminAuth
	mux.Handle("GET /api/admin/stats", adminAuth(http.HandlerFunc(handlers.AdminStats)))
	mux.Handle("GET /api/admin/domains", adminAuth(http.HandlerFunc(handlers.AdminListDomains)))
	mux.Handle("GET /api/admin/domains/{domain}", adminAuth(http.HandlerFunc(handlers.AdminGetDomain)))
	mux.Handle("GET /api/admin/redirects", adminAuth(http.HandlerFunc(handlers.AdminListRedirects)))
	mux.Handle("GET /api/admin/tenants", adminAuth(http.HandlerFunc(handlers.AdminListTenants)))

	// Health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteJSON(w, 200, map[string]any{"status": "ok"})
	})
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		resp, err := handlers.Porkbun.Ping()
		if err != nil {
			handlers.WriteJSON(w, 502, map[string]any{"error": err.Error()})
			return
		}
		handlers.WriteJSON(w, 200, resp)
	})

	// Root — minimal JSON identity. Any non-/api path 404s; the landing
	// and tenant dashboard are served from separate deployments.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		handlers.WriteJSON(w, 200, map[string]any{"service": "domains-api", "status": "ok"})
	})

	// Middleware
	var handler http.Handler = mux
	handler = middleware.CORS(cfg)(handler)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.Logger(handler)

	log.Printf("[domains-api] listening on :%s", cfg.Port)
	log.Printf("[domains-api] cors allowed origins: %v", cfg.AllowedOrigins)
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
