package client

import (
	"context"
	"fmt"
)

// NATGateway represents a NAT gateway
type NATGateway struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Label        string `json:"label"`
	Status       string `json:"status"`
	PlanName     string `json:"plan_name"`
	NetworkID    int    `json:"network_id"`
	ProjectID    int    `json:"project_id"`
	LocationName string `json:"location_name"`
	NetworkCIDR  string `json:"network_cidr"`
	PrivateIP    string `json:"private_ip"`
	Protected    bool   `json:"protected"`
	CreatedAt    string `json:"created_at"`
}

// NATGatewayPlan represents a NAT gateway plan
type NATGatewayPlan struct {
	Name         string `json:"name"`
	PricePerHour string `json:"price_per_hour"`
}

// CreateNATGatewayRequest represents a request to create a NAT gateway
type CreateNATGatewayRequest struct {
	Name      string  `json:"name"`
	Label     *string `json:"label,omitempty"`
	PlanName  string  `json:"plan_name"`
	NetworkID int     `json:"network_id"`
	ProjectID *int    `json:"project_id,omitempty"`
}

// UpdateNATGatewayRequest represents a request to update a NAT gateway
type UpdateNATGatewayRequest struct {
	Name  *string `json:"name,omitempty"`
	Label *string `json:"label,omitempty"`
}

// ResizeNATGatewayRequest represents a request to resize a NAT gateway
type ResizeNATGatewayRequest struct {
	PlanName string `json:"plan_name"`
}

// NATGatewayService provides methods for managing NAT gateways
type NATGatewayService struct {
	client *Client
}

// NewNATGatewayService creates a new NATGatewayService
func NewNATGatewayService(client *Client) *NATGatewayService {
	return &NATGatewayService{client: client}
}

// Create creates a new NAT gateway
func (n *NATGatewayService) Create(ctx context.Context, req *CreateNATGatewayRequest) (*NATGateway, error) {
	var result NATGateway
	err := n.client.Post(ctx, "/nat-gateway/", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create NAT gateway: %w", err)
	}
	return &result, nil
}

// Get retrieves a NAT gateway by UUID
func (n *NATGatewayService) Get(ctx context.Context, uuid string) (*NATGateway, error) {
	var result NATGateway
	err := n.client.Get(ctx, fmt.Sprintf("/nat-gateway/%s", uuid), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get NAT gateway: %w", err)
	}
	return &result, nil
}

// Update updates a NAT gateway's name and/or label
func (n *NATGatewayService) Update(ctx context.Context, uuid string, req *UpdateNATGatewayRequest) (*NATGateway, error) {
	var result NATGateway
	err := n.client.Patch(ctx, fmt.Sprintf("/nat-gateway/%s", uuid), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update NAT gateway: %w", err)
	}
	return &result, nil
}

// Delete deletes a NAT gateway
func (n *NATGatewayService) Delete(ctx context.Context, uuid string) error {
	err := n.client.Delete(ctx, fmt.Sprintf("/nat-gateway/%s", uuid))
	if err != nil {
		return fmt.Errorf("failed to delete NAT gateway: %w", err)
	}
	return nil
}

// Resize resizes a NAT gateway to a new plan
func (n *NATGatewayService) Resize(ctx context.Context, uuid string, planName string) error {
	req := ResizeNATGatewayRequest{PlanName: planName}
	err := n.client.Post(ctx, fmt.Sprintf("/nat-gateway/%s/resize", uuid), req, nil)
	if err != nil {
		return fmt.Errorf("failed to resize NAT gateway: %w", err)
	}
	return nil
}

// ListPlans retrieves available NAT gateway plans
func (n *NATGatewayService) ListPlans(ctx context.Context) ([]NATGatewayPlan, error) {
	var result []NATGatewayPlan
	err := n.client.Get(ctx, "/nat-gateway/plans", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list NAT gateway plans: %w", err)
	}
	return result, nil
}
