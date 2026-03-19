package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &lbListenerResource{}
	_ resource.ResourceWithConfigure   = &lbListenerResource{}
	_ resource.ResourceWithImportState = &lbListenerResource{}
)

func NewLBListenerResource() resource.Resource {
	return &lbListenerResource{}
}

type lbListenerResource struct {
	client *client.Client
}

type lbListenerResourceModel struct {
	ID             types.String `tfsdk:"id"`
	LoadBalancerID types.String `tfsdk:"loadbalancer_id"`
	Name           types.String `tfsdk:"name"`
	Protocol       types.String `tfsdk:"protocol"`
	SourcePort     types.Int64  `tfsdk:"source_port"`
	TargetPort     types.Int64  `tfsdk:"target_port"`
	Algorithm      types.String `tfsdk:"algorithm"`
	StickySessions types.Bool   `tfsdk:"sticky_sessions"`
	Enabled        types.Bool   `tfsdk:"enabled"`
}

func (r *lbListenerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_listener"
}

func (r *lbListenerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a load balancer listener on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the listener.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"loadbalancer_id": schema.StringAttribute{
				Description: "The UUID of the load balancer.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the listener.",
				Required:    true,
			},
			"protocol": schema.StringAttribute{
				Description: "Protocol: http, https, tcp, or tls.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_port": schema.Int64Attribute{
				Description: "The port to listen on.",
				Required:    true,
			},
			"target_port": schema.Int64Attribute{
				Description: "The port to forward traffic to.",
				Required:    true,
			},
			"algorithm": schema.StringAttribute{
				Description: "Load balancing algorithm: round_robin, least_conn, or source.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("round_robin"),
			},
			"sticky_sessions": schema.BoolAttribute{
				Description: "Enable sticky sessions.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the listener is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
		},
	}
}

func (r *lbListenerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *lbListenerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lbListenerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateListenerRequest{
		Name:           plan.Name.ValueString(),
		Protocol:       plan.Protocol.ValueString(),
		SourcePort:     int(plan.SourcePort.ValueInt64()),
		TargetPort:     int(plan.TargetPort.ValueInt64()),
		Algorithm:      plan.Algorithm.ValueString(),
		StickySessions: plan.StickySessions.ValueBool(),
	}

	listener, err := r.client.LoadBalancer.CreateListener(ctx, plan.LoadBalancerID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating listener", err.Error())
		return
	}

	plan.ID = types.StringValue(listener.UUID)
	plan.Enabled = types.BoolValue(listener.Enabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *lbListenerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lbListenerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lb, err := r.client.LoadBalancer.Get(ctx, state.LoadBalancerID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading load balancer", err.Error())
		return
	}

	var found *client.LBListener
	for _, l := range lb.Listeners {
		if l.UUID == state.ID.ValueString() {
			found = &l
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.Protocol = types.StringValue(found.Protocol)
	state.SourcePort = types.Int64Value(int64(found.SourcePort))
	state.TargetPort = types.Int64Value(int64(found.TargetPort))
	state.Algorithm = types.StringValue(found.Algorithm)
	state.StickySessions = types.BoolValue(found.StickySessions)
	state.Enabled = types.BoolValue(found.Enabled)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbListenerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lbListenerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateListenerRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.TargetPort.Equal(state.TargetPort) {
		v := int(plan.TargetPort.ValueInt64())
		updateReq.TargetPort = &v
	}
	if !plan.Algorithm.Equal(state.Algorithm) {
		v := plan.Algorithm.ValueString()
		updateReq.Algorithm = &v
	}
	if !plan.Enabled.Equal(state.Enabled) {
		v := plan.Enabled.ValueBool()
		updateReq.Enabled = &v
	}

	_, err := r.client.LoadBalancer.UpdateListener(ctx, state.LoadBalancerID.ValueString(), state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating listener", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *lbListenerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lbListenerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.LoadBalancer.DeleteListener(ctx, state.LoadBalancerID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting listener", err.Error())
		return
	}
}

func (r *lbListenerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: lb_uuid/listener_uuid
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: lb_uuid/listener_uuid")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("loadbalancer_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
