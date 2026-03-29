package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
)

// KubernetesService provides methods for managing Kubernetes clusters
type KubernetesService struct {
	client *Client
}

// NewKubernetesService creates a new Kubernetes service
func NewKubernetesService(client *Client) *KubernetesService {
	return &KubernetesService{client: client}
}

// --- Versions & Plans ---

// ListVersions lists available Kubernetes versions
func (k *KubernetesService) ListVersions(ctx context.Context) ([]KubernetesVersion, error) {
	var result []KubernetesVersion
	err := k.client.Get(ctx, "/kubernetes/versions", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list kubernetes versions: %w", err)
	}
	return result, nil
}

// ListPlans lists server plans compatible with Kubernetes
func (k *KubernetesService) ListPlans(ctx context.Context, version string) ([]KubernetesPlan, error) {
	path := "/kubernetes/plans"
	if version != "" {
		path += "?version=" + version
	}
	var result []KubernetesPlan
	err := k.client.Get(ctx, path, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list kubernetes plans: %w", err)
	}
	return result, nil
}

// --- Cluster CRUD ---

// List lists all Kubernetes clusters
func (k *KubernetesService) List(ctx context.Context) ([]KubernetesCluster, error) {
	var result []KubernetesCluster
	err := k.client.Get(ctx, "/kubernetes/", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list kubernetes clusters: %w", err)
	}
	return result, nil
}

// Get retrieves a Kubernetes cluster by UUID
func (k *KubernetesService) Get(ctx context.Context, uuid string) (*KubernetesCluster, error) {
	var result KubernetesCluster
	err := k.client.Get(ctx, fmt.Sprintf("/kubernetes/%s", uuid), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubernetes cluster: %w", err)
	}
	return &result, nil
}

// Create creates a new Kubernetes cluster
func (k *KubernetesService) Create(ctx context.Context, req *CreateKubernetesClusterRequest) (*KubernetesCluster, error) {
	var result KubernetesCluster
	err := k.client.Post(ctx, "/kubernetes/", req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes cluster: %w", err)
	}
	return &result, nil
}

// Update updates a Kubernetes cluster
func (k *KubernetesService) Update(ctx context.Context, uuid string, req *UpdateKubernetesClusterRequest) (*KubernetesCluster, error) {
	var result KubernetesCluster
	err := k.client.Patch(ctx, fmt.Sprintf("/kubernetes/%s", uuid), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update kubernetes cluster: %w", err)
	}
	return &result, nil
}

// Delete deletes a Kubernetes cluster
func (k *KubernetesService) Delete(ctx context.Context, uuid string) error {
	err := k.client.Delete(ctx, fmt.Sprintf("/kubernetes/%s", uuid))
	if err != nil {
		return fmt.Errorf("failed to delete kubernetes cluster: %w", err)
	}
	return nil
}

// GetKubeconfig retrieves the kubeconfig for a cluster.
// The API returns raw YAML (not JSON), so we read the response body directly.
func (k *KubernetesService) GetKubeconfig(ctx context.Context, uuid string) (string, error) {
	resp, err := k.client.doRequest(ctx, "GET", fmt.Sprintf("/kubernetes/%s/kubeconfig", uuid), nil)
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", parseAPIError(resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read kubeconfig response: %w", err)
	}

	// Try JSON first (some API versions wrap it)
	var jsonResp KubeconfigResponse
	if err := json.Unmarshal(body, &jsonResp); err == nil && jsonResp.Kubeconfig != "" {
		return jsonResp.Kubeconfig, nil
	}

	// Otherwise it's raw YAML
	return string(body), nil
}

// --- Node Pool CRUD ---

// ListNodePools lists all node pools in a cluster
func (k *KubernetesService) ListNodePools(ctx context.Context, clusterUUID string) ([]KubernetesNodePool, error) {
	var result []KubernetesNodePool
	err := k.client.Get(ctx, fmt.Sprintf("/kubernetes/%s/node-pools/", clusterUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list node pools: %w", err)
	}
	return result, nil
}

// GetNodePool retrieves a specific node pool by UUID
func (k *KubernetesService) GetNodePool(ctx context.Context, clusterUUID, poolUUID string) (*KubernetesNodePool, error) {
	pools, err := k.ListNodePools(ctx, clusterUUID)
	if err != nil {
		return nil, err
	}
	for _, pool := range pools {
		if pool.UUID == poolUUID {
			return &pool, nil
		}
	}
	return nil, &APIError{
		StatusCode: 404,
		Message:    "Not Found",
		Detail:     fmt.Sprintf("node pool %s not found in cluster %s", poolUUID, clusterUUID),
	}
}

// CreateNodePoolResponse represents the response from creating a node pool
type CreateNodePoolResponse struct {
	Detail string `json:"detail"`
	UUID   string `json:"uuid"`
}

// CreateNodePool creates a new node pool in a cluster
func (k *KubernetesService) CreateNodePool(ctx context.Context, clusterUUID string, req *CreateNodePoolRequest) (*KubernetesNodePool, error) {
	// Try to unmarshal as full object first, fallback to simple response
	var result KubernetesNodePool
	err := k.client.Post(ctx, fmt.Sprintf("/kubernetes/%s/node-pools/", clusterUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to create node pool: %w", err)
	}

	// If the response was a simple {uuid, detail} response, the UUID field will be set
	// but other fields may be empty. Try to re-read to get full object.
	if result.UUID != "" && result.Plan.Name == "" {
		pool, getErr := k.GetNodePool(ctx, clusterUUID, result.UUID)
		if getErr == nil {
			return pool, nil
		}
	}

	return &result, nil
}

// UpdateNodePool updates a node pool
func (k *KubernetesService) UpdateNodePool(ctx context.Context, clusterUUID, poolUUID string, req *UpdateNodePoolRequest) (*KubernetesNodePool, error) {
	var result KubernetesNodePool
	err := k.client.Patch(ctx, fmt.Sprintf("/kubernetes/%s/node-pools/%s", clusterUUID, poolUUID), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to update node pool: %w", err)
	}
	return &result, nil
}

// DeleteNodePool deletes a node pool
func (k *KubernetesService) DeleteNodePool(ctx context.Context, clusterUUID, poolUUID string) error {
	err := k.client.Delete(ctx, fmt.Sprintf("/kubernetes/%s/node-pools/%s", clusterUUID, poolUUID))
	if err != nil {
		return fmt.Errorf("failed to delete node pool: %w", err)
	}
	return nil
}

// --- Addon CRUD ---

// ListAddons lists all available addons
func (k *KubernetesService) ListAddons(ctx context.Context) ([]KubernetesAddon, error) {
	var result []KubernetesAddon
	err := k.client.Get(ctx, "/kubernetes/addons", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list addons: %w", err)
	}
	return result, nil
}

// GetAddon retrieves an addon by slug
func (k *KubernetesService) GetAddon(ctx context.Context, slug string) (*KubernetesAddon, error) {
	var result KubernetesAddon
	err := k.client.Get(ctx, fmt.Sprintf("/kubernetes/addons/%s", slug), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get addon: %w", err)
	}
	return &result, nil
}

// ListInstalledAddons lists addons installed on a cluster
func (k *KubernetesService) ListInstalledAddons(ctx context.Context, clusterUUID string) ([]KubernetesInstalledAddon, error) {
	var result []KubernetesInstalledAddon
	err := k.client.Get(ctx, fmt.Sprintf("/kubernetes/%s/addons", clusterUUID), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to list installed addons: %w", err)
	}
	return result, nil
}

// InstallAddonResponse represents the response from installing an addon
type InstallAddonResponse struct {
	Detail string `json:"detail"`
	UUID   string `json:"uuid"`
}

// InstallAddon installs an addon on a cluster
func (k *KubernetesService) InstallAddon(ctx context.Context, clusterUUID, slug string, values json.RawMessage) (*KubernetesInstalledAddon, error) {
	var req interface{}
	if len(values) > 0 {
		req = &InstallAddonRequest{CustomValues: values}
	} else {
		req = map[string]interface{}{}
	}
	var result KubernetesInstalledAddon
	err := k.client.Post(ctx, fmt.Sprintf("/kubernetes/%s/addons/%s/install", clusterUUID, slug), req, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to install addon: %w", err)
	}

	// If the response was partial, re-read from installed list
	if result.UUID != "" && result.Addon.Slug == "" {
		addons, listErr := k.ListInstalledAddons(ctx, clusterUUID)
		if listErr == nil {
			for _, a := range addons {
				if a.UUID == result.UUID {
					return &a, nil
				}
			}
		}
	}

	return &result, nil
}

// UninstallAddon uninstalls an addon from a cluster
func (k *KubernetesService) UninstallAddon(ctx context.Context, clusterUUID, addonUUID string) error {
	err := k.client.Delete(ctx, fmt.Sprintf("/kubernetes/%s/addons/%s", clusterUUID, addonUUID))
	if err != nil {
		return fmt.Errorf("failed to uninstall addon: %w", err)
	}
	return nil
}

// --- Waiters ---

// WaitForClusterReady waits for a cluster to reach a ready state
func (k *KubernetesService) WaitForClusterReady(ctx context.Context, uuid string, timeout time.Duration) (*KubernetesCluster, error) {
	stateConf := &utils.StateChangeConf{
		Pending: []string{"provisioning", "creating", "deploying", "pending", "updating"},
		Target:  []string{"running", "active"},
		Refresh: func() (interface{}, string, error) {
			cluster, err := k.Get(ctx, uuid)
			if err != nil {
				return nil, "", err
			}
			return cluster, cluster.Status, nil
		},
		Timeout:      timeout,
		PollInterval: 15 * time.Second,
		MinTimeout:   10 * time.Second,
	}

	result, err := stateConf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}

	return result.(*KubernetesCluster), nil
}

// WaitForClusterDestroy waits for a cluster to be fully destroyed
func (k *KubernetesService) WaitForClusterDestroy(ctx context.Context, uuid string, timeout time.Duration) error {
	stateConf := &utils.StateChangeConf{
		Pending: []string{"running", "active", "deleting", "destroying"},
		Target:  []string{"deleted"},
		Refresh: func() (interface{}, string, error) {
			_, err := k.Get(ctx, uuid)
			if err != nil {
				if apiErr, ok := err.(*APIError); ok && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				// Unwrap to check inner error
				if wrapped, ok := err.(interface{ Unwrap() error }); ok {
					if apiErr, ok := wrapped.Unwrap().(*APIError); ok && apiErr.IsNotFound() {
						return nil, "deleted", nil
					}
				}
				return nil, "", err
			}
			return nil, "deleting", nil
		},
		Timeout:      timeout,
		PollInterval: 15 * time.Second,
		MinTimeout:   10 * time.Second,
	}

	_, err := stateConf.WaitForState(ctx)
	return err
}
