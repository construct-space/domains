package handlers

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"construct/domains/internal/database"
	"construct/domains/internal/models"
)

// admin.go — server-to-server endpoints consumed by oracle (and any other
// internal service) via X-Internal-Secret. None of these are reachable
// from browsers directly; oracle's UI proxies them.

// GET /api/admin/stats
// High-level portfolio counts + upcoming-renewal heatmap. Cheap aggregates
// only — nothing here scans per-record data.
func AdminStats(w http.ResponseWriter, r *http.Request) {
	var totalDomains, active, expired int64
	database.DB.Model(&models.Domain{}).Count(&totalDomains)
	database.DB.Model(&models.Domain{}).Where("status = ?", "active").Count(&active)
	database.DB.Model(&models.Domain{}).Where("status = ?", "expired").Count(&expired)

	// Expiring within 30 days. ExpireDate is a free-form string on the model
	// (whatever Porkbun returned), so compare in Go rather than SQL to keep
	// the query portable across date formats.
	var allDomains []models.Domain
	database.DB.Model(&models.Domain{}).Select("expire_date").Find(&allDomains)
	expiringSoon := 0
	cutoff := time.Now().AddDate(0, 0, 30)
	for _, d := range allDomains {
		t, ok := parseExpireDate(d.ExpireDate)
		if !ok {
			continue
		}
		if t.Before(cutoff) && t.After(time.Now()) {
			expiringSoon++
		}
	}

	var redirects int64
	database.DB.Model(&models.DomainRedirect{}).Count(&redirects)

	var tenants int64
	database.DB.Model(&models.Domain{}).Distinct("user_id").Count(&tenants)

	WriteJSON(w, 200, map[string]any{
		"domains": map[string]any{
			"total":         totalDomains,
			"active":        active,
			"expired":       expired,
			"expiring_soon": expiringSoon,
		},
		"redirects": redirects,
		"tenants":   tenants,
	})
}

// GET /api/admin/domains?page=N&limit=N&status=active&search=<substr>
// Paginated list across every tenant. Shape mirrors accounts' /api/admin/users
// ({data,total,page,limit}) so oracle-web's Pagination widget drops in clean.
func AdminListDomains(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	search := q.Get("search")

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	query := database.DB.Model(&models.Domain{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("domain LIKE ? OR user_uuid LIKE ?", like, like)
	}

	var total int64
	query.Count(&total)

	var rows []models.Domain
	query.Order("created_at desc").Offset((page - 1) * limit).Limit(limit).Find(&rows)

	WriteJSON(w, 200, map[string]any{
		"data":  rows,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GET /api/admin/domains/{domain}
// Single-domain detail with its DNS records (cached in our DB, not fetched
// live from Porkbun) and any redirect config.
func AdminGetDomain(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("domain")
	var d models.Domain
	if err := database.DB.Where("domain = ?", name).First(&d).Error; err != nil {
		WriteJSON(w, 404, map[string]string{"error": "domain not found"})
		return
	}

	var records []models.DNSRecord
	database.DB.Where("domain = ?", name).Order("type, name").Find(&records)

	var redirect *models.DomainRedirect
	var r0 models.DomainRedirect
	if err := database.DB.Where("source_domain = ?", name).First(&r0).Error; err == nil {
		redirect = &r0
	}

	WriteJSON(w, 200, map[string]any{
		"domain":    d,
		"dns":       records,
		"redirect":  redirect,
	})
}

// GET /api/admin/redirects?page=N&limit=N&search=<substr>
// Paginated list of every configured redirect — used by ops to spot broken
// loops (A → B, B → A) or stale targets.
func AdminListRedirects(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("search")

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit < 1 {
		limit = 25
	}
	if limit > 200 {
		limit = 200
	}

	query := database.DB.Model(&models.DomainRedirect{})
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("source_domain LIKE ? OR target_domain LIKE ?", like, like)
	}

	var total int64
	query.Count(&total)

	var rows []models.DomainRedirect
	query.Order("updated_at desc").Offset((page - 1) * limit).Limit(limit).Find(&rows)

	WriteJSON(w, 200, map[string]any{
		"data":  rows,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GET /api/admin/tenants
// Per-user footprint — answers "who's holding how many domains, using how
// many redirects, with what upcoming expiry pressure?"
func AdminListTenants(w http.ResponseWriter, r *http.Request) {
	type countRow struct {
		UserUUID string
		N        int64
	}
	var domainCounts []countRow
	database.DB.Raw(`SELECT user_uuid, COUNT(*) AS n FROM domains GROUP BY user_uuid`).Scan(&domainCounts)

	var redirectCounts []countRow
	database.DB.Raw(`
		SELECT d.user_uuid, COUNT(*) AS n
		FROM   domain_redirects r
		JOIN   domains d ON d.domain = r.source_domain
		GROUP  BY d.user_uuid
	`).Scan(&redirectCounts)

	// Expiry pressure needs the ExpireDate string parsed, so do it in Go.
	var all []models.Domain
	database.DB.Select("user_uuid, expire_date").Find(&all)
	soon := map[string]int64{}
	cutoff := time.Now().AddDate(0, 0, 30)
	for _, d := range all {
		t, ok := parseExpireDate(d.ExpireDate)
		if !ok {
			continue
		}
		if t.Before(cutoff) && t.After(time.Now()) {
			soon[d.UserUUID]++
		}
	}

	type tenant struct {
		UserUUID     string `json:"user_uuid"`
		Domains      int64  `json:"domains"`
		Redirects    int64  `json:"redirects"`
		ExpiringSoon int64  `json:"expiring_soon"`
	}

	by := map[string]*tenant{}
	get := func(id string) *tenant {
		if t, ok := by[id]; ok {
			return t
		}
		t := &tenant{UserUUID: id}
		by[id] = t
		return t
	}
	for _, c := range domainCounts {
		get(c.UserUUID).Domains = c.N
	}
	for _, c := range redirectCounts {
		get(c.UserUUID).Redirects = c.N
	}
	for uid, n := range soon {
		get(uid).ExpiringSoon = n
	}

	out := make([]*tenant, 0, len(by))
	for _, t := range by {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domains != out[j].Domains {
			return out[i].Domains > out[j].Domains
		}
		return out[i].UserUUID < out[j].UserUUID
	})

	WriteJSON(w, 200, map[string]any{"tenants": out})
}

// parseExpireDate tries a handful of common formats Porkbun returns. Returns
// ok=false for anything we can't parse — caller should skip silently.
func parseExpireDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
