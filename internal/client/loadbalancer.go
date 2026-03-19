package client

import (
	"context"
	"fmt"
)

// LoadBalancerService provides methods for managing load balancers
type LoadBalancerService struct {
	client *Client
}

// NewLoadBalancerService creates a new load balancer service
func NewLoadBalancerService(client *Client) *LoadBalancerService {
	return &LoadBalancerService{client: client}
}

// List retrieves all load balancers
func (l *LoadBalancerService) List(ctx context.Context) ([]LoadBalancer, error) {
	var result []LoadBalancer
	err := l.client.Get(ctx, "/loadbalancer/", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list load balancers: %w", err)
	}
	return result, nil
}

// Get retrieves a load balancer by UUID from the list
func (l *LoadBalancerService) Get(ctx context.Context, uuid string) (*LoadBalancer, error) {
	lbs, err := l.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, lb := range lbs {
		if lb.UUID == uuid {
			return &lb, nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: fmt.Sprintf("load balancer %s not found", uuid)}
}

// Create creates a new load balancer
func (l *LoadBalancerService) Create(ctx context.Context, req *CreateLoadBalancerRequest) (*LoadBalancer, error) {
	var result LoadBalancer
	err := l.client.Post(ctx, "/loadbalancer/", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer: %w", err)
	}
	return &result, nil
}

// Update updates a load balancer
func (l *LoadBalancerService) Update(ctx context.Context, uuid string, req *UpdateLoadBalancerRequest) (*LoadBalancer, error) {
	var result LoadBalancer
	err := l.client.Patch(ctx, fmt.Sprintf("/loadbalancer/%s", uuid), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update load balancer: %w", err)
	}
	return &result, nil
}

// Delete deletes a load balancer
func (l *LoadBalancerService) Delete(ctx context.Context, uuid string) error {
	return l.client.Delete(ctx, fmt.Sprintf("/loadbalancer/%s", uuid))
}

// Resize resizes a load balancer to a new plan
func (l *LoadBalancerService) Resize(ctx context.Context, uuid string, planName string) error {
	req := map[string]string{"plan_name": planName}
	return l.client.Post(ctx, fmt.Sprintf("/loadbalancer/%s/resize", uuid), req, nil)
}

// CreateListener creates a new listener on a load balancer
func (l *LoadBalancerService) CreateListener(ctx context.Context, lbUUID string, req *CreateListenerRequest) (*LBListener, error) {
	var result LBListener
	err := l.client.Post(ctx, fmt.Sprintf("/loadbalancer/%s/listeners", lbUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}
	return &result, nil
}

// UpdateListener updates a listener
func (l *LoadBalancerService) UpdateListener(ctx context.Context, lbUUID, listenerUUID string, req *UpdateListenerRequest) (*LBListener, error) {
	var result LBListener
	err := l.client.Patch(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s", lbUUID, listenerUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update listener: %w", err)
	}
	return &result, nil
}

// DeleteListener deletes a listener
func (l *LoadBalancerService) DeleteListener(ctx context.Context, lbUUID, listenerUUID string) error {
	return l.client.Delete(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s", lbUUID, listenerUUID))
}

// AddTarget adds a target to a listener
func (l *LoadBalancerService) AddTarget(ctx context.Context, lbUUID, listenerUUID string, req *AddTargetRequest) (*LBTarget, error) {
	var result LBTarget
	err := l.client.Post(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets", lbUUID, listenerUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to add target: %w", err)
	}
	return &result, nil
}

// UpdateTarget updates a target
func (l *LoadBalancerService) UpdateTarget(ctx context.Context, lbUUID, listenerUUID, targetUUID string, req *UpdateTargetRequest) (*LBTarget, error) {
	var result LBTarget
	err := l.client.Patch(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets/%s", lbUUID, listenerUUID, targetUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update target: %w", err)
	}
	return &result, nil
}

// RemoveTarget removes a target from a listener
func (l *LoadBalancerService) RemoveTarget(ctx context.Context, lbUUID, listenerUUID, targetUUID string) error {
	return l.client.Delete(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s/targets/%s", lbUUID, listenerUUID, targetUUID))
}

// ConfigureHealthCheck configures a health check for a listener
func (l *LoadBalancerService) ConfigureHealthCheck(ctx context.Context, lbUUID, listenerUUID string, req *HealthCheckConfig) error {
	return l.client.Put(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s/health-check", lbUUID, listenerUUID), req, nil)
}

// DeleteHealthCheck deletes a health check from a listener
func (l *LoadBalancerService) DeleteHealthCheck(ctx context.Context, lbUUID, listenerUUID string) error {
	return l.client.Delete(ctx, fmt.Sprintf("/loadbalancer/%s/listeners/%s/health-check", lbUUID, listenerUUID))
}

// ListPlans retrieves available load balancer plans
func (l *LoadBalancerService) ListPlans(ctx context.Context) ([]LBLocationPlans, error) {
	var result []LBLocationPlans
	err := l.client.Get(ctx, "/loadbalancer/plans", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list load balancer plans: %w", err)
	}
	return result, nil
}
