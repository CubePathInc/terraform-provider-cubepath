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

// networkCreateResponse represents the API response when creating a network
type networkCreateResponse struct {
	Detail    string `json:"detail"`
	NetworkID int    `json:"network_id"`
	Name      string `json:"name"`
}

// Create creates a new network
func (n *Networks) Create(ctx context.Context, req *CreateNetworkRequest) (*Network, error) {
	var createResp networkCreateResponse
	err := n.client.Post(ctx, "/networks/create_network", req, &createResp)
	if err != nil {
		return nil, fmt.Errorf("failed to create network: %w", err)
	}

	// Fetch the full network object by ID
	network, err := n.Get(ctx, createResp.NetworkID)
	if err != nil {
		return nil, fmt.Errorf("network created (ID %d) but failed to fetch details: %w", createResp.NetworkID, err)
	}
	return network, nil
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

// FindByName searches for a network by name within a project
func (n *Networks) FindByName(ctx context.Context, projectID int, name string) (*Network, error) {
	projects, err := n.client.Projects.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, projectResp := range projects {
		if projectResp.Project.ID == projectID {
			for _, network := range projectResp.Networks {
				if network.Name == name {
					return &network, nil
				}
			}
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("network with name %q not found in project %d", name, projectID),
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

// MoveProject moves a network to another project of the organization
func (n *Networks) MoveProject(ctx context.Context, networkID, projectID int) error {
	err := n.client.Post(ctx, fmt.Sprintf("/networks/%d/move-project", networkID), map[string]int{"project_id": projectID}, nil)
	if err != nil {
		return fmt.Errorf("failed to move network to project %d: %w", projectID, err)
	}
	return nil
}

// BGPPeer is an eBGP session between a network gateway and a speaker inside the network
type BGPPeer struct {
	ID               string   `json:"id"`
	NetworkID        int      `json:"network_id"`
	PeerType         string   `json:"peer_type"`
	PeerTarget       string   `json:"peer_target"`
	RemoteASN        int64    `json:"remote_asn"`
	MaxPrefix        int      `json:"max_prefix"`
	Description      *string  `json:"description"`
	Enabled          bool     `json:"enabled"`
	ResolvedPeerIP   *string  `json:"resolved_peer_ip"`
	LastState        *string  `json:"last_state"`
	PrefixesReceived *int     `json:"prefixes_received"`
	ReceivedPrefixes []string `json:"received_prefixes"`
}

// CreateBGPPeerRequest creates a BGP peer
type CreateBGPPeerRequest struct {
	PeerType    string  `json:"peer_type"`
	PeerTarget  string  `json:"peer_target"`
	RemoteASN   int64   `json:"remote_asn"`
	MaxPrefix   *int    `json:"max_prefix,omitempty"`
	Description *string `json:"description,omitempty"`
}

// UpdateBGPPeerRequest updates a BGP peer. Nil fields are left as they are.
type UpdateBGPPeerRequest struct {
	MaxPrefix   *int    `json:"max_prefix,omitempty"`
	Description *string `json:"description,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
}

// ListBGPPeers lists the BGP peers of a network
func (n *Networks) ListBGPPeers(ctx context.Context, networkID int) ([]BGPPeer, error) {
	var result []BGPPeer
	if err := n.client.Get(ctx, fmt.Sprintf("/networks/%d/bgp-peers", networkID), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBGPPeer finds a BGP peer by ID. It returns a 404 APIError when it does not exist.
func (n *Networks) GetBGPPeer(ctx context.Context, networkID int, peerID string) (*BGPPeer, error) {
	peers, err := n.ListBGPPeers(ctx, networkID)
	if err != nil {
		return nil, err
	}
	for i := range peers {
		if peers[i].ID == peerID {
			return &peers[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "BGP peer not found"}
}

// CreateBGPPeer creates a BGP peer and returns its ID
func (n *Networks) CreateBGPPeer(ctx context.Context, networkID int, req *CreateBGPPeerRequest) (string, error) {
	var result struct {
		PeerID string `json:"peer_id"`
	}
	if err := n.client.Post(ctx, fmt.Sprintf("/networks/%d/bgp-peers", networkID), req, &result); err != nil {
		return "", err
	}
	return result.PeerID, nil
}

// UpdateBGPPeer updates a BGP peer
func (n *Networks) UpdateBGPPeer(ctx context.Context, networkID int, peerID string, req *UpdateBGPPeerRequest) error {
	return n.client.Patch(ctx, fmt.Sprintf("/networks/%d/bgp-peers/%s", networkID, peerID), req, nil)
}

// DeleteBGPPeer deletes a BGP peer
func (n *Networks) DeleteBGPPeer(ctx context.Context, networkID int, peerID string) error {
	return n.client.Delete(ctx, fmt.Sprintf("/networks/%d/bgp-peers/%s", networkID, peerID))
}
