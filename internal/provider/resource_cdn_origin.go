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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &cdnOriginResource{}
	_ resource.ResourceWithConfigure   = &cdnOriginResource{}
	_ resource.ResourceWithImportState = &cdnOriginResource{}
)

func NewCDNOriginResource() resource.Resource {
	return &cdnOriginResource{}
}

type cdnOriginResource struct {
	client *client.Client
}

type cdnOriginResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ZoneUUID           types.String `tfsdk:"zone_uuid"`
	Name               types.String `tfsdk:"name"`
	OriginURL          types.String `tfsdk:"origin_url"`
	Address            types.String `tfsdk:"address"`
	Port               types.Int64  `tfsdk:"port"`
	Protocol           types.String `tfsdk:"protocol"`
	Weight             types.Int64  `tfsdk:"weight"`
	Priority           types.Int64  `tfsdk:"priority"`
	IsBackup           types.Bool   `tfsdk:"is_backup"`
	HealthCheckEnabled types.Bool   `tfsdk:"health_check_enabled"`
	HealthCheckPath    types.String `tfsdk:"health_check_path"`
	VerifySSL          types.Bool   `tfsdk:"verify_ssl"`
	HostHeader         types.String `tfsdk:"host_header"`
	BasePath           types.String `tfsdk:"base_path"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	HealthStatus       types.String `tfsdk:"health_status"`
}

func (r *cdnOriginResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cdn_origin"
}

func (r *cdnOriginResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CDN origin server on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the origin.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone_uuid": schema.StringAttribute{
				Description: "The UUID of the CDN zone.",
				Required:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Origin name.",
				Required:    true,
			},
			"origin_url": schema.StringAttribute{
				Description: "Full origin URL (auto-parsed). Use this OR address.",
				Optional:    true,
			},
			"address": schema.StringAttribute{
				Description: "Origin IP or hostname. Use this OR origin_url.",
				Optional:    true,
				Computed:    true,
			},
			"port": schema.Int64Attribute{
				Description: "Origin port (1-65535).",
				Optional:    true,
				Computed:    true,
			},
			"protocol": schema.StringAttribute{
				Description: "Protocol: http or https.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("https"),
			},
			"weight": schema.Int64Attribute{
				Description: "Load balancing weight (1-1000).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(100),
			},
			"priority": schema.Int64Attribute{
				Description: "Priority (1-100).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
			},
			"is_backup": schema.BoolAttribute{
				Description: "Mark as backup origin.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"health_check_enabled": schema.BoolAttribute{
				Description: "Enable health checks.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"health_check_path": schema.StringAttribute{
				Description: "Health check path.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("/health"),
			},
			"verify_ssl": schema.BoolAttribute{
				Description: "Verify SSL certificates.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"host_header": schema.StringAttribute{
				Description: "Custom Host header.",
				Optional:    true,
				Computed:    true,
			},
			"base_path": schema.StringAttribute{
				Description: "Base path prefix.",
				Optional:    true,
				Computed:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the origin is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"health_status": schema.StringAttribute{
				Description: "Current health status.",
				Computed:    true,
			},
		},
	}
}

func (r *cdnOriginResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *cdnOriginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cdnOriginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateCDNOriginRequest{
		Name:               plan.Name.ValueString(),
		Weight:             int(plan.Weight.ValueInt64()),
		Priority:           int(plan.Priority.ValueInt64()),
		IsBackup:           plan.IsBackup.ValueBool(),
		HealthCheckEnabled: plan.HealthCheckEnabled.ValueBool(),
		HealthCheckPath:    plan.HealthCheckPath.ValueString(),
		VerifySSL:          plan.VerifySSL.ValueBool(),
		Enabled:            plan.Enabled.ValueBool(),
	}

	if !plan.OriginURL.IsNull() && !plan.OriginURL.IsUnknown() {
		createReq.OriginURL = plan.OriginURL.ValueString()
	}
	if !plan.Address.IsNull() && !plan.Address.IsUnknown() {
		createReq.Address = plan.Address.ValueString()
	}
	if !plan.Port.IsNull() && !plan.Port.IsUnknown() {
		p := int(plan.Port.ValueInt64())
		createReq.Port = &p
	}
	if !plan.Protocol.IsNull() && !plan.Protocol.IsUnknown() {
		createReq.Protocol = plan.Protocol.ValueString()
	}
	if !plan.HostHeader.IsNull() && !plan.HostHeader.IsUnknown() {
		createReq.HostHeader = plan.HostHeader.ValueString()
	}
	if !plan.BasePath.IsNull() && !plan.BasePath.IsUnknown() {
		createReq.BasePath = plan.BasePath.ValueString()
	}

	origin, err := r.client.CDN.CreateOrigin(ctx, plan.ZoneUUID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating CDN origin", err.Error())
		return
	}

	r.mapToState(&plan, origin)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cdnOriginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cdnOriginResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	origins, err := r.client.CDN.ListOrigins(ctx, state.ZoneUUID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading CDN origins", err.Error())
		return
	}

	var found *client.CDNOrigin
	for _, o := range origins {
		if o.UUID == state.ID.ValueString() {
			found = &o
			break
		}
	}

	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.mapToState(&state, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cdnOriginResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state cdnOriginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateCDNOriginRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Address.Equal(state.Address) {
		v := plan.Address.ValueString()
		updateReq.Address = &v
	}
	if !plan.Port.Equal(state.Port) {
		v := int(plan.Port.ValueInt64())
		updateReq.Port = &v
	}
	if !plan.Protocol.Equal(state.Protocol) {
		v := plan.Protocol.ValueString()
		updateReq.Protocol = &v
	}
	if !plan.Weight.Equal(state.Weight) {
		v := int(plan.Weight.ValueInt64())
		updateReq.Weight = &v
	}
	if !plan.Priority.Equal(state.Priority) {
		v := int(plan.Priority.ValueInt64())
		updateReq.Priority = &v
	}
	if !plan.HostHeader.Equal(state.HostHeader) {
		v := plan.HostHeader.ValueString()
		updateReq.HostHeader = &v
	}
	if !plan.BasePath.Equal(state.BasePath) {
		v := plan.BasePath.ValueString()
		updateReq.BasePath = &v
	}

	err := r.client.CDN.UpdateOrigin(ctx, state.ZoneUUID.ValueString(), state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating CDN origin", err.Error())
		return
	}

	plan.ID = state.ID
	plan.ZoneUUID = state.ZoneUUID
	plan.HealthStatus = state.HealthStatus
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cdnOriginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cdnOriginResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.CDN.DeleteOrigin(ctx, state.ZoneUUID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting CDN origin", err.Error())
		return
	}
}

func (r *cdnOriginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: zone_uuid/origin_uuid")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_uuid"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *cdnOriginResource) mapToState(state *cdnOriginResourceModel, origin *client.CDNOrigin) {
	state.ID = types.StringValue(origin.UUID)
	state.Name = types.StringValue(origin.Name)
	state.Address = types.StringValue(origin.Address)
	state.Port = types.Int64Value(int64(origin.Port))
	state.Protocol = types.StringValue(origin.Protocol)
	state.Weight = types.Int64Value(int64(origin.Weight))
	state.Priority = types.Int64Value(int64(origin.Priority))
	state.IsBackup = types.BoolValue(origin.IsBackup)
	state.HealthCheckEnabled = types.BoolValue(origin.HealthCheckEnabled)
	state.HealthCheckPath = types.StringValue(origin.HealthCheckPath)
	state.VerifySSL = types.BoolValue(origin.VerifySSL)
	state.HostHeader = types.StringValue(origin.HostHeader)
	state.BasePath = types.StringValue(origin.BasePath)
	state.Enabled = types.BoolValue(origin.Enabled)
	state.HealthStatus = types.StringValue(origin.HealthStatus)
}
