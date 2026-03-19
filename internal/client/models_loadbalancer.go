package client

import "encoding/json"

// LoadBalancer represents a load balancer
type LoadBalancer struct {
	UUID             string          `json:"uuid"`
	Name             string          `json:"name"`
	Label            string          `json:"label"`
	Status           string          `json:"status"`
	LocationName     string          `json:"location_name"`
	Plan             *LBPlan         `json:"plan,omitempty"`
	PlanName         string          `json:"plan_name"`
	FloatingIPs      []LBFloatingIP  `json:"floating_ips"`
	Listeners        []LBListener    `json:"listeners"`
	ListenersCount   int             `json:"listeners_count"`
	ProjectID        int             `json:"project_id"`
	CreatedAt        string          `json:"created_at"`
}

// LBPlan represents a load balancer plan
type LBPlan struct {
	Name                 string      `json:"name"`
	PricePerHour         json.Number `json:"price_per_hour"`
	MaxListeners         int         `json:"max_listeners"`
	MaxTargets           int         `json:"max_targets"`
	ConnectionsPerSecond int         `json:"connections_per_second"`
}

// LBFloatingIP represents a floating IP assigned to a load balancer
type LBFloatingIP struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// LBListener represents a load balancer listener
type LBListener struct {
	UUID           string          `json:"uuid"`
	Name           string          `json:"name"`
	Protocol       string          `json:"protocol"`
	SourcePort     int             `json:"source_port"`
	TargetPort     int             `json:"target_port"`
	Algorithm      string          `json:"algorithm"`
	StickySessions bool            `json:"sticky_sessions"`
	Enabled        bool            `json:"enabled"`
	Targets        []LBTarget      `json:"targets"`
	TargetsCount   int             `json:"targets_count"`
	HealthCheck    json.RawMessage `json:"health_check,omitempty"`
}

// LBTarget represents a load balancer target
type LBTarget struct {
	UUID         string `json:"uuid"`
	TargetType   string `json:"target_type"`
	TargetUUID   string `json:"target_uuid"`
	TargetName   string `json:"target_name"`
	TargetIP     string `json:"target_ip"`
	Port         int    `json:"port"`
	Weight       int    `json:"weight"`
	Enabled      bool   `json:"enabled"`
	HealthStatus string `json:"health_status"`
}

// LBLocationPlans represents plans available at a location
type LBLocationPlans struct {
	LocationName string   `json:"location_name"`
	Description  string   `json:"location_description"`
	Plans        []LBPlan `json:"plans"`
}

// HealthCheckConfig represents a health check configuration
type HealthCheckConfig struct {
	Protocol           string `json:"protocol"`
	Path               string `json:"path"`
	IntervalSeconds    int    `json:"interval_seconds"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	HealthyThreshold   int    `json:"healthy_threshold"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
	ExpectedCodes      string `json:"expected_codes"`
}

// CreateLoadBalancerRequest represents a request to create a load balancer
type CreateLoadBalancerRequest struct {
	Name         string `json:"name"`
	PlanName     string `json:"plan_name"`
	LocationName string `json:"location_name"`
	ProjectID    *int   `json:"project_id,omitempty"`
	Label        string `json:"label,omitempty"`
}

// UpdateLoadBalancerRequest represents a request to update a load balancer
type UpdateLoadBalancerRequest struct {
	Name  *string `json:"name,omitempty"`
	Label *string `json:"label,omitempty"`
}

// CreateListenerRequest represents a request to create a listener
type CreateListenerRequest struct {
	Name           string `json:"name"`
	Protocol       string `json:"protocol"`
	SourcePort     int    `json:"source_port"`
	TargetPort     int    `json:"target_port"`
	Algorithm      string `json:"algorithm"`
	StickySessions bool   `json:"sticky_sessions"`
}

// UpdateListenerRequest represents a request to update a listener
type UpdateListenerRequest struct {
	Name       *string `json:"name,omitempty"`
	TargetPort *int    `json:"target_port,omitempty"`
	Algorithm  *string `json:"algorithm,omitempty"`
	Enabled    *bool   `json:"enabled,omitempty"`
}

// AddTargetRequest represents a request to add a target
type AddTargetRequest struct {
	TargetType string `json:"target_type"`
	TargetUUID string `json:"target_uuid"`
	Port       *int   `json:"port,omitempty"`
	Weight     int    `json:"weight"`
}

// UpdateTargetRequest represents a request to update a target
type UpdateTargetRequest struct {
	Port    *int  `json:"port,omitempty"`
	Weight  *int  `json:"weight,omitempty"`
	Enabled *bool `json:"enabled,omitempty"`
}
