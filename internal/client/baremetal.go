package client

import (
	"context"
	"fmt"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
)

// BaremetalService provides methods for managing baremetal servers
type BaremetalService struct {
	client *Client
}

// NewBaremetalService creates a new baremetal service
func NewBaremetalService(client *Client) *BaremetalService {
	return &BaremetalService{client: client}
}

// Deploy deploys a new baremetal server
func (b *BaremetalService) Deploy(ctx context.Context, projectID int, req *CreateBaremetalRequest) (*TaskResponse, error) {
	var result TaskResponse
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/deploy/%d", projectID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to deploy baremetal: %w", err)
	}
	return &result, nil
}

// Get retrieves a specific baremetal server by ID
func (b *BaremetalService) Get(ctx context.Context, baremetalID int) (*Baremetal, error) {
	projects, err := b.client.Projects.List(ctx)
	if err != nil {
		return nil, err
	}

	// Search for baremetal in all projects
	for _, projectResp := range projects {
		for _, baremetal := range projectResp.Baremetals {
			if baremetal.ID == baremetalID {
				// project_id is not included in the nested baremetal objects
				if baremetal.ProjectID == 0 {
					baremetal.ProjectID = projectResp.Project.ID
				}
				return &baremetal, nil
			}
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("Baremetal with ID %d not found", baremetalID),
	}
}

// Update updates a baremetal server's hostname or label
func (b *BaremetalService) Update(ctx context.Context, baremetalID int, req *UpdateBaremetalRequest) error {
	err := b.client.Patch(ctx, fmt.Sprintf("/baremetal/update/%d", baremetalID), req, nil)
	if err != nil {
		return fmt.Errorf("failed to update baremetal: %w", err)
	}
	return nil
}

// Power sends a power control command to a baremetal server
// powerType can be: start_metal, stop_metal, restart_metal
func (b *BaremetalService) Power(ctx context.Context, baremetalID int, powerType string) (*TaskResponse, error) {
	var result TaskResponse
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/power/%s", baremetalID, powerType), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to send power command %s to baremetal: %w", powerType, err)
	}
	return &result, nil
}

// Start starts a stopped baremetal server
func (b *BaremetalService) Start(ctx context.Context, baremetalID int) (*TaskResponse, error) {
	return b.Power(ctx, baremetalID, "start_metal")
}

// Stop gracefully stops a running baremetal server
func (b *BaremetalService) Stop(ctx context.Context, baremetalID int) (*TaskResponse, error) {
	return b.Power(ctx, baremetalID, "stop_metal")
}

// Restart gracefully restarts a baremetal server
func (b *BaremetalService) Restart(ctx context.Context, baremetalID int) (*TaskResponse, error) {
	return b.Power(ctx, baremetalID, "restart_metal")
}

// UpdateMonitoring enables or disables monitoring for a baremetal server
func (b *BaremetalService) UpdateMonitoring(ctx context.Context, baremetalID int, enabled bool) error {
	data := map[string]bool{"monitoring_enable": enabled}
	err := b.client.Put(ctx, fmt.Sprintf("/baremetal/%d/monitoring", baremetalID), data, nil)
	if err != nil {
		return fmt.Errorf("failed to update baremetal monitoring: %w", err)
	}
	return nil
}

// Reinstall reinstalls the OS on a baremetal server
func (b *BaremetalService) Reinstall(ctx context.Context, baremetalID int, req *ReinstallBaremetalRequest) (*TaskResponse, error) {
	var result TaskResponse
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/reinstall", baremetalID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to reinstall baremetal: %w", err)
	}
	return &result, nil
}

// Rescue boots the baremetal server into rescue mode
func (b *BaremetalService) Rescue(ctx context.Context, baremetalID int) (*TaskResponse, error) {
	var result TaskResponse
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/rescue", baremetalID), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to boot baremetal into rescue mode: %w", err)
	}
	return &result, nil
}

// ResetBMC resets the BMC of a baremetal server
func (b *BaremetalService) ResetBMC(ctx context.Context, baremetalID int) (*TaskResponse, error) {
	var result TaskResponse
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/reset-bmc", baremetalID), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to reset baremetal BMC: %w", err)
	}
	return &result, nil
}

// WaitForBaremetalStatus waits for a baremetal to reach a specific status
func (b *BaremetalService) WaitForBaremetalStatus(ctx context.Context, baremetalID int, targetStatus []string, timeout time.Duration) (*Baremetal, error) {
	stateConf := &utils.StateChangeConf{
		Pending: []string{"pending", "provisioning", "deploying", "installing", "starting", "stopping", "rebooting"},
		Target:  targetStatus,
		Refresh: func() (interface{}, string, error) {
			baremetal, err := b.Get(ctx, baremetalID)
			if err != nil {
				if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			return baremetal, baremetal.Status, nil
		},
		Timeout:      timeout,
		PollInterval: 30 * time.Second,
		MinTimeout:   10 * time.Second,
	}

	result, err := stateConf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.(*Baremetal), nil
}

// SetProtection enables or disables protection. A protected server cannot be reinstalled.
func (b *BaremetalService) SetProtection(ctx context.Context, baremetalID int, enabled bool) error {
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/protection", baremetalID), map[string]bool{"enabled": enabled}, nil)
	if err != nil {
		return fmt.Errorf("failed to change baremetal protection: %w", err)
	}
	return nil
}

// MoveProject moves a baremetal server to another project of the organization
func (b *BaremetalService) MoveProject(ctx context.Context, baremetalID, projectID int) error {
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/move-project", baremetalID), map[string]int{"project_id": projectID}, nil)
	if err != nil {
		return fmt.Errorf("failed to move baremetal to project %d: %w", projectID, err)
	}
	return nil
}

// AddSSHKeys attaches SSH keys to a baremetal server. The keys are installed on the next reinstall.
func (b *BaremetalService) AddSSHKeys(ctx context.Context, baremetalID int, keyIDs []int) error {
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/ssh-keys", baremetalID), keyIDs, nil)
	if err != nil {
		return fmt.Errorf("failed to add SSH keys to baremetal: %w", err)
	}
	return nil
}

// RemoveSSHKey detaches an SSH key from a baremetal server
func (b *BaremetalService) RemoveSSHKey(ctx context.Context, baremetalID, keyID int) error {
	err := b.client.Delete(ctx, fmt.Sprintf("/baremetal/%d/ssh-keys/%d", baremetalID, keyID))
	if err != nil {
		return fmt.Errorf("failed to remove SSH key %d from baremetal: %w", keyID, err)
	}
	return nil
}

// AttachNetwork attaches a private network to a baremetal server
func (b *BaremetalService) AttachNetwork(ctx context.Context, baremetalID, networkID int) error {
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/%d/network", baremetalID), map[string]int{"network_id": networkID}, nil)
	if err != nil {
		return fmt.Errorf("failed to attach network to baremetal: %w", err)
	}
	return nil
}

// DetachNetwork detaches the private network from a baremetal server
func (b *BaremetalService) DetachNetwork(ctx context.Context, baremetalID int) error {
	err := b.client.Delete(ctx, fmt.Sprintf("/baremetal/%d/network", baremetalID))
	if err != nil {
		return fmt.Errorf("failed to detach network from baremetal: %w", err)
	}
	return nil
}

// BaremetalModelOffer is a server model on sale in a location
type BaremetalModelOffer struct {
	ModelName      string  `json:"model_name"`
	Price          float64 `json:"price"`
	DiscountValue  float64 `json:"discount_value"`
	DiscountType   string  `json:"discount_type"`
	CPU            string  `json:"cpu"`
	CPUSpecs       string  `json:"cpu_specs"`
	RAMSize        int     `json:"ram_size"`
	RAMType        string  `json:"ram_type"`
	DiskSize       string  `json:"disk_size"`
	DiskType       string  `json:"disk_type"`
	Port           int     `json:"port"`
	Setup          float64 `json:"setup"`
	StockAvailable int     `json:"stock_available"`
}

// BaremetalModelLocation groups the models on sale in a location
type BaremetalModelLocation struct {
	LocationName string                `json:"location_name"`
	Description  string                `json:"description"`
	Models       []BaremetalModelOffer `json:"models"`
}

// ListModels lists the server models on sale per location, with price and stock
func (b *BaremetalService) ListModels(ctx context.Context) ([]BaremetalModelLocation, error) {
	var result struct {
		Locations []BaremetalModelLocation `json:"locations"`
	}
	if err := b.client.Get(ctx, "/baremetal/models", &result); err != nil {
		return nil, fmt.Errorf("failed to list baremetal models: %w", err)
	}
	return result.Locations, nil
}
