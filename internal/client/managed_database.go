package client

import (
	"context"
	"encoding/json"
	"net/url"
)

// ManagedDatabasePlan is a managed database plan. Sizes and price are per node.
type ManagedDatabasePlan struct {
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Engine       string  `json:"engine"`
	CPU          int     `json:"cpu"`
	MemoryGB     int     `json:"memory_gb"`
	StorageGB    int     `json:"storage_gb"`
	MaxReplicas  int     `json:"max_replicas"`
	PricePerHour float64 `json:"price_per_hour"`
}

// ManagedDatabasePlanLocation groups the plans of one location
type ManagedDatabasePlanLocation struct {
	LocationName        string                `json:"location_name"`
	LocationDescription string                `json:"location_description"`
	Plans               []ManagedDatabasePlan `json:"plans"`
}

// ManagedDatabaseLocation is the location of a managed database
type ManagedDatabaseLocation struct {
	ID           int    `json:"id"`
	LocationName string `json:"location_name"`
	Description  string `json:"description"`
}

// ManagedDatabase is a managed MySQL, PostgreSQL or Valkey instance
type ManagedDatabase struct {
	UUID                string                   `json:"uuid"`
	ProjectID           int                      `json:"project_id"`
	Name                string                   `json:"name"`
	Label               *string                  `json:"label"`
	Engine              string                   `json:"engine"`
	Version             string                   `json:"version"`
	Topology            string                   `json:"topology"`
	Replicas            int                      `json:"replicas"`
	Status              string                   `json:"status"`
	EndpointHost        *string                  `json:"endpoint_host"`
	EndpointPort        *int                     `json:"endpoint_port"`
	Plan                *ManagedDatabasePlan     `json:"plan"`
	Location            *ManagedDatabaseLocation `json:"location"`
	BackupEnabled       bool                     `json:"backup_enabled"`
	BackupScheduleCron  *string                  `json:"backup_schedule_cron"`
	BackupRetentionDays int                      `json:"backup_retention_days"`
	BillingType         string                   `json:"billing_type"`
	Protected           bool                     `json:"protected"`
}

// ManagedDatabaseBackup is the backup policy sent when creating a managed database
type ManagedDatabaseBackup struct {
	ScheduleCron  string `json:"schedule_cron"`
	RetentionDays int    `json:"retention_days,omitempty"`
}

// CreateManagedDatabaseRequest creates a managed database
type CreateManagedDatabaseRequest struct {
	ProjectID int                    `json:"project_id"`
	Name      string                 `json:"name"`
	Engine    string                 `json:"engine"`
	Version   string                 `json:"version"`
	PlanUUID  string                 `json:"plan_uuid"`
	Replicas  *int                   `json:"replicas,omitempty"`
	Topology  *string                `json:"topology,omitempty"`
	Backup    *ManagedDatabaseBackup `json:"backup,omitempty"`
}

// ManagedDatabaseBackupUpdate changes the backup policy. Nil fields are left as they are.
type ManagedDatabaseBackupUpdate struct {
	Enabled       *bool   `json:"enabled,omitempty"`
	ScheduleCron  *string `json:"schedule_cron,omitempty"`
	RetentionDays *int    `json:"retention_days,omitempty"`
}

// UpdateManagedDatabaseRequest changes name, label or backup policy. Nil fields are left as they are.
type UpdateManagedDatabaseRequest struct {
	Name   *string                      `json:"name,omitempty"`
	Label  *string                      `json:"label,omitempty"`
	Backup *ManagedDatabaseBackupUpdate `json:"backup,omitempty"`
}

// ScaleManagedDatabaseRequest scales a managed database. Exactly one field must be set.
type ScaleManagedDatabaseRequest struct {
	Replicas *int    `json:"replicas,omitempty"`
	PlanUUID *string `json:"plan_uuid,omitempty"`
}

// ManagedDatabaseCredentials are the admin connection credentials
type ManagedDatabaseCredentials struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	URI      string `json:"uri"`
}

// ManagedDatabaseConfigParam describes one tunable configuration parameter
type ManagedDatabaseConfigParam struct {
	Type            string          `json:"type"`
	Default         json.RawMessage `json:"default"`
	Value           json.RawMessage `json:"value"`
	ValueSource     string          `json:"value_source"`
	RequiresRestart bool            `json:"requires_restart"`
	Description     string          `json:"description"`
}

// ManagedDatabaseConfig lists the tunable parameters of an engine
type ManagedDatabaseConfig struct {
	Engine string                                `json:"engine"`
	Params map[string]ManagedDatabaseConfigParam `json:"params"`
}

// ManagedDatabaseLogicalDatabase is a database inside a managed database instance
type ManagedDatabaseLogicalDatabase struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ManagedDatabaseUser is a user inside a managed database instance. The password is
// only returned when the user is created.
type ManagedDatabaseUser struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Status   string `json:"status"`
}

// ManagedDatabaseService handles managed database API calls
type ManagedDatabaseService struct {
	client *Client
}

// NewManagedDatabaseService creates a new managed database service
func NewManagedDatabaseService(client *Client) *ManagedDatabaseService {
	return &ManagedDatabaseService{client: client}
}

func mdbPath(uuid string) string {
	return "/managed-databases/" + url.PathEscape(uuid)
}

// ListPlans lists the active plans grouped by location. An empty engine lists every engine.
func (s *ManagedDatabaseService) ListPlans(ctx context.Context, engine string) ([]ManagedDatabasePlanLocation, error) {
	path := "/managed-database-plans/"
	if engine != "" {
		path += "?engine=" + url.QueryEscape(engine)
	}
	var result []ManagedDatabasePlanLocation
	if err := s.client.Get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// List lists the organization's managed databases
func (s *ManagedDatabaseService) List(ctx context.Context) ([]ManagedDatabase, error) {
	var result []ManagedDatabase
	if err := s.client.Get(ctx, "/managed-databases/", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Get retrieves a managed database by UUID
func (s *ManagedDatabaseService) Get(ctx context.Context, uuid string) (*ManagedDatabase, error) {
	var result ManagedDatabase
	if err := s.client.Get(ctx, mdbPath(uuid), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Create creates a managed database; it starts as provisioning. Only the UUID is used from the answer.
func (s *ManagedDatabaseService) Create(ctx context.Context, req *CreateManagedDatabaseRequest) (*ManagedDatabase, error) {
	var result ManagedDatabase
	if err := s.client.Post(ctx, "/managed-databases/", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Update changes name, label or backup policy
func (s *ManagedDatabaseService) Update(ctx context.Context, uuid string, req *UpdateManagedDatabaseRequest) error {
	return s.client.Patch(ctx, mdbPath(uuid), req, nil)
}

// Delete starts the deletion of a managed database and all its data
func (s *ManagedDatabaseService) Delete(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, mdbPath(uuid))
}

// SetProtection enables or disables deletion protection
func (s *ManagedDatabaseService) SetProtection(ctx context.Context, uuid string, enabled bool) error {
	return s.client.Post(ctx, mdbPath(uuid)+"/protection", map[string]bool{"enabled": enabled}, nil)
}

// Scale changes the replica count or the plan
func (s *ManagedDatabaseService) Scale(ctx context.Context, uuid string, req *ScaleManagedDatabaseRequest) error {
	return s.client.Post(ctx, mdbPath(uuid)+"/scale", req, nil)
}

// GetCredentials retrieves the admin connection credentials
func (s *ManagedDatabaseService) GetCredentials(ctx context.Context, uuid string) (*ManagedDatabaseCredentials, error) {
	var result ManagedDatabaseCredentials
	if err := s.client.Get(ctx, mdbPath(uuid)+"/credentials", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetConfig describes the tunable configuration parameters
func (s *ManagedDatabaseService) GetConfig(ctx context.Context, uuid string) (*ManagedDatabaseConfig, error) {
	var result ManagedDatabaseConfig
	if err := s.client.Get(ctx, mdbPath(uuid)+"/config", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateConfig changes configuration parameters
func (s *ManagedDatabaseService) UpdateConfig(ctx context.Context, uuid string, params map[string]interface{}) error {
	return s.client.Patch(ctx, mdbPath(uuid)+"/config", map[string]interface{}{"params": params}, nil)
}

// ListDatabases lists the logical databases of an instance
func (s *ManagedDatabaseService) ListDatabases(ctx context.Context, uuid string) ([]ManagedDatabaseLogicalDatabase, error) {
	var result []ManagedDatabaseLogicalDatabase
	if err := s.client.Get(ctx, mdbPath(uuid)+"/databases", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetDatabase finds a logical database by UUID. The API has no detail route, so it searches the list.
func (s *ManagedDatabaseService) GetDatabase(ctx context.Context, uuid, dbUUID string) (*ManagedDatabaseLogicalDatabase, error) {
	dbs, err := s.ListDatabases(ctx, uuid)
	if err != nil {
		return nil, err
	}
	for i := range dbs {
		if dbs[i].UUID == dbUUID {
			return &dbs[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "Database not found"}
}

// CreateDatabase creates a logical database
func (s *ManagedDatabaseService) CreateDatabase(ctx context.Context, uuid, name string) (*ManagedDatabaseLogicalDatabase, error) {
	var result ManagedDatabaseLogicalDatabase
	if err := s.client.Post(ctx, mdbPath(uuid)+"/databases", map[string]string{"name": name}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteDatabase deletes a logical database and its data
func (s *ManagedDatabaseService) DeleteDatabase(ctx context.Context, uuid, dbUUID string) error {
	return s.client.Delete(ctx, mdbPath(uuid)+"/databases/"+url.PathEscape(dbUUID))
}

// ListUsers lists the users of an instance (never passwords)
func (s *ManagedDatabaseService) ListUsers(ctx context.Context, uuid string) ([]ManagedDatabaseUser, error) {
	var result []ManagedDatabaseUser
	if err := s.client.Get(ctx, mdbPath(uuid)+"/users", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetUser finds a user by UUID. The API has no detail route, so it searches the list.
func (s *ManagedDatabaseService) GetUser(ctx context.Context, uuid, userUUID string) (*ManagedDatabaseUser, error) {
	users, err := s.ListUsers(ctx, uuid)
	if err != nil {
		return nil, err
	}
	for i := range users {
		if users[i].UUID == userUUID {
			return &users[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "User not found"}
}

// CreateUser creates a user. An empty password lets the API generate one; either way the
// password is only returned by this call.
func (s *ManagedDatabaseService) CreateUser(ctx context.Context, uuid, username, password string) (*ManagedDatabaseUser, error) {
	body := map[string]string{"username": username}
	if password != "" {
		body["password"] = password
	}
	var result ManagedDatabaseUser
	if err := s.client.Post(ctx, mdbPath(uuid)+"/users", body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteUser deletes a user
func (s *ManagedDatabaseService) DeleteUser(ctx context.Context, uuid, userUUID string) error {
	return s.client.Delete(ctx, mdbPath(uuid)+"/users/"+url.PathEscape(userUUID))
}
