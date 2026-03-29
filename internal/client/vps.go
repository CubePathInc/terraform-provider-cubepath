package client

import (
	"context"
	"fmt"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
)

// VPSService provides methods for managing VPS instances
type VPSService struct {
	client *Client
}

// NewVPSService creates a new VPS service
func NewVPSService(client *Client) *VPSService {
	return &VPSService{client: client}
}

// Create creates a new VPS
func (v *VPSService) Create(ctx context.Context, projectID int, req *CreateVPSRequest) (*TaskResponse, error) {
	var result TaskResponse
	err := v.client.Post(ctx, fmt.Sprintf("/vps/create/%d", projectID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPS: %w", err)
	}
	return &result, nil
}

// Get retrieves a specific VPS by ID
func (v *VPSService) Get(ctx context.Context, vpsID int) (*VPS, error) {
	projects, err := v.client.Projects.List(ctx)
	if err != nil {
		return nil, err
	}

	// Search for VPS in all projects
	for _, projectResp := range projects {
		for _, vps := range projectResp.VPS {
			if vps.ID == vpsID {
				return &vps, nil
			}
		}
	}

	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("VPS with ID %d not found", vpsID),
	}
}

// Destroy destroys a VPS
func (v *VPSService) Destroy(ctx context.Context, vpsID int) (*TaskResponse, error) {
	var result TaskResponse
	err := v.client.Post(ctx, fmt.Sprintf("/vps/destroy/%d", vpsID), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to destroy VPS: %w", err)
	}
	return &result, nil
}

// Resize resizes a VPS to a different plan
func (v *VPSService) Resize(ctx context.Context, vpsID int, planName string) (*TaskResponse, error) {
	var result TaskResponse
	err := v.client.Post(ctx, fmt.Sprintf("/vps/resize/vps_id/%d/resize_plan/%s", vpsID, planName), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to resize VPS: %w", err)
	}
	return &result, nil
}

// ChangePassword changes the root password of a VPS
func (v *VPSService) ChangePassword(ctx context.Context, vpsID int, newPassword string) (*TaskResponse, error) {
	var result TaskResponse
	data := map[string]string{"password": newPassword}
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/change-password", vpsID), data, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to change VPS password: %w", err)
	}
	return &result, nil
}

// Rename renames a VPS
func (v *VPSService) Rename(ctx context.Context, vpsID int, newName string) error {
	data := map[string]string{"name": newName}
	err := v.client.Put(ctx, fmt.Sprintf("/vps/%d", vpsID), data, nil)
	if err != nil {
		return fmt.Errorf("failed to rename VPS: %w", err)
	}
	return nil
}

// WaitForVPSStatus waits for a VPS to reach a specific status
func (v *VPSService) WaitForVPSStatus(ctx context.Context, vpsID int, targetStatus []string, timeout time.Duration) (*VPS, error) {
	stateConf := &utils.StateChangeConf{
		Pending: []string{"pending", "creating", "deploying", "provisioning", "starting", "stopping"},
		Target:  targetStatus,
		Refresh: func() (interface{}, string, error) {
			vps, err := v.Get(ctx, vpsID)
			if err != nil {
				// If not found and we're waiting for deletion, that's OK
				if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			return vps, vps.Status, nil
		},
		Timeout:      timeout,
		PollInterval: 10 * time.Second,
		MinTimeout:   5 * time.Second,
	}

	result, err := stateConf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.(*VPS), nil
}

// WaitForVPSDestroy waits for a VPS to be destroyed
func (v *VPSService) WaitForVPSDestroy(ctx context.Context, vpsID int, timeout time.Duration) error {
	stateConf := &utils.StateChangeConf{
		Pending: []string{"running", "stopped", "stopping", "deleting"},
		Target:  []string{"deleted"},
		Refresh: func() (interface{}, string, error) {
			_, err := v.Get(ctx, vpsID)
			if err != nil {
				// If not found, it's deleted
				if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			// Still exists
			return nil, "deleting", nil
		},
		Timeout:      timeout,
		PollInterval: 10 * time.Second,
		MinTimeout:   5 * time.Second,
	}

	_, err := stateConf.WaitForState(ctx)
	return err
}

// VPSTemplatesResponse represents the response from /vps/templates
type VPSTemplatesResponse struct {
	OperatingSystems []VPSTemplate `json:"operating_systems"`
}

// Templates lists available VPS templates
func (v *VPSService) Templates(ctx context.Context) (*VPSTemplatesResponse, error) {
	var result VPSTemplatesResponse
	err := v.client.Get(ctx, "/vps/templates", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list VPS templates: %w", err)
	}
	return &result, nil
}

// Power sends a power control command to a VPS
// powerType can be: start_vps, stop_vps, restart_vps, reset_vps
func (v *VPSService) Power(ctx context.Context, vpsID int, powerType string) (*TaskResponse, error) {
	var result TaskResponse
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/power/%s", vpsID, powerType), nil, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to send power command %s to VPS: %w", powerType, err)
	}
	return &result, nil
}

// Start starts a stopped VPS
func (v *VPSService) Start(ctx context.Context, vpsID int) (*TaskResponse, error) {
	return v.Power(ctx, vpsID, "start_vps")
}

// Stop gracefully stops a running VPS
func (v *VPSService) Stop(ctx context.Context, vpsID int) (*TaskResponse, error) {
	return v.Power(ctx, vpsID, "stop_vps")
}

// Restart gracefully restarts a VPS
func (v *VPSService) Restart(ctx context.Context, vpsID int) (*TaskResponse, error) {
	return v.Power(ctx, vpsID, "restart_vps")
}

// Reset forcefully resets a VPS (like pressing the reset button)
func (v *VPSService) Reset(ctx context.Context, vpsID int) (*TaskResponse, error) {
	return v.Power(ctx, vpsID, "reset_vps")
}
