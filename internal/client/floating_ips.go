package client

import (
	"context"
	"fmt"
	"net/url"
)

// FloatingIPs provides methods for managing floating IPs
type FloatingIPs struct {
	client *Client
}

// NewFloatingIPs creates a new floating IPs service
func NewFloatingIPs(client *Client) *FloatingIPs {
	return &FloatingIPs{client: client}
}

// List retrieves all floating IPs for the organization
func (f *FloatingIPs) List(ctx context.Context) (*FloatingIPsResponse, error) {
	var result FloatingIPsResponse
	err := f.client.Get(ctx, "/floating_ips/organization", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list floating IPs: %w", err)
	}
	return &result, nil
}

// GetByAddress finds a floating IP by its address
func (f *FloatingIPs) GetByAddress(ctx context.Context, address string) (*FloatingIP, error) {
	resp, err := f.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, ip := range resp.SingleIPs {
		if ip.Address == address {
			return &ip, nil
		}
	}
	for _, subnet := range resp.Subnets {
		for _, ip := range subnet.IPAddresses {
			if ip.Address == address {
				return &ip, nil
			}
		}
	}

	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: fmt.Sprintf("floating IP %s not found", address)}
}

// acquireFloatingIPResponse represents the API response when acquiring a floating IP
type acquireFloatingIPResponse struct {
	Detail      string `json:"detail"`
	IPAddress   string `json:"ip_address"`
	FloatingIPID int   `json:"floating_ip_id"`
}

// Acquire acquires a new floating IP from the pool
func (f *FloatingIPs) Acquire(ctx context.Context, ipType, locationName string) (*FloatingIP, error) {
	path := fmt.Sprintf("/floating_ips/acquire?ip_type=%s&location_name=%s",
		url.QueryEscape(ipType), url.QueryEscape(locationName))
	var result acquireFloatingIPResponse
	err := f.client.Post(ctx, path, nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire floating IP: %w", err)
	}
	return &FloatingIP{
		ID:           result.FloatingIPID,
		Address:      result.IPAddress,
		Type:         ipType,
		Status:       "active",
		LocationName: locationName,
	}, nil
}

// Release releases a floating IP back to the pool
func (f *FloatingIPs) Release(ctx context.Context, address string) error {
	path := fmt.Sprintf("/floating_ips/release/%s", url.PathEscape(address))
	return f.client.Post(ctx, path, nil, nil)
}

// AssignToVPS assigns a floating IP to a VPS
func (f *FloatingIPs) AssignToVPS(ctx context.Context, vpsID int, address string) error {
	path := fmt.Sprintf("/floating_ips/assign/vps/%d?address=%s", vpsID, url.QueryEscape(address))
	return f.client.Post(ctx, path, nil, nil)
}

// AssignToBaremetal assigns a floating IP to a baremetal server
func (f *FloatingIPs) AssignToBaremetal(ctx context.Context, baremetalID int, address string) error {
	path := fmt.Sprintf("/floating_ips/assign/baremetal/%d?address=%s", baremetalID, url.QueryEscape(address))
	return f.client.Post(ctx, path, nil, nil)
}

// Unassign removes a floating IP assignment
func (f *FloatingIPs) Unassign(ctx context.Context, address string) error {
	path := fmt.Sprintf("/floating_ips/unassign/%s", url.PathEscape(address))
	return f.client.Post(ctx, path, nil, nil)
}

// ConfigureReverseDNS sets or removes reverse DNS for a floating IP
func (f *FloatingIPs) ConfigureReverseDNS(ctx context.Context, ip, reverseDNS string) error {
	path := fmt.Sprintf("/floating_ips/reverse_dns/configure?ip=%s&reverse_dns=%s",
		url.QueryEscape(ip), url.QueryEscape(reverseDNS))
	return f.client.Post(ctx, path, nil, nil)
}
