package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &floatingIPResource{}
	_ resource.ResourceWithConfigure   = &floatingIPResource{}
	_ resource.ResourceWithImportState = &floatingIPResource{}
)

func NewFloatingIPResource() resource.Resource {
	return &floatingIPResource{}
}

type floatingIPResource struct {
	client *client.Client
}

type floatingIPResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Address      types.String `tfsdk:"address"`
	IPType       types.String `tfsdk:"ip_type"`
	LocationName types.String `tfsdk:"location_name"`
	ReverseDNS   types.String `tfsdk:"reverse_dns"`
	AssignVPSID  types.Int64  `tfsdk:"assign_vps_id"`
	AssignBaremetalID types.Int64 `tfsdk:"assign_baremetal_id"`
	Status       types.String `tfsdk:"status"`
}

func (r *floatingIPResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_floating_ip"
}

func (r *floatingIPResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a floating IP address on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The floating IP address (used as identifier).",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"address": schema.StringAttribute{
				Description: "The acquired floating IP address.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ip_type": schema.StringAttribute{
				Description: "IP type: IPv4 or IPv6.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location_name": schema.StringAttribute{
				Description: "Location name (e.g., us-mia-1).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"reverse_dns": schema.StringAttribute{
				Description: "Reverse DNS (PTR record) hostname.",
				Optional:    true,
				Computed:    true,
			},
			"assign_vps_id": schema.Int64Attribute{
				Description: "VPS ID to assign the floating IP to.",
				Optional:    true,
			},
			"assign_baremetal_id": schema.Int64Attribute{
				Description: "Baremetal server ID to assign the floating IP to.",
				Optional:    true,
			},
			"status": schema.StringAttribute{
				Description: "Current status of the floating IP.",
				Computed:    true,
			},
		},
	}
}

func (r *floatingIPResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *floatingIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan floatingIPResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normalize ip_type to API format (IPv4/IPv6)
	ipType := plan.IPType.ValueString()
	switch strings.ToLower(ipType) {
	case "ipv4":
		ipType = "IPv4"
	case "ipv6":
		ipType = "IPv6"
	}

	// Acquire floating IP
	ip, err := r.client.FloatingIPs.Acquire(ctx, ipType, plan.LocationName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error acquiring floating IP", err.Error())
		return
	}

	plan.ID = types.StringValue(ip.Address)
	plan.Address = types.StringValue(ip.Address)
	plan.Status = types.StringValue(ip.Status)

	// Assign to VPS if specified
	if !plan.AssignVPSID.IsNull() && !plan.AssignVPSID.IsUnknown() {
		err := r.client.FloatingIPs.AssignToVPS(ctx, int(plan.AssignVPSID.ValueInt64()), ip.Address)
		if err != nil {
			resp.Diagnostics.AddError("Error assigning floating IP to VPS", err.Error())
			return
		}
	}

	// Assign to baremetal if specified
	if !plan.AssignBaremetalID.IsNull() && !plan.AssignBaremetalID.IsUnknown() {
		err := r.client.FloatingIPs.AssignToBaremetal(ctx, int(plan.AssignBaremetalID.ValueInt64()), ip.Address)
		if err != nil {
			resp.Diagnostics.AddError("Error assigning floating IP to baremetal", err.Error())
			return
		}
	}

	// Set reverse DNS if specified
	if !plan.ReverseDNS.IsNull() && !plan.ReverseDNS.IsUnknown() && plan.ReverseDNS.ValueString() != "" {
		err := r.client.FloatingIPs.ConfigureReverseDNS(ctx, ip.Address, plan.ReverseDNS.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error configuring reverse DNS", err.Error())
			return
		}
	} else {
		plan.ReverseDNS = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *floatingIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state floatingIPResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ip, err := r.client.FloatingIPs.GetByAddress(ctx, state.Address.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading floating IP", err.Error())
		return
	}

	state.Status = types.StringValue(ip.Status)
	state.LocationName = types.StringValue(ip.LocationName)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *floatingIPResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state floatingIPResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	address := state.Address.ValueString()

	// Handle assignment changes
	oldVPS := state.AssignVPSID
	newVPS := plan.AssignVPSID
	oldBM := state.AssignBaremetalID
	newBM := plan.AssignBaremetalID

	// Unassign if previously assigned and now different or removed
	needsUnassign := false
	if !oldVPS.IsNull() && (newVPS.IsNull() || oldVPS.ValueInt64() != newVPS.ValueInt64()) {
		needsUnassign = true
	}
	if !oldBM.IsNull() && (newBM.IsNull() || oldBM.ValueInt64() != newBM.ValueInt64()) {
		needsUnassign = true
	}

	if needsUnassign {
		if err := r.client.FloatingIPs.Unassign(ctx, address); err != nil {
			resp.Diagnostics.AddError("Error unassigning floating IP", err.Error())
			return
		}
	}

	// Assign to new target
	if !newVPS.IsNull() && !newVPS.IsUnknown() && (oldVPS.IsNull() || oldVPS.ValueInt64() != newVPS.ValueInt64()) {
		if err := r.client.FloatingIPs.AssignToVPS(ctx, int(newVPS.ValueInt64()), address); err != nil {
			resp.Diagnostics.AddError("Error assigning floating IP to VPS", err.Error())
			return
		}
	}
	if !newBM.IsNull() && !newBM.IsUnknown() && (oldBM.IsNull() || oldBM.ValueInt64() != newBM.ValueInt64()) {
		if err := r.client.FloatingIPs.AssignToBaremetal(ctx, int(newBM.ValueInt64()), address); err != nil {
			resp.Diagnostics.AddError("Error assigning floating IP to baremetal", err.Error())
			return
		}
	}

	// Handle reverse DNS changes
	if !plan.ReverseDNS.Equal(state.ReverseDNS) {
		rdns := ""
		if !plan.ReverseDNS.IsNull() {
			rdns = plan.ReverseDNS.ValueString()
		}
		if err := r.client.FloatingIPs.ConfigureReverseDNS(ctx, address, rdns); err != nil {
			resp.Diagnostics.AddError("Error configuring reverse DNS", err.Error())
			return
		}
	}

	plan.ID = state.ID
	plan.Address = state.Address
	plan.Status = state.Status

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *floatingIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state floatingIPResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	address := state.Address.ValueString()

	// Unassign first if assigned
	if (!state.AssignVPSID.IsNull() && state.AssignVPSID.ValueInt64() > 0) ||
		(!state.AssignBaremetalID.IsNull() && state.AssignBaremetalID.ValueInt64() > 0) {
		_ = r.client.FloatingIPs.Unassign(ctx, address)
	}

	// Release back to pool
	if err := r.client.FloatingIPs.Release(ctx, address); err != nil {
		resp.Diagnostics.AddError("Error releasing floating IP", err.Error())
		return
	}
}

func (r *floatingIPResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
