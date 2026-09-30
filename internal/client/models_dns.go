package client

// DNSZone represents a DNS zone
type DNSZone struct {
	UUID         string   `json:"uuid"`
	Domain       string   `json:"domain"`
	Status       string   `json:"status"`
	RecordsCount int      `json:"records_count"`
	Nameservers  []string `json:"nameservers"`
	ProjectID    int      `json:"project_id"`
	CreatedAt    string   `json:"created_at"`
}

// DNSRecord represents a DNS record
type DNSRecord struct {
	UUID     string `json:"uuid"`
	ZoneUUID string `json:"zone_uuid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
}

// CreateDNSZoneRequest represents a request to create a DNS zone
type CreateDNSZoneRequest struct {
	Domain    string `json:"domain"`
	ProjectID *int   `json:"project_id,omitempty"`
}

// CreateDNSRecordRequest represents a request to create a DNS record
type CreateDNSRecordRequest struct {
	Name     string `json:"name"`
	Type     string `json:"record_type"`
	Content  string `json:"content"`
	TTL      int    `json:"ttl"`
	Priority *int   `json:"priority,omitempty"`
}

// UpdateDNSRecordRequest represents a request to update a DNS record
type UpdateDNSRecordRequest struct {
	Name     *string `json:"name,omitempty"`
	Content  *string `json:"content,omitempty"`
	TTL      *int    `json:"ttl,omitempty"`
	Priority *int    `json:"priority,omitempty"`
}

// DNSZoneSOA is the SOA configuration of a zone
type DNSZoneSOA struct {
	Serial     int64  `json:"serial"`
	Refresh    int    `json:"refresh"`
	Retry      int    `json:"retry"`
	Expire     int    `json:"expire"`
	Minimum    int    `json:"minimum"`
	PrimaryNS  string `json:"primary_ns"`
	Hostmaster string `json:"hostmaster"`
}

// UpdateDNSZoneSOARequest updates the SOA of a zone. Nil fields are left as they are.
type UpdateDNSZoneSOARequest struct {
	Refresh    *int    `json:"refresh,omitempty"`
	Retry      *int    `json:"retry,omitempty"`
	Expire     *int    `json:"expire,omitempty"`
	Minimum    *int    `json:"minimum,omitempty"`
	Hostmaster *string `json:"hostmaster,omitempty"`
}

// DNSHealthCheck is the health check of an A or AAAA record
type DNSHealthCheck struct {
	UUID               string  `json:"uuid,omitempty"`
	RecordUUID         string  `json:"record_uuid,omitempty"`
	Name               string  `json:"name"`
	CheckType          string  `json:"check_type"`
	Target             *string `json:"target"`
	Port               *int    `json:"port"`
	Path               *string `json:"path"`
	ExpectedStatus     *int    `json:"expected_status,omitempty"`
	IntervalSecs       int     `json:"interval_secs"`
	TimeoutSecs        int     `json:"timeout_secs"`
	HealthyThreshold   int     `json:"healthy_threshold"`
	UnhealthyThreshold int     `json:"unhealthy_threshold"`
	Enabled            bool    `json:"enabled"`
	LastStatus         string  `json:"last_status,omitempty"`
}
