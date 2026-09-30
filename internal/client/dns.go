package client

import (
	"context"
	"fmt"
)

// DNSService provides methods for managing DNS zones and records
type DNSService struct {
	client *Client
}

// NewDNSService creates a new DNS service
func NewDNSService(client *Client) *DNSService {
	return &DNSService{client: client}
}

// ListZones retrieves all DNS zones
func (d *DNSService) ListZones(ctx context.Context) ([]DNSZone, error) {
	var result []DNSZone
	err := d.client.Get(ctx, "/dns/zones", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS zones: %w", err)
	}
	return result, nil
}

// GetZone retrieves a DNS zone by UUID
func (d *DNSService) GetZone(ctx context.Context, uuid string) (*DNSZone, error) {
	var result DNSZone
	err := d.client.Get(ctx, fmt.Sprintf("/dns/zones/%s", uuid), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get DNS zone: %w", err)
	}
	return &result, nil
}

// CreateZone creates a new DNS zone
func (d *DNSService) CreateZone(ctx context.Context, req *CreateDNSZoneRequest) (*DNSZone, error) {
	var result DNSZone
	err := d.client.Post(ctx, "/dns/zones", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS zone: %w", err)
	}
	return &result, nil
}

// DeleteZone deletes a DNS zone
func (d *DNSService) DeleteZone(ctx context.Context, uuid string) error {
	return d.client.Delete(ctx, fmt.Sprintf("/dns/zones/%s", uuid))
}

// ListRecords retrieves all records for a DNS zone
func (d *DNSService) ListRecords(ctx context.Context, zoneUUID string) ([]DNSRecord, error) {
	var result []DNSRecord
	err := d.client.Get(ctx, fmt.Sprintf("/dns/zones/%s/records", zoneUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS records: %w", err)
	}
	return result, nil
}

// CreateRecord creates a new DNS record
func (d *DNSService) CreateRecord(ctx context.Context, zoneUUID string, req *CreateDNSRecordRequest) (*DNSRecord, error) {
	var result DNSRecord
	err := d.client.Post(ctx, fmt.Sprintf("/dns/zones/%s/records", zoneUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create DNS record: %w", err)
	}
	return &result, nil
}

// UpdateRecord updates a DNS record
func (d *DNSService) UpdateRecord(ctx context.Context, zoneUUID, recordUUID string, req *UpdateDNSRecordRequest) (*DNSRecord, error) {
	var result DNSRecord
	err := d.client.Put(ctx, fmt.Sprintf("/dns/zones/%s/records/%s", zoneUUID, recordUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update DNS record: %w", err)
	}
	return &result, nil
}

// DeleteRecord deletes a DNS record
func (d *DNSService) DeleteRecord(ctx context.Context, zoneUUID, recordUUID string) error {
	return d.client.Delete(ctx, fmt.Sprintf("/dns/zones/%s/records/%s", zoneUUID, recordUUID))
}

// MoveZone moves a zone to another project of the organization
func (d *DNSService) MoveZone(ctx context.Context, zoneUUID string, projectID int) error {
	return d.client.Post(ctx, fmt.Sprintf("/dns/zones/%s/move-project", zoneUUID), map[string]int{"project_id": projectID}, nil)
}

// GetSOA retrieves the SOA configuration of a zone
func (d *DNSService) GetSOA(ctx context.Context, zoneUUID string) (*DNSZoneSOA, error) {
	var result DNSZoneSOA
	if err := d.client.Get(ctx, fmt.Sprintf("/dns/zones/%s/soa", zoneUUID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateSOA updates the SOA timers and hostmaster of a zone; the serial is incremented
func (d *DNSService) UpdateSOA(ctx context.Context, zoneUUID string, req *UpdateDNSZoneSOARequest) (*DNSZoneSOA, error) {
	var result DNSZoneSOA
	if err := d.client.Put(ctx, fmt.Sprintf("/dns/zones/%s/soa", zoneUUID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetHealthCheck retrieves the health check of a record
func (d *DNSService) GetHealthCheck(ctx context.Context, zoneUUID, recordUUID string) (*DNSHealthCheck, error) {
	var result DNSHealthCheck
	if err := d.client.Get(ctx, fmt.Sprintf("/dns/zones/%s/records/%s/health-check", zoneUUID, recordUUID), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PutHealthCheck creates or fully replaces the health check of a record
func (d *DNSService) PutHealthCheck(ctx context.Context, zoneUUID, recordUUID string, req *DNSHealthCheck) (*DNSHealthCheck, error) {
	var result DNSHealthCheck
	if err := d.client.Put(ctx, fmt.Sprintf("/dns/zones/%s/records/%s/health-check", zoneUUID, recordUUID), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteHealthCheck deletes the health check of a record
func (d *DNSService) DeleteHealthCheck(ctx context.Context, zoneUUID, recordUUID string) error {
	return d.client.Delete(ctx, fmt.Sprintf("/dns/zones/%s/records/%s/health-check", zoneUUID, recordUUID))
}
