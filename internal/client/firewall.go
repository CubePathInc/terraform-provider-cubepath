package client

import (
	"context"
	"fmt"
)

// FirewallService provides methods for managing firewall groups
type FirewallService struct {
	client *Client
}

// NewFirewallService creates a new firewall service
func NewFirewallService(client *Client) *FirewallService {
	return &FirewallService{client: client}
}

// List retrieves all firewall groups for the user's organization
func (f *FirewallService) List(ctx context.Context) ([]FirewallGroup, error) {
	var groups []FirewallGroup
	err := f.client.Get(ctx, "/firewall/groups", &groups)
	if err != nil {
		return nil, fmt.Errorf("failed to list firewall groups: %w", err)
	}
	return groups, nil
}

// Get retrieves a specific firewall group by ID
func (f *FirewallService) Get(ctx context.Context, groupID int) (*FirewallGroup, error) {
	// List all groups and find the one with matching ID
	groups, err := f.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		if group.ID == groupID {
			return &group, nil
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("Firewall group with ID %d not found", groupID),
	}
}

// Create creates a new firewall group
func (f *FirewallService) Create(ctx context.Context, projectID int, req *CreateFirewallGroupRequest) (*FirewallGroup, error) {
	var group FirewallGroup
	path := fmt.Sprintf("/firewall/groups?project_id=%d", projectID)
	err := f.client.Post(ctx, path, req, &group)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall group: %w", err)
	}
	return &group, nil
}

// Update updates an existing firewall group
func (f *FirewallService) Update(ctx context.Context, groupID int, req *UpdateFirewallGroupRequest) (*FirewallGroup, error) {
	var group FirewallGroup
	path := fmt.Sprintf("/firewall/groups/%d", groupID)
	err := f.client.Put(ctx, path, req, &group)
	if err != nil {
		return nil, fmt.Errorf("failed to update firewall group: %w", err)
	}
	return &group, nil
}

// Delete deletes a firewall group
func (f *FirewallService) Delete(ctx context.Context, groupID int) error {
	path := fmt.Sprintf("/firewall/groups/%d", groupID)
	err := f.client.Delete(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to delete firewall group: %w", err)
	}
	return nil
}

// UpdateVPSGroups updates the firewall groups assigned to a VPS
func (f *FirewallService) UpdateVPSGroups(ctx context.Context, vpsID int, groupIDs []int) (*VPSFirewallGroupsResponse, error) {
	req := &VPSFirewallGroupsRequest{
		FirewallGroupIDs: groupIDs,
	}
	var response VPSFirewallGroupsResponse
	path := fmt.Sprintf("/firewall/vps/%d/groups", vpsID)
	err := f.client.Put(ctx, path, req, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to update VPS firewall groups: %w", err)
	}
	return &response, nil
}
