package handlers

import (
	"log"
	"net/http"
	"strings"

	"construct/domains/internal/porkbun"
)

// SetupDomain sets default DNS records (A record to Construct server) for a new domain.
// First removes any conflicting ALIAS/CNAME records (e.g. Porkbun parking page),
// then creates root A and wildcard A records pointing to the Construct server IP.
func SetupDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	ip := Cfg.DefaultIP
	var created []string
	var deleted []string

	// Step 1: Remove conflicting records (ALIAS, CNAME on root and wildcard)
	records, err := Porkbun.GetDNSRecords(domain)
	if err == nil && records.Status == "SUCCESS" {
		for _, rec := range records.Records {
			shouldDelete := false
			recName := strings.TrimSuffix(rec.Name, "."+domain)
			if recName == domain {
				recName = "" // root record
			}

			// Delete ALIAS on root — conflicts with A record
			if rec.Type == "ALIAS" && (recName == "" || recName == domain) {
				shouldDelete = true
			}
			// Delete CNAME on root — conflicts with A record
			if rec.Type == "CNAME" && (recName == "" || recName == domain) {
				shouldDelete = true
			}
			// Delete CNAME on wildcard — we'll replace with A record
			if rec.Type == "CNAME" && (recName == "*" || strings.HasPrefix(rec.Name, "*.")) {
				shouldDelete = true
			}
			// Delete existing A records on root and wildcard (we'll recreate)
			if rec.Type == "A" && (recName == "" || recName == domain || recName == "*" || strings.HasPrefix(rec.Name, "*.")) {
				shouldDelete = true
			}

			if shouldDelete {
				_, delErr := Porkbun.DeleteDNSRecord(domain, rec.ID)
				if delErr != nil {
					log.Printf("Setup: failed to delete %s %s record %s: %v", rec.Type, rec.Name, rec.ID, delErr)
				} else {
					deleted = append(deleted, rec.Type+" "+rec.Name+" ("+rec.Content+")")
					log.Printf("Setup: deleted %s %s → %s (id: %s)", rec.Type, rec.Name, rec.Content, rec.ID)
				}
			}
		}
	}

	// Step 2: Create root A record
	resp, err := Porkbun.CreateDNSRecord(domain, porkbun.DNSRecordInput{
		Type:    "A",
		Name:    "",
		Content: ip,
		TTL:     "600",
	})
	if err != nil {
		log.Printf("Setup A record error for %s: %v", domain, err)
	} else if resp.Status == "SUCCESS" {
		created = append(created, domain+" A → "+ip)
	} else {
		log.Printf("Setup A record for %s: status=%s", domain, resp.Status)
	}

	// Step 3: Create wildcard A record
	resp, err = Porkbun.CreateDNSRecord(domain, porkbun.DNSRecordInput{
		Type:    "A",
		Name:    "*",
		Content: ip,
		TTL:     "600",
	})
	if err != nil {
		log.Printf("Setup wildcard A record error for %s: %v", domain, err)
	} else if resp.Status == "SUCCESS" {
		created = append(created, "*."+domain+" A → "+ip)
	} else {
		log.Printf("Setup wildcard A record for %s: status=%s", domain, resp.Status)
	}

	WriteJSON(w, 200, map[string]any{
		"status":  "success",
		"domain":  domain,
		"ip":      ip,
		"deleted": deleted,
		"created": created,
	})
}
