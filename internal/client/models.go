package client

import (
	"encoding/json"
	"time"
)

// Common models used across the API

// SSHKey represents an SSH key
type SSHKey struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	SSHKey      string `json:"ssh_key"`
	Fingerprint string `json:"fingerprint"`
	KeyType     string `json:"key_type"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// Project represents a project
type Project struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Network represents a private network
type Network struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Label        string    `json:"label"`
	ProjectID    int       `json:"project_id"`
	LocationName string    `json:"location_name"`
	IPRange      string    `json:"ip_range"`
	Prefix       int       `json:"prefix"`
	CreatedAt    time.Time `json:"created_at"`
}

// VPS represents a VPS instance
type VPS struct {
	ID           int             `json:"id"`
	Name         string          `json:"name"`
	Label        string          `json:"label"`
	ProjectID    int             `json:"project_id"`
	Status       string          `json:"status"`
	User         string          `json:"user"`
	Plan         VPSPlan         `json:"plan"`
	Template     VPSTemplate     `json:"template"`
	Location     Location        `json:"location"`
	FloatingIPs  json.RawMessage `json:"floating_ips"`
	IPv6         string          `json:"ipv6"`
	Network      *NetworkInfo    `json:"network,omitempty"`
	SSHKeys      []SSHKey        `json:"ssh_keys"`
	CreatedAt    time.Time       `json:"created_at"`
}

// VPSPlan represents a VPS plan
type VPSPlan struct {
	ID           int    `json:"id"`
	PlanName     string `json:"plan_name"`
	CPU          int    `json:"cpu"`
	RAM          int    `json:"ram"`
	Storage      int    `json:"storage"`
	Bandwidth    int    `json:"bandwidth"`
	PricePerHour string `json:"price_per_hour"` // API returns as string
}

// VPSTemplate represents a VPS template/operating system
type VPSTemplate struct {
	ID           int    `json:"id"`
	TemplateName string `json:"template_name"`
	OSName       string `json:"os_name"`
	Version      string `json:"version"`
}

// Location represents a datacenter location
type Location struct {
	ID           int    `json:"id"`
	LocationName string `json:"location_name"`
	Description  string `json:"description"`
}

// NetworkInfo represents network information for a VPS
type NetworkInfo struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	AssignedIP string `json:"assigned_ip"`
}

// FloatingIP represents a floating IP address
type FloatingIP struct {
	ID             int    `json:"id"`
	Address        string `json:"address"`
	Type           string `json:"type"` // IPv4 or IPv6
	Status         string `json:"status"`
	LocationName   string `json:"location_name"`
	ProtectionType string `json:"protection_type"`
	VPSName        string `json:"vps_name"`
	BaremetalName  string `json:"baremetal_name"`
}

// Baremetal represents a baremetal server
type Baremetal struct {
	ID               int             `json:"id"`
	Hostname         string          `json:"hostname"`
	Label            string          `json:"label"`
	ProjectID        int             `json:"project_id"`
	Status           string          `json:"status"`
	User             string          `json:"user"`
	OS               *OSInfo         `json:"os"`
	Location         Location        `json:"location"`
	BaremetalModel   BaremetalModel  `json:"baremetal_model"`
	FloatingIPs      []FloatingIP    `json:"floating_ips"`
	MonitoringEnable bool            `json:"monitoring_enable"`
	CreatedAt        time.Time       `json:"created_at"`
}

// BaremetalModel represents a baremetal server model
type BaremetalModel struct {
	ID          int    `json:"id"`
	ModelName   string `json:"model_name"`
	CPU         string `json:"cpu"`
	RAM         int    `json:"ram"`          // GB
	StorageType string `json:"storage_type"`
	DiskCount   int    `json:"disk_count"`
	DiskSize    string `json:"disk_size"` // String format like "2 x 4TB"
}

// OSInfo represents operating system information
type OSInfo struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ProjectResponse represents the response from GET /projects/
type ProjectResponse struct {
	Project    Project     `json:"project"`
	VPS        []VPS       `json:"vps"`
	Networks   []Network   `json:"networks"`
	Baremetals []Baremetal `json:"baremetals"`
}

// PricingResponse represents the response from GET /pricing
type PricingResponse struct {
	VPS VPSPricing `json:"vps"`
}

// VPSPricing represents VPS pricing information
type VPSPricing struct {
	Locations []LocationPricing `json:"locations"`
	Templates []VPSTemplate     `json:"templates"`
}

// LocationPricing represents pricing for a specific location
type LocationPricing struct {
	LocationName string    `json:"location_name"`
	Description  string    `json:"description"`
	Clusters     []Cluster `json:"clusters"`
}

// Cluster represents a cluster in a location
type Cluster struct {
	ClusterName string    `json:"cluster_name"`
	Plans       []VPSPlan `json:"plans"`
}

// FloatingIPsResponse represents the response from GET /floating_ips/organization
type FloatingIPsResponse struct {
	SingleIPs []FloatingIP `json:"single_ips"`
	Subnets   []Subnet     `json:"subnets"`
}

// Subnet represents a subnet of floating IPs
type Subnet struct {
	Prefix         int          `json:"prefix"`
	ProtectionType string       `json:"protection_type"`
	IPAddresses    []FloatingIP `json:"ip_addresses"`
}

// CreateVPSRequest represents a request to create a VPS
type CreateVPSRequest struct {
	Name                  string   `json:"name"`
	PlanName              string   `json:"plan_name"`
	TemplateName          string   `json:"template_name"`
	LocationName          string   `json:"location_name"`
	Label                 string   `json:"label"`
	NetworkID             *int     `json:"network_id,omitempty"`
	SSHKeyNames           []string `json:"ssh_key_names,omitempty"`
	User                  string   `json:"user,omitempty"`
	Password              string   `json:"password,omitempty"`
	IPv4                  *bool    `json:"ipv4,omitempty"`
	EnableBackups         *bool    `json:"enable_backups,omitempty"`
	CustomCloudInit       *string  `json:"custom_cloudinit,omitempty"`
	FirewallGroupIDs      []int    `json:"firewall_group_ids,omitempty"`
	AvailabilityGroupUUID *string  `json:"availability_group_uuid,omitempty"`
}

// CreateNetworkRequest represents a request to create a network
type CreateNetworkRequest struct {
	Name         string `json:"name"`
	LocationName string `json:"location_name"`
	IPRange      string `json:"ip_range"`
	Prefix       int    `json:"prefix"`
	ProjectID    int    `json:"project_id"`
	Label        string `json:"label,omitempty"`
}

// CreateProjectRequest represents a request to create a project
type CreateProjectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateSSHKeyRequest represents a request to create an SSH key
type CreateSSHKeyRequest struct {
	Name   string `json:"name"`
	SSHKey string `json:"ssh_key"`
}

// CreateBaremetalRequest represents a request to deploy a baremetal server
type CreateBaremetalRequest struct {
	ModelName      string   `json:"model_name"`
	LocationName   string   `json:"location_name"`
	Hostname       string   `json:"hostname"`
	Label          string   `json:"label,omitempty"`
	User           string   `json:"user,omitempty"`
	Password       string   `json:"password"`
	SSHKeyNames    []string `json:"ssh_key_names,omitempty"`
	OSName         string   `json:"os_name,omitempty"`
	DiskLayoutName string   `json:"disk_layout_name,omitempty"`
}

// UpdateBaremetalRequest represents a request to update a baremetal server
type UpdateBaremetalRequest struct {
	Hostname *string `json:"hostname,omitempty"`
	Label    *string `json:"label,omitempty"`
}

// ReinstallBaremetalRequest represents a request to reinstall a baremetal OS
type ReinstallBaremetalRequest struct {
	OSName         string   `json:"os_name"`
	DiskLayoutName string   `json:"disk_layout_name,omitempty"`
	User           string   `json:"user,omitempty"`
	Password       string   `json:"password"`
	Hostname       string   `json:"hostname,omitempty"`
	SSHKeyNames    []string `json:"ssh_key_names,omitempty"`
}

// TaskResponse represents a response with a task ID
type TaskResponse struct {
	TaskID  string `json:"task_id,omitempty"`
	Message string `json:"message,omitempty"`
}

// FirewallRule represents a single firewall rule
type FirewallRule struct {
	Direction string  `json:"direction"` // "in" or "out"
	Protocol  string  `json:"protocol"`  // "tcp", "udp", "icmp", "gre"
	Port      *string `json:"port,omitempty"`
	Source    *string `json:"source,omitempty"`
	Comment   *string `json:"comment,omitempty"`
}

// FirewallGroup represents a firewall group
type FirewallGroup struct {
	ID        int            `json:"id"`
	ProjectID int            `json:"project_id"`
	Name      string         `json:"name"`
	Rules     []FirewallRule `json:"rules"`
	Enabled   bool           `json:"enabled"`
	VPSCount  int            `json:"vps_count,omitempty"`
}

// CreateFirewallGroupRequest represents a request to create a firewall group
type CreateFirewallGroupRequest struct {
	Name    string         `json:"name"`
	Rules   []FirewallRule `json:"rules"`
	Enabled bool           `json:"enabled"`
}

// UpdateFirewallGroupRequest represents a request to update a firewall group
type UpdateFirewallGroupRequest struct {
	Name    *string         `json:"name,omitempty"`
	Rules   *[]FirewallRule `json:"rules,omitempty"`
	Enabled *bool           `json:"enabled,omitempty"`
}

// VPSFirewallGroupsRequest represents a request to update VPS firewall groups
type VPSFirewallGroupsRequest struct {
	FirewallGroupIDs []int `json:"firewall_group_ids"`
}

// VPSFirewallGroupsResponse represents the response from updating VPS firewall groups
type VPSFirewallGroupsResponse struct {
	Message         string `json:"message"`
	VPSID           int    `json:"vps_id"`
	FirewallGroups  []int  `json:"firewall_groups"`
	SyncTaskCreated bool   `json:"sync_task_created"`
}
