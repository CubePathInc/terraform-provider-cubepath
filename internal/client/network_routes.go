package client

import (
	"context"
	"fmt"
)

// NetworkRoute represents a route in a network
type NetworkRoute struct {
	ID                string `json:"id"`
	NetworkID         int    `json:"network_id"`
	Destination       string `json:"destination"`
	NextHopType       string `json:"next_hop_type"`
	NextHopTarget     string `json:"next_hop_target"`
	ResolvedNextHopIP string `json:"resolved_next_hop_ip"`
	Description       string `json:"description"`
	CreatedAt         string `json:"created_at"`
}

// CreateNetworkRouteRequest represents a request to create a network route
type CreateNetworkRouteRequest struct {
	Destination   string  `json:"destination"`
	NextHopType   string  `json:"next_hop_type"`
	NextHopTarget string  `json:"next_hop_target"`
	Description   *string `json:"description,omitempty"`
}

// NetworkRouteService provides methods for managing network routes
type NetworkRouteService struct {
	client *Client
}

// NewNetworkRouteService creates a new NetworkRouteService
func NewNetworkRouteService(client *Client) *NetworkRouteService {
	return &NetworkRouteService{client: client}
}

// List lists all routes for a network
func (s *NetworkRouteService) List(ctx context.Context, networkID int) ([]NetworkRoute, error) {
	var result []NetworkRoute
	err := s.client.Get(ctx, fmt.Sprintf("/networks/%d/routes", networkID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list network routes: %w", err)
	}
	return result, nil
}

// Create creates a new route in a network
func (s *NetworkRouteService) Create(ctx context.Context, networkID int, req *CreateNetworkRouteRequest) (*NetworkRoute, error) {
	var result NetworkRoute
	err := s.client.Post(ctx, fmt.Sprintf("/networks/%d/routes", networkID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create network route: %w", err)
	}
	return &result, nil
}

// Delete deletes a network route
func (s *NetworkRouteService) Delete(ctx context.Context, networkID int, routeID string) error {
	err := s.client.Delete(ctx, fmt.Sprintf("/networks/%d/routes/%s", networkID, routeID))
	if err != nil {
		return fmt.Errorf("failed to delete network route: %w", err)
	}
	return nil
}
