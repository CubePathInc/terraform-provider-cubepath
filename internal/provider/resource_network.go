package provider

import (
	"context"
	"fmt"
	"strconv"

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
	_ resource.Resource                = &networkResource{}
	_ resource.ResourceWithConfigure   = &networkResource{}
	_ resource.ResourceWithImportState = &networkResource{}
)

func NewNetworkResource() resource.Resource {
	return &networkResource{}
}

type networkResource struct {
	client *client.Client
}

type networkResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Label    types.String `tfsdk:"label"`
	ProjectID types.Int64 `tfsdk:"project_id"`
	Location types.String `tfsdk:"location"`
	IPRange  types.String `tfsdk:"ip_range"`
	Prefix   types.Int64  `tfsdk:"prefix"`
	CIDR     types.String `tfsdk:"cidr"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func (r *networkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network"
}

func (r *networkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a private network on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the network.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the network.",
				Required:    true,
			},
			"label": schema.StringAttribute{
				Description: "A label for the network.",
				Optional:    true,
				Computed:    true,
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID to associate the network with.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Description: "The location where the network will be created.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ip_range": schema.StringAttribute{
				Description: "The base IP address for the network (e.g., '10.0.0.0').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"prefix": schema.Int64Attribute{
				Description: "The network prefix (8-30).",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"cidr": schema.StringAttribute{
				Description: "The full CIDR notation (computed from ip_range and prefix).",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the network was created.",
				Computed:    true,
			},
		},
	}
}

func (r *networkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *networkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate prefix range
	prefix := plan.Prefix.ValueInt64()
	if prefix < 8 || prefix > 30 {
		resp.Diagnostics.AddError(
			"Invalid Prefix",
			"Network prefix must be between 8 and 30",
		)
		return
	}

	// Create network request
	createReq := &client.CreateNetworkRequest{
		Name:         plan.Name.ValueString(),
		LocationName: plan.Location.ValueString(),
		IPRange:      plan.IPRange.ValueString(),
		Prefix:       int(prefix),
		ProjectID:    int(plan.ProjectID.ValueInt64()),
	}

	if !plan.Label.IsNull() {
		createReq.Label = plan.Label.ValueString()
	}

	network, err := r.client.Networks.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating network",
			"Could not create network: "+err.Error(),
		)
		return
	}

	// Update state
	plan.ID = types.StringValue(strconv.Itoa(network.ID))
	plan.CIDR = types.StringValue(fmt.Sprintf("%s/%d", network.IPRange, network.Prefix))
	plan.CreatedAt = types.StringValue(network.CreatedAt.String())
	if plan.Label.IsNull() {
		plan.Label = types.StringValue(network.Label)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *networkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing network ID",
			"Could not parse network ID: "+err.Error(),
		)
		return
	}

	network, err := r.client.Networks.Get(ctx, id)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error reading network",
			"Could not read network: "+err.Error(),
		)
		return
	}

	// Update state
	state.Name = types.StringValue(network.Name)
	state.Label = types.StringValue(network.Label)
	state.IPRange = types.StringValue(network.IPRange)
	state.Prefix = types.Int64Value(int64(network.Prefix))
	state.CIDR = types.StringValue(fmt.Sprintf("%s/%d", network.IPRange, network.Prefix))
	state.CreatedAt = types.StringValue(network.CreatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan networkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing network ID",
			"Could not parse network ID: "+err.Error(),
		)
		return
	}

	// Update network (name and label only)
	_, err = r.client.Networks.Update(ctx, id, plan.Name.ValueString(), plan.Label.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating network",
			"Could not update network: "+err.Error(),
		)
		return
	}

	// Read updated network
	network, err := r.client.Networks.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading network after update",
			"Could not read network: "+err.Error(),
		)
		return
	}

	plan.CreatedAt = types.StringValue(network.CreatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *networkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing network ID",
			"Could not parse network ID: "+err.Error(),
		)
		return
	}

	err = r.client.Networks.Delete(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting network",
			"Could not delete network: "+err.Error(),
		)
		return
	}
}

func (r *networkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
