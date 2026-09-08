package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"construct/domains/internal/database"
	"construct/domains/internal/models"
)

// isTruthy handles Porkbun's inconsistent types (string "1" or number 1 or 0)
func isTruthy(v any) bool {
	switch val := v.(type) {
	case string:
		return val == "1"
	case float64:
		return val == 1
	case int:
		return val == 1
	case bool:
		return val
	default:
		return fmt.Sprintf("%v", v) == "1"
	}
}

// ListDomains returns only the authenticated user's domains, with registrar
// status refreshed for matching local rows.
func ListDomains(w http.ResponseWriter, r *http.Request) {
	userUUID := getUserUUID(r)
	if userUUID == "" {
		WriteJSON(w, 401, map[string]any{"error": "unauthorized"})
		return
	}
	// userID kept for legacy rows where user_uuid was never backfilled.
	// New writes always set user_uuid; the OR clause below is a soft
	// migration guard until we run a one-shot backfill.
	userID := getUserID(r)

	resp, err := Porkbun.ListDomains()
	if err != nil {
		log.Printf("Porkbun listAll error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to fetch domains from registrar"})
		return
	}

	if resp.Status != "SUCCESS" {
		WriteJSON(w, 502, map[string]any{"error": "registrar returned error", "status": resp.Status})
		return
	}

	owned := map[string]models.Domain{}
	var rows []models.Domain
	database.DB.Where("user_uuid = ? OR ((user_uuid = '' OR user_uuid IS NULL) AND user_id = ?)", userUUID, userID).Find(&rows)
	// Backfill user_uuid on legacy rows we just matched by user_id so
	// subsequent calls can drop the OR-clause entirely once everyone's rows are migrated.
	for i := range rows {
		if rows[i].UserUUID == "" && userUUID != "" {
			database.DB.Model(&rows[i]).Update("user_uuid", userUUID)
			rows[i].UserUUID = userUUID
		}
		owned[rows[i].Domain] = rows[i]
	}

	// Sync domains to local DB
	for _, d := range resp.Domains {
		if _, ok := owned[d.Domain]; !ok {
			continue
		}
		var domain models.Domain
		result := database.DB.Where("domain = ? AND user_uuid = ?", d.Domain, userUUID).First(&domain)
		if result.Error != nil {
			domain = models.Domain{
				UserID:       userID,
				UserUUID:     userUUID,
				Domain:       d.Domain,
				Status:       d.Status,
				AutoRenew:    isTruthy(d.AutoRenew),
				ExpireDate:   d.ExpireDate,
				CreateDate:   d.CreateDate,
				SecurityLock: isTruthy(d.SecurityLock),
				WhoisPrivacy: isTruthy(d.WhoisPrivacy),
			}
			database.DB.Create(&domain)
		} else {
			database.DB.Model(&domain).Updates(map[string]any{
				"status":        d.Status,
				"auto_renew":    isTruthy(d.AutoRenew),
				"expire_date":   d.ExpireDate,
				"security_lock": isTruthy(d.SecurityLock),
				"whois_privacy": isTruthy(d.WhoisPrivacy),
			})
		}
	}

	domains := make([]models.Domain, 0, len(rows))
	database.DB.Where("user_uuid = ?", userUUID).Order("domain ASC").Find(&domains)

	WriteJSON(w, 200, map[string]any{
		"status":  "success",
		"domains": domains,
	})
}

// GetDomain returns details for a specific domain. Falls back to the
// local row when Porkbun is unreachable or doesn't recognize the
// domain (e.g. transferred to another registrar but the local row
// hasn't been pruned). The detail page is "best effort" — losing the
// registrar enrichment shouldn't 502 the whole response.
func GetDomain(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	local, ok := requireOwnedDomain(w, r, domain)
	if !ok {
		return
	}

	resp, err := Porkbun.GetDomain(domain)
	if err != nil || resp == nil || resp.Status != "SUCCESS" {
		if err != nil {
			log.Printf("Porkbun getDomain error for %s: %v — serving local row", domain, err)
		} else {
			log.Printf("Porkbun getDomain non-success for %s: %s — serving local row", domain, resp.Status)
		}
		WriteJSON(w, 200, map[string]any{
			"status":                "success",
			"domain":                local.Domain,
			"create_date":           local.CreateDate,
			"expire_date":           local.ExpireDate,
			"security_lock":         local.SecurityLock,
			"whois_privacy":         local.WhoisPrivacy,
			"auto_renew":            local.AutoRenew,
			"registrar_unavailable": true,
		})
		return
	}

	// Get nameservers
	nsResp, _ := Porkbun.GetNameservers(domain)

	result := map[string]any{
		"status":        "success",
		"domain":        resp.Domain,
		"create_date":   resp.CreateDate,
		"expire_date":   resp.ExpireDate,
		"security_lock": resp.SecurityLock,
		"whois_privacy": resp.WhoisPrivacy,
		"auto_renew":    resp.AutoRenew,
	}

	if nsResp != nil && nsResp.Status == "SUCCESS" {
		result["nameservers"] = nsResp.NS
	}

	WriteJSON(w, 200, result)
}

// UpdateAutoRenew toggles auto-renew for a domain
func UpdateAutoRenew(w http.ResponseWriter, r *http.Request) {
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

	status := getString(body, "status")
	if status != "on" && status != "off" {
		WriteJSON(w, 400, map[string]any{"error": "status must be 'on' or 'off'"})
		return
	}

	resp, err := Porkbun.UpdateAutoRenew(domain, status)
	if err != nil {
		log.Printf("Porkbun updateAutoRenew error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to update auto-renew"})
		return
	}

	// Sync to local DB
	database.DB.Model(&models.Domain{}).Where("domain = ? AND user_uuid = ?", domain, getUserUUID(r)).Update("auto_renew", status == "on")

	WriteJSON(w, 200, map[string]any{
		"status":  resp.Status,
		"message": resp.Message,
	})
}

// UpdateNameservers sets nameservers for a domain
func UpdateNameservers(w http.ResponseWriter, r *http.Request) {
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

	nsRaw, ok := body["nameservers"]
	if !ok {
		WriteJSON(w, 400, map[string]any{"error": "nameservers array is required"})
		return
	}

	nsSlice, ok := nsRaw.([]any)
	if !ok {
		WriteJSON(w, 400, map[string]any{"error": "nameservers must be an array"})
		return
	}

	var ns []string
	for _, v := range nsSlice {
		if s, ok := v.(string); ok {
			ns = append(ns, s)
		}
	}

	resp, err := Porkbun.UpdateNameservers(domain, ns)
	if err != nil {
		log.Printf("Porkbun updateNs error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to update nameservers"})
		return
	}

	// Sync to local DB
	database.DB.Model(&models.Domain{}).Where("domain = ? AND user_uuid = ?", domain, getUserUUID(r)).Update("nameservers", strings.Join(ns, ","))

	WriteJSON(w, 200, map[string]any{
		"status":  resp.Status,
		"message": resp.Message,
	})
}

// GetURLForwarding returns URL forwarding rules
func GetURLForwarding(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	resp, err := Porkbun.GetURLForwarding(domain)
	if err != nil {
		log.Printf("Porkbun getUrlForwarding error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to fetch URL forwarding"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":   resp.Status,
		"forwards": resp.Forwards,
	})
}

// GetSSLBundle retrieves SSL certificate bundle for a domain
func GetSSLBundle(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	resp, err := Porkbun.GetSSLBundle(domain)
	if err != nil {
		log.Printf("Porkbun getSSL error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to fetch SSL bundle"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":            resp.Status,
		"intermediate_cert": resp.IntermediateCert,
		"certificate_chain": resp.CertificateChain,
		"private_key":       resp.PrivateKey,
		"public_key":        resp.PublicKey,
	})
}
