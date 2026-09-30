package client

import (
	"context"
	"fmt"
)

// AvailabilityGroup represents an availability group
type AvailabilityGroup struct {
	UUID         string                 `json:"uuid"`
	ProjectID    int                    `json:"project_id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Strategy     string                 `json:"strategy"`
	LocationName string                 `json:"location_name"`
	MaxServers   int                    `json:"max_servers"`
	VPSCount     int                    `json:"vps_count"`
	VPSList      []AvailabilityGroupVPS `json:"vps_list"`
	CreatedAt    string                 `json:"created_at"`
}

// AvailabilityGroupVPS represents a VPS within an availability group
type AvailabilityGroupVPS struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Label  string `json:"label"`
	Status string `json:"status"`
}

// AvailabilityGroupListResponse represents the response from listing availability groups
type AvailabilityGroupListResponse struct {
	Groups []AvailabilityGroup `json:"groups"`
}

// CreateAvailabilityGroupRequest represents a request to create an availability group
type CreateAvailabilityGroupRequest struct {
	ProjectID    int    `json:"project_id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	LocationName string `json:"location_name"`
}

// AvailabilityGroups provides methods for managing availability groups
type AvailabilityGroups struct {
	client *Client
}

// NewAvailabilityGroups creates a new availability groups service
func NewAvailabilityGroups(client *Client) *AvailabilityGroups {
	return &AvailabilityGroups{client: client}
}

// Create creates a new availability group
func (s *AvailabilityGroups) Create(ctx context.Context, req *CreateAvailabilityGroupRequest) (*AvailabilityGroup, error) {
	var result AvailabilityGroup
	err := s.client.Post(ctx, "/vps/availability-groups/", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create availability group: %w", err)
	}
	return &result, nil
}

// Get retrieves a specific availability group by UUID
func (s *AvailabilityGroups) Get(ctx context.Context, uuid string) (*AvailabilityGroup, error) {
	var result AvailabilityGroup
	err := s.client.Get(ctx, fmt.Sprintf("/vps/availability-groups/%s", uuid), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get availability group: %w", err)
	}
	return &result, nil
}

// List retrieves all availability groups for a project
func (s *AvailabilityGroups) List(ctx context.Context, projectID int) ([]AvailabilityGroup, error) {
	var result AvailabilityGroupListResponse
	err := s.client.Get(ctx, fmt.Sprintf("/vps/availability-groups/project/%d", projectID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list availability groups: %w", err)
	}
	return result.Groups, nil
}

// FindByName searches for an availability group by name within a project
func (s *AvailabilityGroups) FindByName(ctx context.Context, projectID int, name string) (*AvailabilityGroup, error) {
	groups, err := s.List(ctx, projectID)
	if err != nil {
		return nil, err
	}

	for _, group := range groups {
		if group.Name == name {
			return &group, nil
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("availability group with name %q not found in project %d", name, projectID),
	}
}

// Delete deletes an availability group by UUID
func (s *AvailabilityGroups) Delete(ctx context.Context, uuid string) error {
	err := s.client.Delete(ctx, fmt.Sprintf("/vps/availability-groups/%s", uuid))
	if err != nil {
		return fmt.Errorf("failed to delete availability group: %w", err)
	}
	return nil
}

// AddVPS adds a VPS to an availability group
func (s *AvailabilityGroups) AddVPS(ctx context.Context, uuid string, vpsID int) error {
	err := s.client.Post(ctx, fmt.Sprintf("/vps/availability-groups/%s/vps/%d", uuid, vpsID), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to add VPS %d to availability group: %w", vpsID, err)
	}
	return nil
}

// RemoveVPS removes a VPS from an availability group
func (s *AvailabilityGroups) RemoveVPS(ctx context.Context, uuid string, vpsID int) error {
	err := s.client.Delete(ctx, fmt.Sprintf("/vps/availability-groups/%s/vps/%d", uuid, vpsID))
	if err != nil {
		return fmt.Errorf("failed to remove VPS %d from availability group: %w", vpsID, err)
	}
	return nil
}

// MoveProject moves an availability group to another project of the organization
func (s *AvailabilityGroups) MoveProject(ctx context.Context, uuid string, projectID int) error {
	err := s.client.Post(ctx, fmt.Sprintf("/vps/availability-groups/%s/move-project", uuid), map[string]int{"project_id": projectID}, nil)
	if err != nil {
		return fmt.Errorf("failed to move availability group to project %d: %w", projectID, err)
	}
	return nil
}
