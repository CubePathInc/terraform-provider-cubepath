package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &kubernetesNodePoolResource{}
	_ resource.ResourceWithConfigure   = &kubernetesNodePoolResource{}
	_ resource.ResourceWithImportState = &kubernetesNodePoolResource{}
)

func NewKubernetesNodePoolResource() resource.Resource {
	return &kubernetesNodePoolResource{}
}

type kubernetesNodePoolResource struct {
	client *client.Client
}

type kubernetesNodePoolResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ClusterID types.String `tfsdk:"cluster_id"`
	Name      types.String `tfsdk:"name"`
	Plan      types.String `tfsdk:"plan"`
	NodeCount types.Int64  `tfsdk:"node_count"`
	AutoScale types.Bool   `tfsdk:"auto_scale"`
	MinNodes  types.Int64  `tfsdk:"min_nodes"`
	MaxNodes  types.Int64  `tfsdk:"max_nodes"`
	Labels    types.Map    `tfsdk:"labels"`
	Taints    types.List   `tfsdk:"taints"`
	Nodes     types.List   `tfsdk:"nodes"`
}

func (r *kubernetesNodePoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_node_pool"
}

func (r *kubernetesNodePoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Kubernetes node pool on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the node pool.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cluster_id": schema.StringAttribute{
				Description: "UUID of the Kubernetes cluster.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Name of the node pool.",
				Required:    true,
			},
			"plan": schema.StringAttribute{
				Description: "Server plan for the nodes.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"node_count": schema.Int64Attribute{
				Description: "Desired number of worker nodes.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
			},
			"auto_scale": schema.BoolAttribute{
				Description: "Enable auto scaling.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"min_nodes": schema.Int64Attribute{
				Description: "Minimum number of nodes (set by the API).",
				Computed:    true,
			},
			"max_nodes": schema.Int64Attribute{
				Description: "Maximum number of nodes (set by the API).",
				Computed:    true,
			},
			"labels": schema.MapAttribute{
				Description: "Kubernetes labels applied to nodes in this pool.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"taints": schema.ListNestedAttribute{
				Description: "Kubernetes taints applied to nodes in this pool.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Description: "Taint key.",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "Taint value.",
							Required:    true,
						},
						"effect": schema.StringAttribute{
							Description: "Taint effect (NoSchedule, NoExecute, PreferNoSchedule).",
							Required:    true,
						},
					},
				},
			},
			"nodes": schema.ListNestedAttribute{
				Description: "Current nodes in the pool.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"vps_name": schema.StringAttribute{
							Description: "VPS name of the node.",
							Computed:    true,
						},
						"vps_status": schema.StringAttribute{
							Description: "VPS status of the node.",
							Computed:    true,
						},
						"k8s_status": schema.StringAttribute{
							Description: "Kubernetes status of the node.",
							Computed:    true,
						},
						"floating_ip": schema.StringAttribute{
							Description: "Floating IP address.",
							Computed:    true,
						},
						"private_ip": schema.StringAttribute{
							Description: "Private IP address.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (r *kubernetesNodePoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *kubernetesNodePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateNodePoolRequest{
		Name:      plan.Name.ValueString(),
		Plan:      plan.Plan.ValueString(),
		Count:     int(plan.NodeCount.ValueInt64()),
		AutoScale: plan.AutoScale.ValueBool(),
	}

	// Parse labels
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		labels := make(map[string]string)
		resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Labels = labels
	}

	// Parse taints
	if !plan.Taints.IsNull() && !plan.Taints.IsUnknown() {
		taints, diags := r.parseTaintsFromState(ctx, plan.Taints)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Taints = taints
	}

	// Retry create in case cluster has active tasks (HTTP 409)
	var created *client.KubernetesNodePool
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		created, err = r.client.Kubernetes.CreateNodePool(ctx, plan.ClusterID.ValueString(), createReq)
		if err == nil {
			break
		}
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsConflict() {
			time.Sleep(15 * time.Second)
			continue
		}
		break
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating node pool", err.Error())
		return
	}

	// The create response may be partial, so re-read the full object
	pool, err := r.client.Kubernetes.GetNodePool(ctx, plan.ClusterID.ValueString(), created.UUID)
	if err != nil {
		// If re-read fails, use the create response
		pool = created
	}

	r.mapPoolToState(ctx, &plan, pool)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *kubernetesNodePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pool, err := r.client.Kubernetes.GetNodePool(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading node pool", err.Error())
		return
	}

	r.mapPoolToState(ctx, &state, pool)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *kubernetesNodePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateNodePoolRequest{}
	needsUpdate := false

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
		needsUpdate = true
	}
	if !plan.NodeCount.Equal(state.NodeCount) {
		v := int(plan.NodeCount.ValueInt64())
		updateReq.DesiredNodes = &v
		needsUpdate = true
	}
	if !plan.AutoScale.Equal(state.AutoScale) {
		v := plan.AutoScale.ValueBool()
		updateReq.AutoScale = &v
		needsUpdate = true
	}
	if !plan.Labels.Equal(state.Labels) {
		labels := make(map[string]string)
		resp.Diagnostics.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Labels = &labels
		needsUpdate = true
	}
	if !plan.Taints.Equal(state.Taints) {
		taints, diags := r.parseTaintsFromState(ctx, plan.Taints)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Taints = &taints
		needsUpdate = true
	}

	if needsUpdate {
		_, err := r.client.Kubernetes.UpdateNodePool(ctx, state.ClusterID.ValueString(), state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating node pool", err.Error())
			return
		}
	}

	// Re-read to get updated state
	pool, err := r.client.Kubernetes.GetNodePool(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading node pool", err.Error())
		return
	}

	r.mapPoolToState(ctx, &plan, pool)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *kubernetesNodePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retry delete in case there are active tasks (HTTP 409)
	var err error
	for attempt := 0; attempt < 30; attempt++ {
		err = r.client.Kubernetes.DeleteNodePool(ctx, state.ClusterID.ValueString(), state.ID.ValueString())
		if err == nil {
			return
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
		break
	}
	if err != nil {
		resp.Diagnostics.AddError("Error deleting node pool", err.Error())
		return
	}
}

func (r *kubernetesNodePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID",
			"Expected format: cluster_uuid/pool_uuid")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *kubernetesNodePoolResource) parseTaintsFromState(ctx context.Context, taintsList types.List) ([]client.KubernetesTaint, diag.Diagnostics) {
	var diags diag.Diagnostics
	if taintsList.IsNull() || taintsList.IsUnknown() {
		return nil, diags
	}

	var taintObjects []struct {
		Key    string `tfsdk:"key"`
		Value  string `tfsdk:"value"`
		Effect string `tfsdk:"effect"`
	}
	diags.Append(taintsList.ElementsAs(ctx, &taintObjects, false)...)
	if diags.HasError() {
		return nil, diags
	}

	taints := make([]client.KubernetesTaint, 0, len(taintObjects))
	for _, t := range taintObjects {
		taints = append(taints, client.KubernetesTaint{
			Key:    t.Key,
			Value:  t.Value,
			Effect: t.Effect,
		})
	}
	return taints, diags
}

func (r *kubernetesNodePoolResource) mapPoolToState(ctx context.Context, state *kubernetesNodePoolResourceModel, pool *client.KubernetesNodePool) {
	state.ID = types.StringValue(pool.UUID)
	state.Name = types.StringValue(pool.Name)
	state.Plan = types.StringValue(pool.Plan.Name)
	state.NodeCount = types.Int64Value(int64(pool.DesiredNodes))
	state.AutoScale = types.BoolValue(pool.AutoScale)
	state.MinNodes = types.Int64Value(int64(pool.MinNodes))
	state.MaxNodes = types.Int64Value(int64(pool.MaxNodes))

	// Map labels
	if len(pool.Labels) > 0 {
		labelElements := make(map[string]attr.Value)
		for k, v := range pool.Labels {
			labelElements[k] = types.StringValue(v)
		}
		state.Labels, _ = types.MapValue(types.StringType, labelElements)
	} else {
		state.Labels = types.MapNull(types.StringType)
	}

	// Map taints
	taintAttrTypes := map[string]attr.Type{
		"key":    types.StringType,
		"value":  types.StringType,
		"effect": types.StringType,
	}
	taintObjType := types.ObjectType{AttrTypes: taintAttrTypes}

	if len(pool.Taints) > 0 {
		taintValues := make([]attr.Value, 0, len(pool.Taints))
		for _, t := range pool.Taints {
			obj, _ := types.ObjectValue(taintAttrTypes, map[string]attr.Value{
				"key":    types.StringValue(t.Key),
				"value":  types.StringValue(t.Value),
				"effect": types.StringValue(t.Effect),
			})
			taintValues = append(taintValues, obj)
		}
		state.Taints, _ = types.ListValue(taintObjType, taintValues)
	} else {
		state.Taints = types.ListNull(taintObjType)
	}

	// Map nodes
	nodeAttrTypes := map[string]attr.Type{
		"vps_name":    types.StringType,
		"vps_status":  types.StringType,
		"k8s_status":  types.StringType,
		"floating_ip": types.StringType,
		"private_ip":  types.StringType,
	}
	nodeObjType := types.ObjectType{AttrTypes: nodeAttrTypes}

	if len(pool.Nodes) > 0 {
		nodeValues := make([]attr.Value, 0, len(pool.Nodes))
		for _, n := range pool.Nodes {
			obj, _ := types.ObjectValue(nodeAttrTypes, map[string]attr.Value{
				"vps_name":    types.StringValue(n.VPSName),
				"vps_status":  types.StringValue(n.VPSStatus),
				"k8s_status":  types.StringValue(n.K8sStatus),
				"floating_ip": types.StringValue(n.FloatingIP),
				"private_ip":  types.StringValue(n.PrivateIP),
			})
			nodeValues = append(nodeValues, obj)
		}
		state.Nodes, _ = types.ListValue(nodeObjType, nodeValues)
	} else {
		state.Nodes, _ = types.ListValue(nodeObjType, []attr.Value{})
	}
}
