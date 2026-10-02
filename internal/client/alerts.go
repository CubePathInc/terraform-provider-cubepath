package client

import (
	"context"
	"net/url"
	"strconv"
)

// AlertChannel is a notification channel of Cloud Alerts (Slack, Discord or email).
// Read endpoints return webhook URLs masked.
type AlertChannel struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config"`
	Enabled bool              `json:"enabled"`
}

// CreateAlertChannelRequest creates a notification channel
type CreateAlertChannelRequest struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Config  map[string]string `json:"config,omitempty"`
	Enabled *bool             `json:"enabled,omitempty"`
}

// UpdateAlertChannelRequest updates a notification channel. Nil fields are left as they are.
type UpdateAlertChannelRequest struct {
	Name    *string           `json:"name,omitempty"`
	Config  map[string]string `json:"config,omitempty"`
	Enabled *bool             `json:"enabled,omitempty"`
}

// AlertRuleAction is an action of an alert rule. The API only accepts notify actions.
type AlertRuleAction struct {
	ID            string `json:"id,omitempty"`
	ActionType    string `json:"action_type"`
	NotificatorID string `json:"notificator_id,omitempty"`
	Order         int    `json:"order"`
	Enabled       bool   `json:"enabled"`
}

// AlertRule is a Cloud Alert: a metric threshold on a server, availability group, bucket or organization
type AlertRule struct {
	ID              string            `json:"id"`
	ProjectID       int               `json:"project_id"`
	Name            string            `json:"name"`
	Description     *string           `json:"description"`
	TargetType      string            `json:"target_type"`
	TargetID        string            `json:"target_id"`
	TargetName      *string           `json:"target_name"`
	MetricType      string            `json:"metric_type"`
	Operator        string            `json:"operator"`
	Threshold       float64           `json:"threshold"`
	DurationSeconds int               `json:"duration_seconds"`
	CooldownSeconds int               `json:"cooldown_seconds"`
	Status          string            `json:"status"`
	LastTriggeredAt *string           `json:"last_triggered_at"`
	Actions         []AlertRuleAction `json:"actions"`
}

// CreateAlertRuleRequest creates an alert rule
type CreateAlertRuleRequest struct {
	ProjectID       int               `json:"project_id"`
	Name            string            `json:"name"`
	Description     *string           `json:"description,omitempty"`
	TargetType      string            `json:"target_type"`
	TargetID        string            `json:"target_id"`
	MetricType      string            `json:"metric_type"`
	Operator        string            `json:"operator"`
	Threshold       float64           `json:"threshold"`
	DurationSeconds int               `json:"duration_seconds"`
	CooldownSeconds int               `json:"cooldown_seconds"`
	Actions         []AlertRuleAction `json:"actions"`
}

// UpdateAlertRuleRequest updates an alert rule. Nil fields are left as they are; a
// non-nil Actions list replaces every action.
type UpdateAlertRuleRequest struct {
	Name            *string           `json:"name,omitempty"`
	Description     *string           `json:"description,omitempty"`
	TargetType      *string           `json:"target_type,omitempty"`
	TargetID        *string           `json:"target_id,omitempty"`
	MetricType      *string           `json:"metric_type,omitempty"`
	Operator        *string           `json:"operator,omitempty"`
	Threshold       *float64          `json:"threshold,omitempty"`
	DurationSeconds *int              `json:"duration_seconds,omitempty"`
	CooldownSeconds *int              `json:"cooldown_seconds,omitempty"`
	Status          *string           `json:"status,omitempty"`
	Actions         []AlertRuleAction `json:"actions,omitempty"`
}

// AlertsService handles Cloud Alerts API calls (alert rules and notification channels)
type AlertsService struct {
	client *Client
}

// NewAlertsService creates a new Cloud Alerts service
func NewAlertsService(client *Client) *AlertsService {
	return &AlertsService{client: client}
}

// ListChannels lists the organization's notification channels (webhook URLs masked)
func (s *AlertsService) ListChannels(ctx context.Context) ([]AlertChannel, error) {
	var result []AlertChannel
	if err := s.client.Get(ctx, "/triggers/notificators/", &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetChannel retrieves a notification channel (webhook URL masked)
func (s *AlertsService) GetChannel(ctx context.Context, id string) (*AlertChannel, error) {
	var result AlertChannel
	if err := s.client.Get(ctx, "/triggers/notificators/"+url.PathEscape(id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateChannel creates a notification channel
func (s *AlertsService) CreateChannel(ctx context.Context, req *CreateAlertChannelRequest) (*AlertChannel, error) {
	var result AlertChannel
	if err := s.client.Post(ctx, "/triggers/notificators/", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateChannel updates a notification channel
func (s *AlertsService) UpdateChannel(ctx context.Context, id string, req *UpdateAlertChannelRequest) (*AlertChannel, error) {
	var result AlertChannel
	if err := s.client.Put(ctx, "/triggers/notificators/"+url.PathEscape(id), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteChannel deletes a notification channel. It fails while an alert rule uses it.
func (s *AlertsService) DeleteChannel(ctx context.Context, id string) error {
	return s.client.Delete(ctx, "/triggers/notificators/"+url.PathEscape(id))
}

// ListRules lists the organization's alert rules, optionally for one project (0 = all)
func (s *AlertsService) ListRules(ctx context.Context, projectID int) ([]AlertRule, error) {
	path := "/triggers/"
	if projectID > 0 {
		path += "?project_id=" + strconv.Itoa(projectID)
	}
	var result []AlertRule
	if err := s.client.Get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetRule retrieves an alert rule with its actions
func (s *AlertsService) GetRule(ctx context.Context, id string) (*AlertRule, error) {
	var result AlertRule
	if err := s.client.Get(ctx, "/triggers/"+url.PathEscape(id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateRule creates an alert rule
func (s *AlertsService) CreateRule(ctx context.Context, req *CreateAlertRuleRequest) (*AlertRule, error) {
	var result AlertRule
	if err := s.client.Post(ctx, "/triggers/", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateRule updates an alert rule
func (s *AlertsService) UpdateRule(ctx context.Context, id string, req *UpdateAlertRuleRequest) (*AlertRule, error) {
	var result AlertRule
	if err := s.client.Put(ctx, "/triggers/"+url.PathEscape(id), req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteRule deletes an alert rule and its history
func (s *AlertsService) DeleteRule(ctx context.Context, id string) error {
	return s.client.Delete(ctx, "/triggers/"+url.PathEscape(id))
}
