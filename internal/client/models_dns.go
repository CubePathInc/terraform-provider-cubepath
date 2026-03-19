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
