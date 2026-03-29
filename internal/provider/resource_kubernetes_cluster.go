package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &kubernetesClusterResource{}
	_ resource.ResourceWithConfigure   = &kubernetesClusterResource{}
	_ resource.ResourceWithImportState = &kubernetesClusterResource{}
)

func NewKubernetesClusterResource() resource.Resource {
	return &kubernetesClusterResource{}
}

type kubernetesClusterResource struct {
	client *client.Client
}

type kubernetesClusterResourceModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Label            types.String `tfsdk:"label"`
	ProjectID        types.Int64  `tfsdk:"project_id"`
	Location         types.String `tfsdk:"location"`
	Plan             types.String `tfsdk:"plan"`
	Version          types.String `tfsdk:"version"`
	HAControlPlane   types.Bool   `tfsdk:"ha_control_plane"`
	InitialNodeCount types.Int64  `tfsdk:"initial_node_count"`
	NetworkID        types.Int64  `tfsdk:"network_id"`
	NodeCIDR         types.String `tfsdk:"node_cidr"`
	PodCIDR          types.String `tfsdk:"pod_cidr"`
	ServiceCIDR      types.String `tfsdk:"service_cidr"`
	Status           types.String `tfsdk:"status"`
	APIEndpoint      types.String `tfsdk:"api_endpoint"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (r *kubernetesClusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cluster"
}

func (r *kubernetesClusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Kubernetes cluster on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the Kubernetes cluster.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the cluster.",
				Required:    true,
			},
			"label": schema.StringAttribute{
				Description: "Optional label for the cluster.",
				Optional:    true,
				Computed:    true,
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID to create the cluster in.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Description: "The location name (e.g., us-mia-1).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plan": schema.StringAttribute{
				Description: "The server plan for the default node pool.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version": schema.StringAttribute{
				Description: "Kubernetes version. Defaults to the latest available version.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ha_control_plane": schema.BoolAttribute{
				Description: "Enable high-availability control plane.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"initial_node_count": schema.Int64Attribute{
				Description: "Number of initial worker nodes in the default pool.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
			},
			"network_id": schema.Int64Attribute{
				Description: "Existing network ID to use. If not set, a new network is created.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"node_cidr": schema.StringAttribute{
				Description: "Custom node CIDR.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"pod_cidr": schema.StringAttribute{
				Description: "Pod CIDR range.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("10.42.0.0/16"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_cidr": schema.StringAttribute{
				Description: "Service CIDR range.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("10.43.0.0/16"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Current status of the cluster.",
				Computed:    true,
			},
			"api_endpoint": schema.StringAttribute{
				Description: "The API endpoint URL for the cluster.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "When the cluster was created.",
				Computed:    true,
			},
		},
	}
}

func (r *kubernetesClusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *kubernetesClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	initialNodes := int(plan.InitialNodeCount.ValueInt64())
	if initialNodes < 1 {
		initialNodes = 1
	}

	createReq := &client.CreateKubernetesClusterRequest{
		ProjectID:      int(plan.ProjectID.ValueInt64()),
		Name:           plan.Name.ValueString(),
		LocationName:   plan.Location.ValueString(),
		HAControlPlane: plan.HAControlPlane.ValueBool(),
		NodePools: []client.CreateNodePoolInlineRequest{
			{
				Name:  "default",
				Plan:  plan.Plan.ValueString(),
				Count: initialNodes,
			},
		},
	}

	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		createReq.Version = plan.Version.ValueString()
	}

	// Build network config
	var network client.ClusterNetworkRequest
	hasNetwork := false

	if !plan.NetworkID.IsNull() && !plan.NetworkID.IsUnknown() {
		nid := int(plan.NetworkID.ValueInt64())
		network.NetworkID = &nid
		hasNetwork = true
	}
	if !plan.NodeCIDR.IsNull() && !plan.NodeCIDR.IsUnknown() {
		network.NodeCIDR = plan.NodeCIDR.ValueString()
		hasNetwork = true
	}
	if !plan.PodCIDR.IsNull() && !plan.PodCIDR.IsUnknown() && plan.PodCIDR.ValueString() != "10.42.0.0/16" {
		network.PodCIDR = plan.PodCIDR.ValueString()
		hasNetwork = true
	}
	if !plan.ServiceCIDR.IsNull() && !plan.ServiceCIDR.IsUnknown() && plan.ServiceCIDR.ValueString() != "10.43.0.0/16" {
		network.ServiceCIDR = plan.ServiceCIDR.ValueString()
		hasNetwork = true
	}
	if hasNetwork {
		createReq.Network = &network
	}

	cluster, err := r.client.Kubernetes.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Kubernetes cluster", err.Error())
		return
	}

	// Wait for cluster to be ready
	if cluster.Status != "running" && cluster.Status != "active" {
		cluster, err = r.client.Kubernetes.WaitForClusterReady(ctx, cluster.UUID, 20*time.Minute)
		if err != nil {
			resp.Diagnostics.AddError("Error waiting for cluster to be ready", err.Error())
			return
		}
	}

	r.mapClusterToState(&plan, cluster)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *kubernetesClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cluster, err := r.client.Kubernetes.Get(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Kubernetes cluster", err.Error())
		return
	}

	r.mapClusterToState(&state, cluster)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *kubernetesClusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state kubernetesClusterResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateKubernetesClusterRequest{}
	needsUpdate := false

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
		needsUpdate = true
	}
	if !plan.Label.Equal(state.Label) {
		v := plan.Label.ValueString()
		updateReq.Label = &v
		needsUpdate = true
	}

	if needsUpdate {
		_, err := r.client.Kubernetes.Update(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating Kubernetes cluster", err.Error())
			return
		}
	}

	// Re-read to get updated state
	cluster, err := r.client.Kubernetes.Get(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kubernetes cluster", err.Error())
		return
	}

	r.mapClusterToState(&plan, cluster)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *kubernetesClusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesClusterResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retry delete in case there are active tasks (HTTP 409)
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		err = r.client.Kubernetes.Delete(ctx, state.ID.ValueString())
		if err == nil {
			break
		}
		if apiErr, ok := err.(*client.APIError); ok {
			if apiErr.IsNotFound() {
				return
			}
			if apiErr.IsConflict() {
				time.Sleep(15 * time.Second)
				continue
			}
		}
		resp.Diagnostics.AddError("Error deleting Kubernetes cluster", err.Error())
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error deleting Kubernetes cluster", err.Error())
		return
	}

	// Wait for cluster to be destroyed
	err = r.client.Kubernetes.WaitForClusterDestroy(ctx, state.ID.ValueString(), 15*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for cluster deletion", err.Error())
		return
	}
}

func (r *kubernetesClusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *kubernetesClusterResource) mapClusterToState(state *kubernetesClusterResourceModel, cluster *client.KubernetesCluster) {
	state.ID = types.StringValue(cluster.UUID)
	state.Name = types.StringValue(cluster.Name)
	state.Status = types.StringValue(cluster.Status)
	state.Version = types.StringValue(cluster.Version)
	state.HAControlPlane = types.BoolValue(cluster.HAControlPlane)
	state.APIEndpoint = types.StringValue(cluster.APIEndpoint)
	state.PodCIDR = types.StringValue(cluster.PodCIDR)
	state.ServiceCIDR = types.StringValue(cluster.ServiceCIDR)
	state.Location = types.StringValue(cluster.Location.LocationName)
	state.CreatedAt = types.StringValue(cluster.CreatedAt)

	if cluster.Label != "" {
		state.Label = types.StringValue(cluster.Label)
	} else {
		state.Label = types.StringValue("")
	}

	if cluster.Network != nil {
		state.NodeCIDR = types.StringValue(cluster.Network.IPRange)
	} else if state.NodeCIDR.IsUnknown() {
		state.NodeCIDR = types.StringNull()
	}

	// Derive plan from first node pool if available
	if len(cluster.NodePools) > 0 {
		state.Plan = types.StringValue(cluster.NodePools[0].Plan.Name)
	}
}
