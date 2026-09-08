package handlers

import (
	"log"
	"net/http"
	"strings"
)

// SearchDomain checks domain availability (public endpoint)
func SearchDomain(w http.ResponseWriter, r *http.Request) {
	body, err := parseBody(r)
	if err != nil {
		WriteJSON(w, 400, map[string]any{"error": "invalid request body"})
		return
	}

	query := getString(body, "query")
	if query == "" {
		WriteJSON(w, 400, map[string]any{"error": "query is required"})
		return
	}

	query = strings.TrimSpace(strings.ToLower(query))

	// If no TLD provided, check popular ones
	hasTLD := strings.Contains(query, ".")

	type result struct {
		Domain    string `json:"domain"`
		Available bool   `json:"available"`
		Price     string `json:"price,omitempty"`
		Renewal   string `json:"renewal,omitempty"`
	}

	var results []result

	if hasTLD {
		resp, err := Porkbun.CheckDomain(query)
		if err != nil {
			log.Printf("Porkbun check error for %s: %v", query, err)
			WriteJSON(w, 502, map[string]any{"error": "failed to check domain"})
			return
		}

		results = append(results, result{
			Domain:    query,
			Available: resp.Available(),
			Price:     resp.RegistrationPrice(),
			Renewal:   resp.RenewalPrice(),
		})
	} else {
		checkTLDs := []string{".com", ".net", ".io", ".dev", ".org", ".co", ".app", ".domains", ".space", ".sh"}
		type checkResult struct {
			res result
			err error
		}
		ch := make(chan checkResult, len(checkTLDs))

		for _, tld := range checkTLDs {
			go func(domain string) {
				resp, err := Porkbun.CheckDomain(domain)
				if err != nil {
					ch <- checkResult{err: err}
					return
				}
				ch <- checkResult{res: result{
					Domain:    domain,
					Available: resp.Available(),
					Price:     resp.RegistrationPrice(),
					Renewal:   resp.RenewalPrice(),
				}}
			}(query + tld)
		}

		for range checkTLDs {
			cr := <-ch
			if cr.err == nil {
				results = append(results, cr.res)
			}
		}
	}

	WriteJSON(w, 200, map[string]any{
		"status":  "success",
		"results": results,
	})
}

// GetPricing returns TLD pricing (public endpoint)
func GetPricing(w http.ResponseWriter, r *http.Request) {
	resp, err := Porkbun.GetPricing()
	if err != nil {
		log.Printf("Porkbun pricing error: %v", err)
		WriteJSON(w, 502, map[string]any{"error": "failed to fetch pricing"})
		return
	}

	WriteJSON(w, 200, map[string]any{
		"status":  resp.Status,
		"pricing": resp.Pricing,
	})
}
