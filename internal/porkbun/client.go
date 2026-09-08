package porkbun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://api.porkbun.com/api/json/v3"

type Client struct {
	APIKey    string
	SecretKey string
	HTTP      *http.Client
}

func New(apiKey, secretKey string) *Client {
	return &Client{
		APIKey:    apiKey,
		SecretKey: secretKey,
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}
}

type authBody struct {
	SecretAPIKey string `json:"secretapikey"`
	APIKey       string `json:"apikey"`
}

func (c *Client) auth() authBody {
	return authBody{
		SecretAPIKey: c.SecretKey,
		APIKey:       c.APIKey,
	}
}

func (c *Client) do(method, path string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest(method, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	return data, nil
}

// Ping checks API authentication
func (c *Client) Ping() (*PingResponse, error) {
	data, err := c.do("POST", "/ping", c.auth())
	if err != nil {
		return nil, err
	}
	var resp PingResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListDomains returns all domains in the account
func (c *Client) ListDomains() (*ListDomainsResponse, error) {
	data, err := c.do("POST", "/domain/listAll", c.auth())
	if err != nil {
		return nil, err
	}
	var resp ListDomainsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetDomain returns details for a specific domain
func (c *Client) GetDomain(domain string) (*GetDomainResponse, error) {
	data, err := c.do("POST", "/domain/getDomain/"+domain, c.auth())
	if err != nil {
		return nil, err
	}
	var resp GetDomainResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNameservers returns the nameservers for a domain
func (c *Client) GetNameservers(domain string) (*NameserversResponse, error) {
	data, err := c.do("POST", "/domain/getNs/"+domain, c.auth())
	if err != nil {
		return nil, err
	}
	var resp NameserversResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateNameservers sets nameservers for a domain
func (c *Client) UpdateNameservers(domain string, ns []string) (*StatusResponse, error) {
	payload := struct {
		authBody
		NS []string `json:"ns"`
	}{c.auth(), ns}

	data, err := c.do("POST", "/domain/updateNs/"+domain, payload)
	if err != nil {
		return nil, err
	}
	var resp StatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// UpdateAutoRenew toggles auto-renew for a domain
func (c *Client) UpdateAutoRenew(domain string, status string) (*StatusResponse, error) {
	payload := struct {
		authBody
		Status string `json:"status"`
	}{c.auth(), status}

	data, err := c.do("POST", "/domain/updateAutoRenew/"+domain, payload)
	if err != nil {
		return nil, err
	}
	var resp StatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetDNSRecords returns all DNS records for a domain
func (c *Client) GetDNSRecords(domain string) (*DNSRecordsResponse, error) {
	data, err := c.do("POST", "/dns/retrieve/"+domain, c.auth())
	if err != nil {
		return nil, err
	}
	var resp DNSRecordsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateDNSRecord creates a new DNS record
func (c *Client) CreateDNSRecord(domain string, rec DNSRecordInput) (*CreateDNSResponse, error) {
	payload := struct {
		authBody
		DNSRecordInput
	}{c.auth(), rec}

	data, err := c.do("POST", "/dns/create/"+domain, payload)
	if err != nil {
		return nil, err
	}
	var resp CreateDNSResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EditDNSRecord edits an existing DNS record
func (c *Client) EditDNSRecord(domain, recordID string, rec DNSRecordInput) (*StatusResponse, error) {
	payload := struct {
		authBody
		DNSRecordInput
	}{c.auth(), rec}

	data, err := c.do("POST", "/dns/edit/"+domain+"/"+recordID, payload)
	if err != nil {
		return nil, err
	}
	var resp StatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteDNSRecord deletes a DNS record
func (c *Client) DeleteDNSRecord(domain, recordID string) (*StatusResponse, error) {
	data, err := c.do("POST", "/dns/delete/"+domain+"/"+recordID, c.auth())
	if err != nil {
		return nil, err
	}
	var resp StatusResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetURLForwarding returns URL forwarding rules for a domain
func (c *Client) GetURLForwarding(domain string) (*URLForwardingResponse, error) {
	data, err := c.do("POST", "/domain/getUrlForwarding/"+domain, c.auth())
	if err != nil {
		return nil, err
	}
	var resp URLForwardingResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CheckDomain checks if a domain is available for registration
func (c *Client) CheckDomain(domain string) (*CheckDomainResponse, error) {
	data, err := c.do("POST", "/domain/checkDomain/"+domain, c.auth())
	if err != nil {
		return nil, err
	}
	var resp CheckDomainResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetPricing returns pricing for all TLDs
func (c *Client) GetPricing() (*PricingResponse, error) {
	data, err := c.do("POST", "/pricing/get", c.auth())
	if err != nil {
		return nil, err
	}
	var resp PricingResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetSSLBundle retrieves the SSL certificate bundle for a domain
func (c *Client) GetSSLBundle(domain string) (*SSLBundleResponse, error) {
	data, err := c.do("POST", "/ssl/retrieve/"+domain, c.auth())
	if err != nil {
		return nil, err
	}
	var resp SSLBundleResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
