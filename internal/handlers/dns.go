package handlers

import (
	"log"
	"net/http"

	"construct/domains/internal/porkbun"
)

// ListDNSRecords returns all DNS records for a domain
func ListDNSRecords(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	if domain == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain is required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	resp, err := Porkbun.GetDNSRecords(domain)
	if err != nil {
		log.Printf("Porkbun dns retrieve error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to fetch DNS records"})
		return
	}

	if resp.Status != "SUCCESS" {
		WriteJSON(w, 502, map[string]any{"error": "registrar returned error", "status": resp.Status})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":  "success",
		"records": resp.Records,
	})
}

// CreateDNSRecord creates a new DNS record
func CreateDNSRecord(w http.ResponseWriter, r *http.Request) {
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

	recType := getString(body, "type")
	content := getString(body, "content")
	if recType == "" || content == "" {
		WriteJSON(w, 400, map[string]any{"error": "type and content are required"})
		return
	}

	rec := porkbun.DNSRecordInput{
		Name:    getString(body, "name"),
		Type:    recType,
		Content: content,
		TTL:     getString(body, "ttl"),
		Prio:    getString(body, "prio"),
	}

	resp, err := Porkbun.CreateDNSRecord(domain, rec)
	if err != nil {
		log.Printf("Porkbun dns create error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to create DNS record"})
		return
	}

	if resp.Status != "SUCCESS" {
		WriteJSON(w, 400, map[string]any{"error": "registrar rejected record", "status": resp.Status})
		return
	}

	WriteJSON(w, 201, map[string]any{
		"status": "success",
		"id":     resp.ID,
	})
}

// BulkCreateDNS creates multiple DNS records at once
func BulkCreateDNS(w http.ResponseWriter, r *http.Request) {
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

	recordsRaw, ok := body["records"]
	if !ok {
		WriteJSON(w, 400, map[string]any{"error": "records array is required"})
		return
	}

	recordsSlice, ok := recordsRaw.([]any)
	if !ok {
		WriteJSON(w, 400, map[string]any{"error": "records must be an array"})
		return
	}

	var results []map[string]any
	for _, raw := range recordsSlice {
		recMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		recType := ""
		if v, ok := recMap["type"].(string); ok {
			recType = v
		}
		name := ""
		if v, ok := recMap["name"].(string); ok {
			name = v
		}
		content := ""
		if v, ok := recMap["content"].(string); ok {
			content = v
		}

		if recType == "" || content == "" {
			results = append(results, map[string]any{
				"type": recType, "name": name, "content": content,
				"success": false, "error": "type and content are required",
			})
			continue
		}

		ttl := "600"
		if v, ok := recMap["ttl"].(string); ok && v != "" {
			ttl = v
		}
		prio := ""
		if v, ok := recMap["prio"].(string); ok {
			prio = v
		}

		resp, err := Porkbun.CreateDNSRecord(domain, porkbun.DNSRecordInput{
			Name:    name,
			Type:    recType,
			Content: content,
			TTL:     ttl,
			Prio:    prio,
		})

		if err != nil {
			log.Printf("Bulk DNS create error for %s %s: %v", recType, name, err)
			results = append(results, map[string]any{
				"type": recType, "name": name, "content": content,
				"success": false, "error": err.Error(),
			})
		} else if resp.Status != "SUCCESS" {
			results = append(results, map[string]any{
				"type": recType, "name": name, "content": content,
				"success": false, "error": "registrar rejected: " + resp.Status,
			})
		} else {
			results = append(results, map[string]any{
				"type": recType, "name": name, "content": content,
				"success": true, "id": resp.ID,
			})
		}
	}

	WriteJSON(w, 200, map[string]any{
		"status":  "success",
		"results": results,
	})
}

// EditDNSRecord updates an existing DNS record
func EditDNSRecord(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	recordID := r.PathValue("id")
	if domain == "" || recordID == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain and record id are required"})
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

	rec := porkbun.DNSRecordInput{
		Name:    getString(body, "name"),
		Type:    getString(body, "type"),
		Content: getString(body, "content"),
		TTL:     getString(body, "ttl"),
		Prio:    getString(body, "prio"),
	}

	resp, err := Porkbun.EditDNSRecord(domain, recordID, rec)
	if err != nil {
		log.Printf("Porkbun dns edit error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to edit DNS record"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":  resp.Status,
		"message": resp.Message,
	})
}

// DeleteDNSRecord deletes a DNS record
func DeleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	recordID := r.PathValue("id")
	if domain == "" || recordID == "" {
		WriteJSON(w, 400, map[string]any{"error": "domain and record id are required"})
		return
	}
	if _, ok := requireOwnedDomain(w, r, domain); !ok {
		return
	}

	resp, err := Porkbun.DeleteDNSRecord(domain, recordID)
	if err != nil {
		log.Printf("Porkbun dns delete error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to delete DNS record"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":  resp.Status,
		"message": resp.Message,
	})
}
