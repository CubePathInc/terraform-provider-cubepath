package client

import "encoding/json"

// KubernetesCluster represents a Kubernetes cluster
type KubernetesCluster struct {
	UUID           string              `json:"uuid"`
	Name           string              `json:"name"`
	Label          string              `json:"label"`
	Status         string              `json:"status"`
	Version        string              `json:"version"`
	HAControlPlane bool                `json:"ha_control_plane"`
	APIEndpoint    string              `json:"api_endpoint"`
	PodCIDR        string              `json:"pod_cidr"`
	ServiceCIDR    string              `json:"service_cidr"`
	BillingType    string              `json:"billing_type"`
	Location       KubernetesLocation  `json:"location"`
	Network        *KubernetesNetwork  `json:"network,omitempty"`
	NodePools      []KubernetesNodePool `json:"node_pools"`
	CreatedAt      string              `json:"created_at"`
}

// KubernetesLocation represents a cluster location
type KubernetesLocation struct {
	LocationName string `json:"location_name"`
	Description  string `json:"description"`
}

// KubernetesNetwork represents cluster network info
type KubernetesNetwork struct {
	Name    string `json:"name"`
	IPRange string `json:"ip_range"`
	Prefix  int    `json:"prefix"`
}

// KubernetesNodePool represents a node pool in a cluster
type KubernetesNodePool struct {
	UUID         string            `json:"uuid"`
	Name         string            `json:"name"`
	DesiredNodes int               `json:"desired_nodes"`
	MinNodes     int               `json:"min_nodes"`
	MaxNodes     int               `json:"max_nodes"`
	AutoScale    bool              `json:"auto_scale"`
	Plan         KubernetesNodePlan `json:"plan"`
	Labels       map[string]string `json:"labels,omitempty"`
	Taints       []KubernetesTaint `json:"taints,omitempty"`
	Nodes        []KubernetesNode  `json:"nodes"`
}

// KubernetesNodePlan represents the plan of a node pool
type KubernetesNodePlan struct {
	Name string `json:"name"`
}

// KubernetesTaint represents a Kubernetes taint
type KubernetesTaint struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// KubernetesNode represents a node in a pool
type KubernetesNode struct {
	VPSName    string `json:"vps_name"`
	VPSStatus  string `json:"vps_status"`
	K8sStatus  string `json:"k8s_status"`
	FloatingIP string `json:"floating_ip"`
	PrivateIP  string `json:"private_ip"`
}

// KubernetesVersion represents an available K8s version
type KubernetesVersion struct {
	Version   string `json:"version"`
	IsDefault bool   `json:"is_default"`
	MinCPU    int    `json:"min_cpu"`
	MinRAMMB  int    `json:"min_ram_mb"`
}

// KubernetesPlan represents a server plan compatible with K8s
type KubernetesPlan struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	CPU          int     `json:"cpu"`
	RAM          int     `json:"ram"`
	Storage      int     `json:"storage"`
	PricePerHour float64 `json:"price_per_hour"`
}

// KubernetesAddon represents an available addon
type KubernetesAddon struct {
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	HelmRepoName     string `json:"helm_repo_name"`
	HelmRepoURL      string `json:"helm_repo_url"`
	HelmChart        string `json:"helm_chart"`
	DefaultVersion   string `json:"default_version"`
	Namespace        string `json:"namespace"`
	IconURL          string `json:"icon_url"`
	DocumentationURL string `json:"documentation_url"`
	Keywords         string `json:"keywords"`
	MinK8sVersion    string `json:"min_k8s_version"`
}

// KubernetesInstalledAddon represents an addon installed on a cluster
type KubernetesInstalledAddon struct {
	UUID             string           `json:"uuid"`
	Status           string           `json:"status"`
	InstalledVersion string           `json:"installed_version"`
	Addon            KubernetesAddon  `json:"addon"`
	InstalledAt      string           `json:"installed_at"`
}

// CreateKubernetesClusterRequest represents a request to create a cluster
type CreateKubernetesClusterRequest struct {
	ProjectID      int                          `json:"project_id"`
	Name           string                       `json:"name"`
	LocationName   string                       `json:"location_name"`
	HAControlPlane bool                         `json:"ha_control_plane"`
	Version        string                       `json:"version,omitempty"`
	NodePools      []CreateNodePoolInlineRequest `json:"node_pools"`
	Network        *ClusterNetworkRequest       `json:"network,omitempty"`
}

// CreateNodePoolInlineRequest is the inline node pool in cluster creation
type CreateNodePoolInlineRequest struct {
	Name  string `json:"name"`
	Plan  string `json:"plan"`
	Count int    `json:"count"`
}

// ClusterNetworkRequest represents network configuration for cluster creation
type ClusterNetworkRequest struct {
	NetworkID   *int   `json:"network_id,omitempty"`
	NodeCIDR    string `json:"node_cidr,omitempty"`
	PodCIDR     string `json:"pod_cidr,omitempty"`
	ServiceCIDR string `json:"service_cidr,omitempty"`
}

// UpdateKubernetesClusterRequest represents a request to update a cluster
type UpdateKubernetesClusterRequest struct {
	Name  *string `json:"name,omitempty"`
	Label *string `json:"label,omitempty"`
}

// CreateNodePoolRequest represents a request to create a node pool
type CreateNodePoolRequest struct {
	Name      string            `json:"name"`
	Plan      string            `json:"plan"`
	Count     int               `json:"count"`
	AutoScale bool              `json:"auto_scale"`
	Labels    map[string]string `json:"labels,omitempty"`
	Taints    []KubernetesTaint `json:"taints,omitempty"`
}

// UpdateNodePoolRequest represents a request to update a node pool
type UpdateNodePoolRequest struct {
	Name         *string            `json:"name,omitempty"`
	DesiredNodes *int               `json:"desired_nodes,omitempty"`
	MinNodes     *int               `json:"min_nodes,omitempty"`
	MaxNodes     *int               `json:"max_nodes,omitempty"`
	AutoScale    *bool              `json:"auto_scale,omitempty"`
	Labels       *map[string]string `json:"labels,omitempty"`
	Taints       *[]KubernetesTaint `json:"taints,omitempty"`
}

// InstallAddonRequest represents a request to install an addon
type InstallAddonRequest struct {
	CustomValues json.RawMessage `json:"custom_values,omitempty"`
}

// KubeconfigResponse represents the kubeconfig response
type KubeconfigResponse struct {
	Kubeconfig string `json:"kubeconfig"`
}
