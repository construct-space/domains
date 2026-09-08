package porkbun

type PingResponse struct {
	Status string `json:"status"`
	YourIP string `json:"yourIp"`
}

type StatusResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type DomainInfo struct {
	Domain       string `json:"domain"`
	Status       string `json:"status"`
	TLD          string `json:"tld"`
	CreateDate   string `json:"createDate"`
	ExpireDate   string `json:"expireDate"`
	SecurityLock any    `json:"securityLock"`
	WhoisPrivacy any    `json:"whoisPrivacy"`
	AutoRenew    any    `json:"autoRenew"`
	NotLocal     int    `json:"notLocal"`
}

type ListDomainsResponse struct {
	Status  string       `json:"status"`
	Domains []DomainInfo `json:"domains"`
}

type GetDomainResponse struct {
	Status       string   `json:"status"`
	Domain       string   `json:"domain"`
	CreateDate   string   `json:"createDate"`
	ExpireDate   string   `json:"expireDate"`
	SecurityLock any      `json:"securityLock"`
	WhoisPrivacy any      `json:"whoisPrivacy"`
	AutoRenew    any      `json:"autoRenew"`
	Nameservers  []string `json:"nameservers"`
}

type NameserversResponse struct {
	Status string   `json:"status"`
	NS     []string `json:"ns"`
}

type DNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     string `json:"ttl"`
	Prio    string `json:"prio"`
	Notes   string `json:"notes"`
}

type DNSRecordsResponse struct {
	Status  string      `json:"status"`
	Records []DNSRecord `json:"records"`
}

type DNSRecordInput struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     string `json:"ttl,omitempty"`
	Prio    string `json:"prio,omitempty"`
}

type CreateDNSResponse struct {
	Status string `json:"status"`
	ID     int    `json:"id"`
}

type URLForwardingRecord struct {
	ID          string `json:"id"`
	Subdomain   string `json:"subdomain"`
	Location    string `json:"location"`
	Type        string `json:"type"`
	IncludePath string `json:"includePath"`
	Wildcard    string `json:"wildcard"`
}

type URLForwardingResponse struct {
	Status   string                `json:"status"`
	Forwards []URLForwardingRecord `json:"forwards"`
}

type SSLBundleResponse struct {
	Status           string `json:"status"`
	IntermediateCert string `json:"intermediatecertificate"`
	CertificateChain string `json:"certificatechain"`
	PrivateKey       string `json:"privatekey"`
	PublicKey        string `json:"publickey"`
}

type CheckDomainResponse struct {
	Status   string             `json:"status"`
	Response *CheckDomainResult `json:"response,omitempty"`
}

type CheckDomainResult struct {
	Avail        string                        `json:"avail"`
	Type         string                        `json:"type"`
	Price        string                        `json:"price"`
	RegularPrice string                        `json:"regularPrice"`
	Premium      string                        `json:"premium"`
	Additional   map[string]*CheckDomainPrice  `json:"additional,omitempty"`
}

type CheckDomainPrice struct {
	Type         string `json:"type"`
	Price        string `json:"price"`
	RegularPrice string `json:"regularPrice"`
}

func (r *CheckDomainResponse) Available() bool {
	return r.Response != nil && r.Response.Avail == "yes"
}

func (r *CheckDomainResponse) RegistrationPrice() string {
	if r.Response != nil {
		return r.Response.Price
	}
	return ""
}

func (r *CheckDomainResponse) RenewalPrice() string {
	if r.Response != nil && r.Response.Additional != nil {
		if renewal, ok := r.Response.Additional["renewal"]; ok {
			return renewal.Price
		}
	}
	return ""
}

type TLDPricing struct {
	Registration string `json:"registration"`
	Renewal      string `json:"renewal"`
	Transfer     string `json:"transfer"`
}

type PricingResponse struct {
	Status  string                 `json:"status"`
	Pricing map[string]*TLDPricing `json:"pricing"`
}

type RegisterDomainResponse struct {
	Status  string `json:"status"`
	Domain  string `json:"domain"`
	OrderID int    `json:"orderId"`
	Message string `json:"message,omitempty"`
}
