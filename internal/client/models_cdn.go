package client

import "encoding/json"

// CDNPlan represents a CDN plan
type CDNPlan struct {
	UUID              string          `json:"uuid"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	PricePerGB        json.RawMessage `json:"price_per_gb"`
	BasePricePerHour  float64         `json:"base_price_per_hour"`
	MaxZones          int             `json:"max_zones"`
	MaxOriginsPerZone int             `json:"max_origins_per_zone"`
	MaxRulesPerZone   int             `json:"max_rules_per_zone"`
	CustomSSLAllowed  bool            `json:"custom_ssl_allowed"`
}

// CDNZone represents a CDN zone
type CDNZone struct {
	UUID         string      `json:"uuid"`
	Name         string      `json:"name"`
	Domain       string      `json:"domain"`
	CustomDomain string      `json:"custom_domain"`
	Status       string      `json:"status"`
	PlanName     string      `json:"plan_name"`
	SSLType      string      `json:"ssl_type"`
	ProjectID    int         `json:"project_id"`
	Origins      []CDNOrigin `json:"origins"`
	Rules        []CDNRule   `json:"rules"`
	CreatedAt    string      `json:"created_at"`
	UpdatedAt    string      `json:"updated_at"`
}

// CDNOrigin represents a CDN origin server
type CDNOrigin struct {
	UUID               string `json:"uuid"`
	Name               string `json:"name"`
	Address            string `json:"address"`
	Port               int    `json:"port"`
	Protocol           string `json:"protocol"`
	Weight             int    `json:"weight"`
	Priority           int    `json:"priority"`
	IsBackup           bool   `json:"is_backup"`
	HealthCheckEnabled bool   `json:"health_check_enabled"`
	HealthCheckPath    string `json:"health_check_path"`
	HealthStatus       string `json:"health_status"`
	VerifySSL          bool   `json:"verify_ssl"`
	HostHeader         string `json:"host_header"`
	BasePath           string `json:"base_path"`
	Enabled            bool   `json:"enabled"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// CDNRule represents a CDN edge rule or WAF rule
type CDNRule struct {
	UUID            string          `json:"uuid"`
	Name            string          `json:"name"`
	RuleType        string          `json:"rule_type"`
	Priority        int             `json:"priority"`
	MatchConditions json.RawMessage `json:"match_conditions"`
	ActionConfig    json.RawMessage `json:"action_config"`
	Enabled         bool            `json:"enabled"`
	ExpiresAt       string          `json:"expires_at"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
}

// CreateCDNZoneRequest represents a request to create a CDN zone
type CreateCDNZoneRequest struct {
	Name         string `json:"name"`
	PlanName     string `json:"plan_name"`
	CustomDomain string `json:"custom_domain,omitempty"`
	ProjectID    *int   `json:"project_id,omitempty"`
}

// UpdateCDNZoneRequest represents a request to update a CDN zone
type UpdateCDNZoneRequest struct {
	Name            *string `json:"name,omitempty"`
	CustomDomain    *string `json:"custom_domain,omitempty"`
	SSLType         *string `json:"ssl_type,omitempty"`
	CertificateUUID *string `json:"certificate_uuid,omitempty"`
}

// CreateCDNOriginRequest represents a request to create a CDN origin
type CreateCDNOriginRequest struct {
	Name               string  `json:"name"`
	OriginURL          string  `json:"origin_url,omitempty"`
	Address            string  `json:"address,omitempty"`
	Port               *int    `json:"port,omitempty"`
	Protocol           string  `json:"protocol,omitempty"`
	Weight             int     `json:"weight"`
	Priority           int     `json:"priority"`
	IsBackup           bool    `json:"is_backup"`
	HealthCheckEnabled bool    `json:"health_check_enabled"`
	HealthCheckPath    string  `json:"health_check_path"`
	VerifySSL          bool    `json:"verify_ssl"`
	HostHeader         string  `json:"host_header,omitempty"`
	BasePath           string  `json:"base_path,omitempty"`
	Enabled            bool    `json:"enabled"`
}

// UpdateCDNOriginRequest represents a request to update a CDN origin
type UpdateCDNOriginRequest struct {
	Name               *string `json:"name,omitempty"`
	Address            *string `json:"address,omitempty"`
	Port               *int    `json:"port,omitempty"`
	Protocol           *string `json:"protocol,omitempty"`
	Weight             *int    `json:"weight,omitempty"`
	Priority           *int    `json:"priority,omitempty"`
	HostHeader         *string `json:"host_header,omitempty"`
	BasePath           *string `json:"base_path,omitempty"`
	HealthCheckEnabled *bool   `json:"health_check_enabled,omitempty"`
	HealthCheckPath    *string `json:"health_check_path,omitempty"`
	VerifySSL          *bool   `json:"verify_ssl,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`
}

// CreateCDNRuleRequest represents a request to create a CDN rule
type CreateCDNRuleRequest struct {
	Name            string          `json:"name"`
	RuleType        string          `json:"rule_type"`
	Priority        int             `json:"priority"`
	MatchConditions json.RawMessage `json:"match_conditions,omitempty"`
	ActionConfig    json.RawMessage `json:"action_config"`
	Enabled         bool            `json:"enabled"`
}

// UpdateCDNRuleRequest represents a request to update a CDN rule
type UpdateCDNRuleRequest struct {
	Name            *string          `json:"name,omitempty"`
	Priority        *int             `json:"priority,omitempty"`
	MatchConditions *json.RawMessage `json:"match_conditions,omitempty"`
	ActionConfig    *json.RawMessage `json:"action_config,omitempty"`
	Enabled         *bool            `json:"enabled,omitempty"`
}
