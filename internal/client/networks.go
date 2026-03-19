package client

import (
	"context"
	"fmt"
)

// Networks provides methods for managing networks
type Networks struct {
	client *Client
}

// NewNetworks creates a new networks service
func NewNetworks(client *Client) *Networks {
	return &Networks{client: client}
}

// Create creates a new network
func (n *Networks) Create(ctx context.Context, req *CreateNetworkRequest) (*Network, error) {
	var result Network
	err := n.client.Post(ctx, "/networks/create_network", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}
	return &result, nil
}

// Get retrieves a specific network by ID
func (n *Networks) Get(ctx context.Context, networkID int) (*Network, error) {
	projects, err := n.client.Projects.List(ctx)
	if err != nil {
		return nil, err
	}

	// Search for network in all projects
	for _, projectResp := range projects {
		for _, network := range projectResp.Networks {
			if network.ID == networkID {
				return &network, nil
			}
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("network with ID %d not found", networkID),
	}
}

// Update updates a network (name and label)
func (n *Networks) Update(ctx context.Context, networkID int, name, label string) (*Network, error) {
	var result Network
	data := map[string]string{}
	if name != "" {
		data["name"] = name
	}
	if label != "" {
		data["label"] = label
	}

	err := n.client.Put(ctx, fmt.Sprintf("/networks/%d", networkID), data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update network: %w", err)
	}
	return &result, nil
}

// Delete deletes a network by ID
func (n *Networks) Delete(ctx context.Context, networkID int) error {
	err := n.client.Delete(ctx, fmt.Sprintf("/networks/%d", networkID))
	if err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}
	return nil
}
