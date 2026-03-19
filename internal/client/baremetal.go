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

// Destroy cancels/terminates a baremetal server
func (b *BaremetalService) Destroy(ctx context.Context, baremetalID int) (*TaskResponse, error) {
	var result TaskResponse
	err := b.client.Post(ctx, fmt.Sprintf("/baremetal/cancel/%d", baremetalID), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel baremetal: %w", err)
	}
	return &result, nil
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

// WaitForBaremetalDestroy waits for a baremetal to be destroyed
func (b *BaremetalService) WaitForBaremetalDestroy(ctx context.Context, baremetalID int, timeout time.Duration) error {
	stateConf := &utils.StateChangeConf{
		Pending: []string{"running", "stopped", "stopping", "cancelling", "active"},
		Target:  []string{"deleted", "cancelled"},
		Refresh: func() (interface{}, string, error) {
			_, err := b.Get(ctx, baremetalID)
			if err != nil {
				if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			return nil, "cancelling", nil
		},
		Timeout:      timeout,
		PollInterval: 30 * time.Second,
		MinTimeout:   10 * time.Second,
	}

	_, err := stateConf.WaitForState(ctx)
	return err
}
