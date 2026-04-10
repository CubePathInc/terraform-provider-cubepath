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
	_ resource.Resource                = &availabilityGroupResource{}
	_ resource.ResourceWithConfigure   = &availabilityGroupResource{}
	_ resource.ResourceWithImportState = &availabilityGroupResource{}
)

func NewAvailabilityGroupResource() resource.Resource {
	return &availabilityGroupResource{}
}

type availabilityGroupResource struct {
	client *client.Client
}

type availabilityGroupResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	LocationName types.String `tfsdk:"location_name"`
	Strategy     types.String `tfsdk:"strategy"`
	MaxServers   types.Int64  `tfsdk:"max_servers"`
	VPSCount     types.Int64  `tfsdk:"vps_count"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *availabilityGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_availability_group"
}

func (r *availabilityGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an availability group on CubePath Cloud. VPS instances in the same availability group are distributed across different physical hosts for high availability.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the availability group.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The ID of the project this availability group belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the availability group. Must be unique per project.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "A description of the availability group.",
				Optional:    true,
				Computed:    true,
			},
			"location_name": schema.StringAttribute{
				Description: "The location where the availability group operates (e.g., 'us-mia-1').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"strategy": schema.StringAttribute{
				Description: "The placement strategy. Currently always 'spread' (distribute VPS across different physical hosts).",
				Computed:    true,
			},
			"max_servers": schema.Int64Attribute{
				Description: "Maximum number of VPS that can be assigned to this group.",
				Computed:    true,
			},
			"vps_count": schema.Int64Attribute{
				Description: "Current number of VPS assigned to this group.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the availability group was created.",
				Computed:    true,
			},
		},
	}
}

func (r *availabilityGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *availabilityGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan availabilityGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if an availability group with this name already exists before creating
	existing, _ := r.client.AvailabilityGroups.FindByName(ctx, int(plan.ProjectID.ValueInt64()), plan.Name.ValueString())
	var group *client.AvailabilityGroup
	if existing != nil {
		group = existing
	} else {
		createReq := &client.CreateAvailabilityGroupRequest{
			ProjectID:    int(plan.ProjectID.ValueInt64()),
			Name:         plan.Name.ValueString(),
			Description:  plan.Description.ValueString(),
			LocationName: plan.LocationName.ValueString(),
		}
		created, err := r.client.AvailabilityGroups.Create(ctx, createReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error creating availability group",
				"Could not create availability group, unexpected error: "+err.Error(),
			)
			return
		}
		group = created
	}

	plan.ID = types.StringValue(group.UUID)
	plan.Strategy = types.StringValue(group.Strategy)
	plan.MaxServers = types.Int64Value(int64(group.MaxServers))
	plan.VPSCount = types.Int64Value(int64(group.VPSCount))
	plan.CreatedAt = types.StringValue(group.CreatedAt)
	// Keep plan description — the API create response may not echo it back
	if group.Description != "" {
		plan.Description = types.StringValue(group.Description)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *availabilityGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state availabilityGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.ID.IsNull() || state.ID.IsUnknown() || state.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	group, err := r.client.AvailabilityGroups.Get(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading availability group",
			"Could not read availability group "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	state.ProjectID = types.Int64Value(int64(group.ProjectID))
	state.Name = types.StringValue(group.Name)
	state.Description = types.StringValue(group.Description)
	state.LocationName = types.StringValue(group.LocationName)
	state.Strategy = types.StringValue(group.Strategy)
	state.MaxServers = types.Int64Value(int64(group.MaxServers))
	state.VPSCount = types.Int64Value(int64(group.VPSCount))
	state.CreatedAt = types.StringValue(group.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *availabilityGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// The API does not support updating availability groups.
	// Name changes require destroy and recreate.
	// project_id and location_name have RequiresReplace so they trigger recreation.
	resp.Diagnostics.AddError(
		"Update not supported",
		"Availability groups cannot be updated. Changes require resource replacement.",
	)
}

func (r *availabilityGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state availabilityGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.AvailabilityGroups.Delete(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting availability group",
			"Could not delete availability group, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *availabilityGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
