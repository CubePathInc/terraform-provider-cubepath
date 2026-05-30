package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &natGatewayResource{}
	_ resource.ResourceWithConfigure   = &natGatewayResource{}
	_ resource.ResourceWithImportState = &natGatewayResource{}
)

func NewNATGatewayResource() resource.Resource {
	return &natGatewayResource{}
}

type natGatewayResource struct {
	client *client.Client
}

type natGatewayResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Label        types.String `tfsdk:"label"`
	PlanName     types.String `tfsdk:"plan_name"`
	NetworkID    types.Int64  `tfsdk:"network_id"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	Status       types.String `tfsdk:"status"`
	LocationName types.String `tfsdk:"location_name"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *natGatewayResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nat_gateway"
}

func (r *natGatewayResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a NAT gateway on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the NAT gateway.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the NAT gateway.",
				Required:    true,
			},
			"label": schema.StringAttribute{
				Description: "Optional label for the NAT gateway.",
				Optional:    true,
				Computed:    true,
			},
			"plan_name": schema.StringAttribute{
				Description: "The plan name (e.g., nat.small). Changing this resizes the NAT gateway in-place.",
				Required:    true,
			},
			"network_id": schema.Int64Attribute{
				Description: "The private network ID to attach the NAT gateway to. Requires replacement if changed.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID. Uses default project if not specified. Requires replacement if changed.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Current status of the NAT gateway.",
				Computed:    true,
			},
			"location_name": schema.StringAttribute{
				Description: "The location where the NAT gateway is deployed.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Description: "When the NAT gateway was created.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *natGatewayResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = c
}

func (r *natGatewayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan natGatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateNATGatewayRequest{
		Name:      plan.Name.ValueString(),
		PlanName:  plan.PlanName.ValueString(),
		NetworkID: int(plan.NetworkID.ValueInt64()),
	}
	if !plan.Label.IsNull() && !plan.Label.IsUnknown() {
		v := plan.Label.ValueString()
		createReq.Label = &v
	}
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
	}

	gw, err := r.client.NATGateway.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating NAT gateway", err.Error())
		return
	}

	r.mapToState(&plan, gw)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *natGatewayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state natGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gw, err := r.client.NATGateway.Get(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading NAT gateway", err.Error())
		return
	}

	r.mapToState(&state, gw)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *natGatewayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state natGatewayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resize if plan_name changed
	if !plan.PlanName.Equal(state.PlanName) {
		if err := r.client.NATGateway.Resize(ctx, state.ID.ValueString(), plan.PlanName.ValueString()); err != nil {
			resp.Diagnostics.AddError("Error resizing NAT gateway", err.Error())
			return
		}
	}

	// Update name/label if changed
	if !plan.Name.Equal(state.Name) || !plan.Label.Equal(state.Label) {
		updateReq := &client.UpdateNATGatewayRequest{}
		if !plan.Name.Equal(state.Name) {
			v := plan.Name.ValueString()
			updateReq.Name = &v
		}
		if !plan.Label.Equal(state.Label) {
			v := plan.Label.ValueString()
			updateReq.Label = &v
		}
		_, err := r.client.NATGateway.Update(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating NAT gateway", err.Error())
			return
		}
	}

	// Re-read to get updated state
	gw, err := r.client.NATGateway.Get(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading NAT gateway after update", err.Error())
		return
	}

	r.mapToState(&plan, gw)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *natGatewayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state natGatewayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.NATGateway.Delete(ctx, state.ID.ValueString()); err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Error deleting NAT gateway", err.Error())
	}
}

func (r *natGatewayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *natGatewayResource) mapToState(state *natGatewayResourceModel, gw *client.NATGateway) {
	state.ID = types.StringValue(gw.UUID)
	state.Name = types.StringValue(gw.Name)
	state.Label = types.StringValue(gw.Label)
	state.Status = types.StringValue(gw.Status)
	state.PlanName = types.StringValue(gw.PlanName)
	state.NetworkID = types.Int64Value(int64(gw.NetworkID))
	state.ProjectID = types.Int64Value(int64(gw.ProjectID))
	state.LocationName = types.StringValue(gw.LocationName)
	state.CreatedAt = types.StringValue(gw.CreatedAt)
}
