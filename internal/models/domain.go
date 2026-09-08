package models

import "time"

type Session struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Token     string    `gorm:"size:128;uniqueIndex;not null" json:"-"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	UserUUID  string    `gorm:"size:36" json:"user_uuid"`
	UserAgent *string   `gorm:"type:text" json:"user_agent"`
	IPAddress *string   `gorm:"size:255" json:"ip_address"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

type Domain struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       uint      `json:"user_id" gorm:"index"`
	UserUUID     string    `json:"user_uuid" gorm:"size:36;index"`
	Domain       string    `json:"domain" gorm:"uniqueIndex;size:255"`
	Status       string    `json:"status" gorm:"size:50;default:active"`
	AutoRenew    bool      `json:"auto_renew" gorm:"default:true"`
	ExpireDate   string    `json:"expire_date" gorm:"size:50"`
	CreateDate   string    `json:"create_date" gorm:"size:50"`
	SecurityLock bool      `json:"security_lock"`
	WhoisPrivacy bool      `json:"whois_privacy" gorm:"default:true"`
	Nameservers  string    `json:"nameservers" gorm:"size:1000"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DomainRedirect struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	UserID       uint      `json:"user_id" gorm:"index"`
	UserUUID     string    `json:"user_uuid" gorm:"size:36;index"`
	SourceDomain string    `json:"source_domain" gorm:"uniqueIndex;size:255"`
	TargetDomain string    `json:"target_domain" gorm:"size:255;not null"`
	RedirectType int       `json:"redirect_type" gorm:"default:301"`         // 301 permanent, 302 temporary
	IncludePath  bool      `json:"include_path" gorm:"default:true"`         // forward path from source to target
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DNSRecord struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	DomainID  uint      `json:"domain_id" gorm:"index"`
	Domain    string    `json:"domain" gorm:"index;size:255"`
	RecordID  string    `json:"record_id" gorm:"size:100"`
	Name      string    `json:"name" gorm:"size:255"`
	Type      string    `json:"type" gorm:"size:10"`
	Content   string    `json:"content" gorm:"size:1000"`
	TTL       string    `json:"ttl" gorm:"size:20"`
	Prio      string    `json:"prio" gorm:"size:10"`
	Notes     string    `json:"notes" gorm:"size:500"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
