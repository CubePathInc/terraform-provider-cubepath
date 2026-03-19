package client

import (
	"context"
	"fmt"
)

// CDNService provides methods for managing CDN resources
type CDNService struct {
	client *Client
}

// NewCDNService creates a new CDN service
func NewCDNService(client *Client) *CDNService {
	return &CDNService{client: client}
}

// ListPlans retrieves available CDN plans
func (c *CDNService) ListPlans(ctx context.Context) ([]CDNPlan, error) {
	var result []CDNPlan
	err := c.client.Get(ctx, "/cdn/plans", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list CDN plans: %w", err)
	}
	return result, nil
}

// ListZones retrieves all CDN zones
func (c *CDNService) ListZones(ctx context.Context) ([]CDNZone, error) {
	var result []CDNZone
	err := c.client.Get(ctx, "/cdn/zones", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list CDN zones: %w", err)
	}
	return result, nil
}

// GetZone retrieves a CDN zone by UUID
func (c *CDNService) GetZone(ctx context.Context, uuid string) (*CDNZone, error) {
	var result CDNZone
	err := c.client.Get(ctx, fmt.Sprintf("/cdn/zones/%s", uuid), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get CDN zone: %w", err)
	}
	return &result, nil
}

// CreateZone creates a new CDN zone
func (c *CDNService) CreateZone(ctx context.Context, req *CreateCDNZoneRequest) (*CDNZone, error) {
	var result CDNZone
	err := c.client.Post(ctx, "/cdn/zones", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create CDN zone: %w", err)
	}
	return &result, nil
}

// UpdateZone updates a CDN zone
func (c *CDNService) UpdateZone(ctx context.Context, uuid string, req *UpdateCDNZoneRequest) error {
	return c.client.Patch(ctx, fmt.Sprintf("/cdn/zones/%s", uuid), req, nil)
}

// DeleteZone deletes a CDN zone
func (c *CDNService) DeleteZone(ctx context.Context, uuid string) error {
	return c.client.Delete(ctx, fmt.Sprintf("/cdn/zones/%s", uuid))
}

// ListOrigins retrieves all origins for a CDN zone
func (c *CDNService) ListOrigins(ctx context.Context, zoneUUID string) ([]CDNOrigin, error) {
	var result []CDNOrigin
	err := c.client.Get(ctx, fmt.Sprintf("/cdn/zones/%s/origins", zoneUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list CDN origins: %w", err)
	}
	return result, nil
}

// CreateOrigin creates a new origin for a CDN zone
func (c *CDNService) CreateOrigin(ctx context.Context, zoneUUID string, req *CreateCDNOriginRequest) (*CDNOrigin, error) {
	var result CDNOrigin
	err := c.client.Post(ctx, fmt.Sprintf("/cdn/zones/%s/origins", zoneUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create CDN origin: %w", err)
	}
	return &result, nil
}

// UpdateOrigin updates a CDN origin
func (c *CDNService) UpdateOrigin(ctx context.Context, zoneUUID, originUUID string, req *UpdateCDNOriginRequest) error {
	return c.client.Patch(ctx, fmt.Sprintf("/cdn/zones/%s/origins/%s", zoneUUID, originUUID), req, nil)
}

// DeleteOrigin deletes a CDN origin
func (c *CDNService) DeleteOrigin(ctx context.Context, zoneUUID, originUUID string) error {
	return c.client.Delete(ctx, fmt.Sprintf("/cdn/zones/%s/origins/%s", zoneUUID, originUUID))
}

// ListRules retrieves all edge rules for a CDN zone
func (c *CDNService) ListRules(ctx context.Context, zoneUUID string) ([]CDNRule, error) {
	var result []CDNRule
	err := c.client.Get(ctx, fmt.Sprintf("/cdn/zones/%s/rules", zoneUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list CDN rules: %w", err)
	}
	return result, nil
}

// GetRule retrieves a CDN edge rule
func (c *CDNService) GetRule(ctx context.Context, zoneUUID, ruleUUID string) (*CDNRule, error) {
	var result CDNRule
	err := c.client.Get(ctx, fmt.Sprintf("/cdn/zones/%s/rules/%s", zoneUUID, ruleUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get CDN rule: %w", err)
	}
	return &result, nil
}

// CreateRule creates a new edge rule
func (c *CDNService) CreateRule(ctx context.Context, zoneUUID string, req *CreateCDNRuleRequest) (*CDNRule, error) {
	var result CDNRule
	err := c.client.Post(ctx, fmt.Sprintf("/cdn/zones/%s/rules", zoneUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create CDN rule: %w", err)
	}
	return &result, nil
}

// UpdateRule updates a CDN edge rule
func (c *CDNService) UpdateRule(ctx context.Context, zoneUUID, ruleUUID string, req *UpdateCDNRuleRequest) error {
	return c.client.Patch(ctx, fmt.Sprintf("/cdn/zones/%s/rules/%s", zoneUUID, ruleUUID), req, nil)
}

// DeleteRule deletes a CDN edge rule
func (c *CDNService) DeleteRule(ctx context.Context, zoneUUID, ruleUUID string) error {
	return c.client.Delete(ctx, fmt.Sprintf("/cdn/zones/%s/rules/%s", zoneUUID, ruleUUID))
}

// ListWAFRules retrieves all WAF rules for a CDN zone
func (c *CDNService) ListWAFRules(ctx context.Context, zoneUUID string) ([]CDNRule, error) {
	var result []CDNRule
	err := c.client.Get(ctx, fmt.Sprintf("/cdn/zones/%s/waf-rules", zoneUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list CDN WAF rules: %w", err)
	}
	return result, nil
}

// GetWAFRule retrieves a CDN WAF rule
func (c *CDNService) GetWAFRule(ctx context.Context, zoneUUID, ruleUUID string) (*CDNRule, error) {
	var result CDNRule
	err := c.client.Get(ctx, fmt.Sprintf("/cdn/zones/%s/waf-rules/%s", zoneUUID, ruleUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get CDN WAF rule: %w", err)
	}
	return &result, nil
}

// CreateWAFRule creates a new WAF rule
func (c *CDNService) CreateWAFRule(ctx context.Context, zoneUUID string, req *CreateCDNRuleRequest) (*CDNRule, error) {
	var result CDNRule
	err := c.client.Post(ctx, fmt.Sprintf("/cdn/zones/%s/waf-rules", zoneUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create CDN WAF rule: %w", err)
	}
	return &result, nil
}

// UpdateWAFRule updates a CDN WAF rule
func (c *CDNService) UpdateWAFRule(ctx context.Context, zoneUUID, ruleUUID string, req *UpdateCDNRuleRequest) error {
	return c.client.Patch(ctx, fmt.Sprintf("/cdn/zones/%s/waf-rules/%s", zoneUUID, ruleUUID), req, nil)
}

// DeleteWAFRule deletes a CDN WAF rule
func (c *CDNService) DeleteWAFRule(ctx context.Context, zoneUUID, ruleUUID string) error {
	return c.client.Delete(ctx, fmt.Sprintf("/cdn/zones/%s/waf-rules/%s", zoneUUID, ruleUUID))
}
