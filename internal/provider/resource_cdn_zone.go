package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &cdnZoneResource{}
	_ resource.ResourceWithConfigure   = &cdnZoneResource{}
	_ resource.ResourceWithImportState = &cdnZoneResource{}
)

func NewCDNZoneResource() resource.Resource {
	return &cdnZoneResource{}
}

type cdnZoneResource struct {
	client *client.Client
}

type cdnZoneResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Domain       types.String `tfsdk:"domain"`
	CustomDomain types.String `tfsdk:"custom_domain"`
	PlanName     types.String `tfsdk:"plan_name"`
	SSLType      types.String `tfsdk:"ssl_type"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *cdnZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cdn_zone"
}

func (r *cdnZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CDN zone on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the CDN zone.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Zone name (3-32 chars, lowercase alphanumeric + hyphens).",
				Required:    true,
			},
			"domain": schema.StringAttribute{
				Description: "The CDN domain assigned to the zone.",
				Computed:    true,
			},
			"custom_domain": schema.StringAttribute{
				Description: "Custom domain for the CDN zone.",
				Optional:    true,
				Computed:    true,
			},
			"plan_name": schema.StringAttribute{
				Description: "The CDN plan name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ssl_type": schema.StringAttribute{
				Description: "SSL type: automatic or custom.",
				Optional:    true,
				Computed:    true,
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID.",
				Optional:    true,
				Computed:    true,
			},
			"status": schema.StringAttribute{
				Description: "Current status of the CDN zone.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "When the zone was created.",
				Computed:    true,
			},
			"updated_at": schema.StringAttribute{
				Description: "When the zone was last updated.",
				Computed:    true,
			},
		},
	}
}

func (r *cdnZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *cdnZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cdnZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateCDNZoneRequest{
		Name:     plan.Name.ValueString(),
		PlanName: plan.PlanName.ValueString(),
	}
	if !plan.CustomDomain.IsNull() && !plan.CustomDomain.IsUnknown() {
		createReq.CustomDomain = plan.CustomDomain.ValueString()
	}
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
	}

	zone, err := r.client.CDN.CreateZone(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating CDN zone", err.Error())
		return
	}

	r.mapToState(&plan, zone)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cdnZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cdnZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.CDN.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading CDN zone", err.Error())
		return
	}

	r.mapToState(&state, zone)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cdnZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state cdnZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateCDNZoneRequest{}
	changed := false

	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
		changed = true
	}
	if !plan.CustomDomain.Equal(state.CustomDomain) {
		v := plan.CustomDomain.ValueString()
		updateReq.CustomDomain = &v
		changed = true
	}
	if !plan.SSLType.Equal(state.SSLType) {
		v := plan.SSLType.ValueString()
		updateReq.SSLType = &v
		changed = true
	}

	if changed {
		err := r.client.CDN.UpdateZone(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating CDN zone", err.Error())
			return
		}
	}

	// Re-read
	zone, err := r.client.CDN.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading CDN zone", err.Error())
		return
	}

	r.mapToState(&plan, zone)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cdnZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cdnZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.CDN.DeleteZone(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting CDN zone", err.Error())
		return
	}
}

func (r *cdnZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *cdnZoneResource) mapToState(state *cdnZoneResourceModel, zone *client.CDNZone) {
	state.ID = types.StringValue(zone.UUID)
	state.Name = types.StringValue(zone.Name)
	state.Domain = types.StringValue(zone.Domain)
	state.CustomDomain = types.StringValue(zone.CustomDomain)
	state.PlanName = types.StringValue(zone.PlanName)
	state.SSLType = types.StringValue(zone.SSLType)
	state.ProjectID = types.Int64Value(int64(zone.ProjectID))
	state.Status = types.StringValue(zone.Status)
	state.CreatedAt = types.StringValue(zone.CreatedAt)
	state.UpdatedAt = types.StringValue(zone.UpdatedAt)
}
