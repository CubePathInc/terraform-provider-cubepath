package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &dnsZoneResource{}
	_ resource.ResourceWithConfigure   = &dnsZoneResource{}
	_ resource.ResourceWithImportState = &dnsZoneResource{}
)

func NewDNSZoneResource() resource.Resource {
	return &dnsZoneResource{}
}

type dnsZoneResource struct {
	client *client.Client
}

type dnsZoneResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Domain       types.String `tfsdk:"domain"`
	Status       types.String `tfsdk:"status"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	RecordsCount types.Int64  `tfsdk:"records_count"`
	Nameservers  types.List   `tfsdk:"nameservers"`
	CreatedAt    types.String `tfsdk:"created_at"`

	SOARefresh    types.Int64  `tfsdk:"soa_refresh"`
	SOARetry      types.Int64  `tfsdk:"soa_retry"`
	SOAExpire     types.Int64  `tfsdk:"soa_expire"`
	SOAMinimum    types.Int64  `tfsdk:"soa_minimum"`
	SOAHostmaster types.String `tfsdk:"soa_hostmaster"`
}

func (r *dnsZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zone"
}

func (r *dnsZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DNS zone on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the DNS zone.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain": schema.StringAttribute{
				Description: "The domain name for the zone.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Description: "The status of the DNS zone.",
				Computed:    true,
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID. Uses default project if not specified. Changing it moves the zone in place.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"soa_refresh": soaTimer("SOA refresh, in seconds (300-86400). Defaults to 3600.", 300, 86400),
			"soa_retry":   soaTimer("SOA retry, in seconds (60-86400). Defaults to 900.", 60, 86400),
			"soa_expire":  soaTimer("SOA expire, in seconds (86400-2419200). Defaults to 604800.", 86400, 2419200),
			"soa_minimum": soaTimer("SOA minimum (negative caching) TTL, in seconds (60-86400). Defaults to 300.", 60, 86400),
			"soa_hostmaster": schema.StringAttribute{
				Description: "SOA hostmaster email. Defaults to hostmaster@<domain>. Once set it cannot go back to the default.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"records_count": schema.Int64Attribute{
				Description: "Number of DNS records in the zone.",
				Computed:    true,
			},
			"nameservers": schema.ListAttribute{
				Description: "Assigned nameservers for the zone.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"created_at": schema.StringAttribute{
				Description: "When the zone was created.",
				Computed:    true,
			},
		},
	}
}

func (r *dnsZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *dnsZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateDNSZoneRequest{
		Domain: plan.Domain.ValueString(),
	}
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
	}

	zone, err := r.client.DNS.CreateZone(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS zone", err.Error())
		return
	}

	plan.ID = types.StringValue(zone.UUID)
	plan.Status = types.StringValue(zone.Status)
	plan.ProjectID = types.Int64Value(int64(zone.ProjectID))
	plan.RecordsCount = types.Int64Value(int64(zone.RecordsCount))
	plan.CreatedAt = types.StringValue(zone.CreatedAt)

	nameservers, diags := types.ListValueFrom(ctx, types.StringType, zone.Nameservers)
	resp.Diagnostics.Append(diags...)
	plan.Nameservers = nameservers

	// Save the zone before the SOA call so a failure does not leave it untracked.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	r.applySOA(ctx, &plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnsZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNS zone", err.Error())
		return
	}

	state.Domain = types.StringValue(zone.Domain)
	state.Status = types.StringValue(zone.Status)
	state.ProjectID = types.Int64Value(int64(zone.ProjectID))
	state.RecordsCount = types.Int64Value(int64(zone.RecordsCount))
	state.CreatedAt = types.StringValue(zone.CreatedAt)

	nameservers, diags := types.ListValueFrom(ctx, types.StringType, zone.Nameservers)
	resp.Diagnostics.Append(diags...)
	state.Nameservers = nameservers

	soa, err := r.client.DNS.GetSOA(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone SOA", err.Error())
		return
	}
	mapSOA(&state, soa)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update moves the zone to another project and changes its SOA; the domain forces a new zone.
func (r *dnsZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state dnsZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	if !plan.ProjectID.IsUnknown() && !plan.ProjectID.Equal(state.ProjectID) {
		if err := r.client.DNS.MoveZone(ctx, id, int(plan.ProjectID.ValueInt64())); err != nil {
			resp.Diagnostics.AddError("Error moving DNS zone", err.Error())
			return
		}
	}
	r.applySOA(ctx, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone", err.Error())
		return
	}
	plan.ID = state.ID
	plan.Status = types.StringValue(zone.Status)
	plan.ProjectID = types.Int64Value(int64(zone.ProjectID))
	plan.RecordsCount = types.Int64Value(int64(zone.RecordsCount))
	plan.CreatedAt = types.StringValue(zone.CreatedAt)
	nameservers, diags := types.ListValueFrom(ctx, types.StringType, zone.Nameservers)
	resp.Diagnostics.Append(diags...)
	plan.Nameservers = nameservers

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func soaTimer(description string, min, max int64) schema.Int64Attribute {
	return schema.Int64Attribute{
		Description: description,
		Optional:    true,
		Computed:    true,
		Validators:  []validator.Int64{Int64Between(min, max)},
		PlanModifiers: []planmodifier.Int64{
			int64planmodifier.UseStateForUnknown(),
		},
	}
}

// applySOA sends the SOA values set in the configuration when they differ from the prior
// state (prior is nil on create), then records the resulting SOA.
func (r *dnsZoneResource) applySOA(ctx context.Context, plan, prior *dnsZoneResourceModel, diags *diag.Diagnostics) {
	req := &client.UpdateDNSZoneSOARequest{}
	changed := false
	setInt := func(p types.Int64, s *types.Int64, dst **int) {
		if v := knownInt(p); v != nil && (s == nil || !p.Equal(*s)) {
			*dst = v
			changed = true
		}
	}
	var refresh, retry, expire, minimum *types.Int64
	var hostmaster *types.String
	if prior != nil {
		refresh, retry, expire, minimum = &prior.SOARefresh, &prior.SOARetry, &prior.SOAExpire, &prior.SOAMinimum
		hostmaster = &prior.SOAHostmaster
	}
	setInt(plan.SOARefresh, refresh, &req.Refresh)
	setInt(plan.SOARetry, retry, &req.Retry)
	setInt(plan.SOAExpire, expire, &req.Expire)
	setInt(plan.SOAMinimum, minimum, &req.Minimum)
	if v := knownString(plan.SOAHostmaster); v != nil && (hostmaster == nil || !plan.SOAHostmaster.Equal(*hostmaster)) {
		req.Hostmaster = v
		changed = true
	}

	var soa *client.DNSZoneSOA
	var err error
	if changed {
		soa, err = r.client.DNS.UpdateSOA(ctx, plan.ID.ValueString(), req)
	} else {
		soa, err = r.client.DNS.GetSOA(ctx, plan.ID.ValueString())
	}
	if err != nil {
		diags.AddError("Error updating DNS zone SOA", err.Error())
		return
	}
	mapSOA(plan, soa)
}

func mapSOA(m *dnsZoneResourceModel, soa *client.DNSZoneSOA) {
	m.SOARefresh = types.Int64Value(int64(soa.Refresh))
	m.SOARetry = types.Int64Value(int64(soa.Retry))
	m.SOAExpire = types.Int64Value(int64(soa.Expire))
	m.SOAMinimum = types.Int64Value(int64(soa.Minimum))
	m.SOAHostmaster = types.StringValue(soa.Hostmaster)
}

func (r *dnsZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DNS.DeleteZone(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting DNS zone", err.Error())
		return
	}
}

func (r *dnsZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
