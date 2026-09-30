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
				// project_id is not included in the nested VPS objects
				if vps.ProjectID == 0 {
					vps.ProjectID = projectResp.Project.ID
				}
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

// UpdateVPSRequest changes a VPS hostname and/or label. Nil fields are left as they are.
type UpdateVPSRequest struct {
	Name  *string `json:"name,omitempty"`
	Label *string `json:"label,omitempty"`
}

// Update changes a VPS hostname and/or label (PATCH /vps/update/{id}).
func (v *VPSService) Update(ctx context.Context, vpsID int, req *UpdateVPSRequest) error {
	err := v.client.Patch(ctx, fmt.Sprintf("/vps/update/%d", vpsID), req, nil)
	if err != nil {
		return fmt.Errorf("failed to update VPS: %w", err)
	}
	return nil
}

// Rename changes a VPS hostname.
func (v *VPSService) Rename(ctx context.Context, vpsID int, newName string) error {
	return v.Update(ctx, vpsID, &UpdateVPSRequest{Name: &newName})
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

// SetProtection enables or disables destruction protection. A protected VPS cannot be
// destroyed or reinstalled.
func (v *VPSService) SetProtection(ctx context.Context, vpsID int, enabled bool) error {
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/protection", vpsID), map[string]bool{"enabled": enabled}, nil)
	if err != nil {
		return fmt.Errorf("failed to change VPS protection: %w", err)
	}
	return nil
}

// MoveProject moves a VPS to another project of the organization
func (v *VPSService) MoveProject(ctx context.Context, vpsID, projectID int) error {
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/move-project", vpsID), map[string]int{"project_id": projectID}, nil)
	if err != nil {
		return fmt.Errorf("failed to move VPS to project %d: %w", projectID, err)
	}
	return nil
}

// AddSSHKeys attaches SSH keys to a VPS. The keys are installed on the next reinstall.
func (v *VPSService) AddSSHKeys(ctx context.Context, vpsID int, keyIDs []int) error {
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/ssh-keys", vpsID), keyIDs, nil)
	if err != nil {
		return fmt.Errorf("failed to add SSH keys to VPS: %w", err)
	}
	return nil
}

// RemoveSSHKey detaches an SSH key from a VPS
func (v *VPSService) RemoveSSHKey(ctx context.Context, vpsID, keyID int) error {
	err := v.client.Delete(ctx, fmt.Sprintf("/vps/%d/ssh-keys/%d", vpsID, keyID))
	if err != nil {
		return fmt.Errorf("failed to remove SSH key %d from VPS: %w", keyID, err)
	}
	return nil
}

// AttachNetwork attaches a private network to a VPS. The NIC is added asynchronously and
// takes effect after a restart.
func (v *VPSService) AttachNetwork(ctx context.Context, vpsID, networkID int) error {
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/network", vpsID), map[string]int{"network_id": networkID}, nil)
	if err != nil {
		return fmt.Errorf("failed to attach network to VPS: %w", err)
	}
	return nil
}

// DetachNetwork detaches the private network from a VPS
func (v *VPSService) DetachNetwork(ctx context.Context, vpsID int) error {
	err := v.client.Delete(ctx, fmt.Sprintf("/vps/%d/network", vpsID))
	if err != nil {
		return fmt.Errorf("failed to detach network from VPS: %w", err)
	}
	return nil
}

// VPSBackupSettings are the automatic backup settings of a VPS
type VPSBackupSettings struct {
	Enabled       bool `json:"enabled"`
	ScheduleHour  int  `json:"schedule_hour"`
	RetentionDays int  `json:"retention_days"`
	MaxBackups    int  `json:"max_backups"`
}

// GetBackupSettings retrieves the automatic backup settings (the defaults when never set)
func (v *VPSService) GetBackupSettings(ctx context.Context, vpsID int) (*VPSBackupSettings, error) {
	var result VPSBackupSettings
	err := v.client.Get(ctx, fmt.Sprintf("/vps/%d/backup/settings", vpsID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to read VPS backup settings: %w", err)
	}
	return &result, nil
}

// UpdateBackupSettings replaces the automatic backup settings
func (v *VPSService) UpdateBackupSettings(ctx context.Context, vpsID int, settings *VPSBackupSettings) (*VPSBackupSettings, error) {
	var result VPSBackupSettings
	err := v.client.Put(ctx, fmt.Sprintf("/vps/%d/backup/settings", vpsID), settings, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update VPS backup settings: %w", err)
	}
	return &result, nil
}

// VPSISO is an ISO image that can be attached to a VPS
type VPSISO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Filename    string `json:"filename"`
	FileSize    int64  `json:"file_size"`
	IsMounted   bool   `json:"is_mounted"`
}

// ListISOs lists the ISOs available to a VPS and whether each one is mounted
func (v *VPSService) ListISOs(ctx context.Context, vpsID int) ([]VPSISO, error) {
	var result struct {
		Items []VPSISO `json:"items"`
	}
	err := v.client.Get(ctx, fmt.Sprintf("/vps/%d/isos", vpsID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list VPS ISOs: %w", err)
	}
	return result.Items, nil
}

// AttachISO attaches an ISO to a VPS (asynchronous)
func (v *VPSService) AttachISO(ctx context.Context, vpsID int, isoID string) error {
	err := v.client.Post(ctx, fmt.Sprintf("/vps/%d/iso", vpsID), map[string]string{"iso_id": isoID}, nil)
	if err != nil {
		return fmt.Errorf("failed to attach ISO to VPS: %w", err)
	}
	return nil
}

// DetachISO detaches the mounted ISO from a VPS (asynchronous)
func (v *VPSService) DetachISO(ctx context.Context, vpsID int) error {
	err := v.client.Delete(ctx, fmt.Sprintf("/vps/%d/iso", vpsID))
	if err != nil {
		return fmt.Errorf("failed to detach ISO from VPS: %w", err)
	}
	return nil
}
