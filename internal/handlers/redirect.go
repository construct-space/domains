package handlers

import (
	"log"
	"net/http"
	"strings"

	"construct/domains/internal/database"
	"construct/domains/internal/models"
	"construct/domains/internal/porkbun"
)

// CreateRedirect sets up a domain redirect: source → https://target
// Also configures DNS (A records) for the source domain so it points to Construct server.
func CreateRedirect(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	body, err := parseBody(r)
	if err != nil {
		WriteJSON(w, 400, map[string]any{"error": "invalid request body"})
		return
	}

	target := strings.TrimSpace(getString(body, "target"))
	if target == "" {
		WriteJSON(w, 400, map[string]any{"error": "target domain is required"})
		return
	}

	// Strip protocol if provided, we always redirect to https
	target = strings.TrimPrefix(target, "https://")
	target = strings.TrimPrefix(target, "http://")
	target = strings.TrimRight(target, "/")

	if target == domain {
		WriteJSON(w, 400, map[string]any{"error": "source and target cannot be the same domain"})
		return
	}

	redirectType := 301
	if rt := getString(body, "redirect_type"); rt == "302" {
		redirectType = 302
	}

	includePath := true
	if ip, ok := body["include_path"]; ok {
		if v, ok := ip.(bool); ok {
			includePath = v
		}
	}

	userUUID := getUserUUID(r)
	userID := getUserID(r)

	// Check if redirect already exists for this source domain. Match
	// either UUID (canonical) or legacy user_id row for pre-migration data.
	var existing models.DomainRedirect
	if database.DB.Where(
		"source_domain = ? AND (user_uuid = ? OR ((user_uuid = '' OR user_uuid IS NULL) AND user_id = ?))",
		domain, userUUID, userID,
	).First(&existing).Error == nil {
		// Update existing — backfill user_uuid on legacy rows.
		updates := map[string]any{
			"target_domain": target,
			"redirect_type": redirectType,
			"include_path":  includePath,
		}
		if existing.UserUUID == "" && userUUID != "" {
			updates["user_uuid"] = userUUID
		}
		database.DB.Model(&existing).Updates(updates)
	} else {
		// Create new
		existing = models.DomainRedirect{
			UserID:       userID,
			UserUUID:     userUUID,
			SourceDomain: domain,
			TargetDomain: target,
			RedirectType: redirectType,
			IncludePath:  includePath,
		}
		if err := database.DB.Create(&existing).Error; err != nil {
			log.Printf("Failed to create redirect: %v", err)
			WriteJSON(w, 500, map[string]any{"error": "failed to create redirect"})
			return
		}
	}

	// Set up DNS: A records pointing to Construct server (root + wildcard)
	// The Construct server handles SSL and does the HTTP redirect.
	dnsResult := setupDNSForRedirect(domain)

	WriteJSON(w, 200, map[string]any{
		"status":      "success",
		"redirect":    existing,
		"target_url":  "https://" + target,
		"dns_deleted": dnsResult.deleted,
		"dns_created": dnsResult.created,
	})
}

// GetRedirect returns the redirect config for a domain
func GetRedirect(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	var redirect models.DomainRedirect
	if err := database.DB.Where(
		"source_domain = ? AND (user_uuid = ? OR ((user_uuid = '' OR user_uuid IS NULL) AND user_id = ?))",
		domain, getUserUUID(r), getUserID(r),
	).First(&redirect).Error; err != nil {
		WriteJSON(w, 404, map[string]any{"error": "no redirect configured for this domain"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":     "success",
		"redirect":   redirect,
		"target_url": "https://" + redirect.TargetDomain,
	})
}

// DeleteRedirect removes a redirect for a domain
func DeleteRedirect(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	var redirect models.DomainRedirect
	if err := database.DB.Where(
		"source_domain = ? AND (user_uuid = ? OR ((user_uuid = '' OR user_uuid IS NULL) AND user_id = ?))",
		domain, getUserUUID(r), getUserID(r),
	).First(&redirect).Error; err != nil {
		WriteJSON(w, 404, map[string]any{"error": "no redirect configured for this domain"})
		return
	}

	database.DB.Delete(&redirect)

	WriteJSON(w, 200, map[string]any{
		"status":  "success",
		"message": "redirect removed for " + domain,
	})
}

// LookupRedirect is a public endpoint for the Construct server to resolve redirects.
// Given a hostname (e.g. "www.old.com" or "old.com"), returns where to redirect.
func LookupRedirect(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		WriteJSON(w, 400, map[string]any{"error": "host query parameter is required"})
		return
	}

	// Strip port if present
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Try exact match first
	var redirect models.DomainRedirect
	if err := database.DB.Where("source_domain = ?", host).First(&redirect).Error; err == nil {
		writeRedirectLookup(w, redirect)
		return
	}

	// Try matching the root domain (handles *.domain.com → domain.com)
	parts := strings.SplitN(host, ".", 2)
	if len(parts) == 2 {
		rootDomain := parts[1]
		if err := database.DB.Where("source_domain = ?", rootDomain).First(&redirect).Error; err == nil {
			writeRedirectLookup(w, redirect)
			return
		}
	}

	WriteJSON(w, 404, map[string]any{"error": "no redirect found"})
}

func writeRedirectLookup(w http.ResponseWriter, redirect models.DomainRedirect) {
	WriteJSON(w, 200, map[string]any{
		"status":        "success",
		"target_url":    "https://" + redirect.TargetDomain,
		"redirect_type": redirect.RedirectType,
		"include_path":  redirect.IncludePath,
		"source_domain": redirect.SourceDomain,
	})
}

type dnsSetupResult struct {
	deleted []string
	created []string
}

// setupDNSForRedirect sets A records (root + wildcard) pointing to the Construct server.
// The Construct server handles SSL via CapRover/Let's Encrypt and performs the HTTP redirect.
// Reuses the same DNS cleanup + A record logic as SetupDomain.
func setupDNSForRedirect(domain string) dnsSetupResult {
	ip := Cfg.DefaultIP
	result := dnsSetupResult{}

	// Remove conflicting records on root and wildcard
	records, err := Porkbun.GetDNSRecords(domain)
	if err == nil && records.Status == "SUCCESS" {
		for _, rec := range records.Records {
			shouldDelete := false
			recName := strings.TrimSuffix(rec.Name, "."+domain)
			if recName == domain {
				recName = ""
			}

			if (rec.Type == "A" || rec.Type == "ALIAS" || rec.Type == "CNAME") && (recName == "" || recName == domain) {
				shouldDelete = true
			}
			if (rec.Type == "A" || rec.Type == "CNAME") && (recName == "*" || strings.HasPrefix(rec.Name, "*.")) {
				shouldDelete = true
			}

			if shouldDelete {
				_, delErr := Porkbun.DeleteDNSRecord(domain, rec.ID)
				if delErr != nil {
					log.Printf("Redirect DNS: failed to delete %s %s: %v", rec.Type, rec.Name, delErr)
				} else {
					result.deleted = append(result.deleted, rec.Type+" "+rec.Name+" ("+rec.Content+")")
				}
			}
		}
	}

	// Create root A record → Construct server
	resp, err := Porkbun.CreateDNSRecord(domain, porkbun.DNSRecordInput{
		Type:    "A",
		Name:    "",
		Content: ip,
		TTL:     "600",
	})
	if err == nil && resp.Status == "SUCCESS" {
		result.created = append(result.created, domain+" A → "+ip)
	} else if err != nil {
		log.Printf("Redirect A record error for %s: %v", domain, err)
	}

	// Create wildcard A record → Construct server (handles www and all subdomains)
	resp, err = Porkbun.CreateDNSRecord(domain, porkbun.DNSRecordInput{
		Type:    "A",
		Name:    "*",
		Content: ip,
		TTL:     "600",
	})
	if err == nil && resp.Status == "SUCCESS" {
		result.created = append(result.created, "*."+domain+" A → "+ip)
	} else if err != nil {
		log.Printf("Redirect wildcard A error for %s: %v", domain, err)
	}

	return result
}
