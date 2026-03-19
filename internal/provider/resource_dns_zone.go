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
				Description: "The project ID. Uses default project if not specified.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
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
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
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

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// DNS zones don't support updates - domain and project_id both require replacement
	resp.Diagnostics.AddError("Update Not Supported",
		"DNS zones cannot be updated. Changes require recreating the resource.")
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
