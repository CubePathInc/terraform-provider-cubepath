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

	// Token Auth and CORS. The secret is only returned to callers with cdn:write.
	TokenAuthEnabled   bool    `json:"token_auth_enabled"`
	TokenAuthIPBinding bool    `json:"token_auth_ip_binding"`
	TokenAuthSecret    *string `json:"token_auth_secret"`
	CORSEnabled        bool    `json:"cors_enabled"`
	CORSAllowOrigins   *string `json:"cors_allow_origins"`

	// Name of the origin response header that carries the cache tags (null: tag purges are off)
	// and compression of text responses at the edge.
	CacheTagHeader     *string `json:"cache_tag_header"`
	CompressionEnabled bool    `json:"compression_enabled"`
	// Absent from an API that has no managed WAF.
	ManagedWAF *CDNManagedWAF `json:"managed_waf"`
}

// CDNManagedWAF is the managed WAF in detection mode: it logs what its rule set finds and
// blocks nothing.
type CDNManagedWAF struct {
	Mode              string   `json:"mode"`
	Level             int      `json:"level"`
	ExcludeRuleIDs    []int64  `json:"exclude_rule_ids"`
	ExcludeCategories []string `json:"exclude_categories"`
}

// UpdateCDNManagedWAF is the managed_waf part of an update: lists are sent whole.
type UpdateCDNManagedWAF struct {
	Mode              string   `json:"mode"`
	Level             int      `json:"level"`
	ExcludeRuleIDs    []int64  `json:"exclude_rule_ids"`
	ExcludeCategories []string `json:"exclude_categories"`
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
	// Set when the origin serves a CubePath Object Storage bucket (read only connection fields)
	ObjectStorageBucketUUID *string `json:"object_storage_bucket_uuid"`
	// Read only: why the origin is down and when the status last changed (null until known)
	HealthDetail    *string `json:"health_detail"`
	HealthChangedAt *string `json:"health_changed_at"`
	// The check of the zone: tcp or http, and the settings an http check uses
	HealthCheckType           string `json:"health_check_type"`
	HealthCheckMethod         string `json:"health_check_method"`
	HealthCheckExpectedStatus string `json:"health_check_expected_status"`
	HealthCheckInterval       int    `json:"health_check_interval"`
	HealthCheckThreshold      int    `json:"health_check_threshold"`
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
	// Nil booleans are left as they are
	TokenAuthEnabled   *bool                `json:"token_auth_enabled,omitempty"`
	TokenAuthIPBinding *bool                `json:"token_auth_ip_binding,omitempty"`
	CORSEnabled        *bool                `json:"cors_enabled,omitempty"`
	CORSAllowOrigins   *string              `json:"cors_allow_origins,omitempty"`
	CompressionEnabled *bool                `json:"compression_enabled,omitempty"`
	ManagedWAF         *UpdateCDNManagedWAF `json:"managed_waf,omitempty"`
	// CacheTagHeader sets the cache tag header; nil leaves it as it is. An empty string turns
	// tag purges off: it is sent as JSON null, which is how the API clears it.
	CacheTagHeader *string `json:"-"`
}

// MarshalJSON adds cache_tag_header: the name, or null when it is cleared with an empty string.
func (r UpdateCDNZoneRequest) MarshalJSON() ([]byte, error) {
	type plain UpdateCDNZoneRequest
	body, err := json.Marshal(plain(r))
	if err != nil || r.CacheTagHeader == nil {
		return body, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	if *r.CacheTagHeader == "" {
		fields["cache_tag_header"] = json.RawMessage("null")
	} else {
		name, err := json.Marshal(*r.CacheTagHeader)
		if err != nil {
			return nil, err
		}
		fields["cache_tag_header"] = name
	}
	return json.Marshal(fields)
}

// CreateCDNOriginRequest represents a request to create a CDN origin
type CreateCDNOriginRequest struct {
	Name               string `json:"name"`
	OriginURL          string `json:"origin_url,omitempty"`
	Address            string `json:"address,omitempty"`
	Port               *int   `json:"port,omitempty"`
	Protocol           string `json:"protocol,omitempty"`
	Weight             int    `json:"weight"`
	Priority           int    `json:"priority"`
	IsBackup           bool   `json:"is_backup"`
	HealthCheckEnabled bool   `json:"health_check_enabled"`
	HealthCheckPath    string `json:"health_check_path,omitempty"`
	VerifySSL          bool   `json:"verify_ssl"`
	HostHeader         string `json:"host_header,omitempty"`
	BasePath           string `json:"base_path,omitempty"`
	Enabled            bool   `json:"enabled"`
	// Sent only when set, so the API applies its defaults otherwise
	HealthCheckType           string `json:"health_check_type,omitempty"`
	HealthCheckMethod         string `json:"health_check_method,omitempty"`
	HealthCheckExpectedStatus string `json:"health_check_expected_status,omitempty"`
	HealthCheckInterval       *int   `json:"health_check_interval,omitempty"`
	HealthCheckThreshold      *int   `json:"health_check_threshold,omitempty"`
}

// CreateCDNBucketOriginRequest creates an origin that serves a CubePath Object Storage bucket.
// The API fills every connection field from the bucket and refuses them next to the bucket UUID,
// so only these fields are sent.
type CreateCDNBucketOriginRequest struct {
	Name                    string `json:"name"`
	ObjectStorageBucketUUID string `json:"object_storage_bucket_uuid"`
	Weight                  int    `json:"weight"`
	Priority                int    `json:"priority"`
	IsBackup                bool   `json:"is_backup"`
}

// UpdateCDNOriginRequest represents a request to update a CDN origin
type UpdateCDNOriginRequest struct {
	Name               *string `json:"name,omitempty"`
	Address            *string `json:"address,omitempty"`
	Port               *int    `json:"port,omitempty"`
	Protocol           *string `json:"protocol,omitempty"`
	Weight             *int    `json:"weight,omitempty"`
	Priority           *int    `json:"priority,omitempty"`
	IsBackup           *bool   `json:"is_backup,omitempty"`
	HostHeader         *string `json:"host_header,omitempty"`
	BasePath           *string `json:"base_path,omitempty"`
	HealthCheckEnabled *bool   `json:"health_check_enabled,omitempty"`
	HealthCheckPath    *string `json:"health_check_path,omitempty"`
	VerifySSL          *bool   `json:"verify_ssl,omitempty"`
	Enabled            *bool   `json:"enabled,omitempty"`

	HealthCheckType           *string `json:"health_check_type,omitempty"`
	HealthCheckMethod         *string `json:"health_check_method,omitempty"`
	HealthCheckExpectedStatus *string `json:"health_check_expected_status,omitempty"`
	HealthCheckInterval       *int    `json:"health_check_interval,omitempty"`
	HealthCheckThreshold      *int    `json:"health_check_threshold,omitempty"`
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
