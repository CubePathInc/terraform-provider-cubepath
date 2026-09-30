package provider

import (
	"context"
	"strconv"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &networkBGPPeerResource{}
	_ resource.ResourceWithConfigure   = &networkBGPPeerResource{}
	_ resource.ResourceWithImportState = &networkBGPPeerResource{}
)

func NewNetworkBGPPeerResource() resource.Resource {
	return &networkBGPPeerResource{}
}

type networkBGPPeerResource struct {
	client *client.Client
}

type networkBGPPeerModel struct {
	ID             types.String `tfsdk:"id"`
	NetworkID      types.Int64  `tfsdk:"network_id"`
	PeerType       types.String `tfsdk:"peer_type"`
	PeerTarget     types.String `tfsdk:"peer_target"`
	RemoteASN      types.Int64  `tfsdk:"remote_asn"`
	MaxPrefix      types.Int64  `tfsdk:"max_prefix"`
	Description    types.String `tfsdk:"description"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	ResolvedPeerIP types.String `tfsdk:"resolved_peer_ip"`
	State          types.String `tfsdk:"state"`
}

func (r *networkBGPPeerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_bgp_peer"
}

func (r *networkBGPPeerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a BGP peer (Dynamic Routes) of a private network: an eBGP session between the network " +
			"gateway (AS 64512) and a router inside the network, which announces the prefixes it can reach. " +
			"The network needs at least one server attached. Up to 8 peers per network. " +
			"Import with network_id:peer_id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The ID of the peer.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"network_id": schema.Int64Attribute{
				Description:   "ID of the private network. Changing it forces a new peer.",
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"peer_type": schema.StringAttribute{
				Description:   "How the neighbor is given: ip, vps or baremetal. Changing it forces a new peer.",
				Required:      true,
				Validators:    []validator.String{StringOneOf("ip", "vps", "baremetal")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"peer_target": schema.StringAttribute{
				Description: "An IP address inside the network, or the ID of a VPS or baremetal server attached to it. " +
					"Changing it forces a new peer.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"remote_asn": schema.Int64Attribute{
				Description:   "AS number of the neighbor, not 64512. Changing it forces a new peer.",
				Required:      true,
				Validators:    []validator.Int64{Int64Between(1, 4294967295)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"max_prefix": schema.Int64Attribute{
				Description: "Maximum prefixes accepted from the neighbor, 1 to 1000. Defaults to 100.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(100),
				Validators:  []validator.Int64{Int64Between(1, 1000)},
			},
			"description": schema.StringAttribute{
				Description: "Description.",
				Optional:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the session is configured. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"resolved_peer_ip": schema.StringAttribute{
				Description: "IP address of the neighbor.",
				Computed:    true,
			},
			"state": schema.StringAttribute{
				Description: "Last observed BGP session state (for example Established).",
				Computed:    true,
			},
		},
	}
}

func (r *networkBGPPeerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *networkBGPPeerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkBGPPeerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	networkID := int(plan.NetworkID.ValueInt64())

	id, err := r.client.Networks.CreateBGPPeer(ctx, networkID, &client.CreateBGPPeerRequest{
		PeerType:    plan.PeerType.ValueString(),
		PeerTarget:  plan.PeerTarget.ValueString(),
		RemoteASN:   plan.RemoteASN.ValueInt64(),
		MaxPrefix:   knownInt(plan.MaxPrefix),
		Description: knownString(plan.Description),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating BGP peer", err.Error())
		return
	}
	plan.ID = types.StringValue(id)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), plan.NetworkID)...)

	if !plan.Enabled.ValueBool() {
		disabled := false
		if err := r.client.Networks.UpdateBGPPeer(ctx, networkID, id, &client.UpdateBGPPeerRequest{Enabled: &disabled}); err != nil {
			resp.Diagnostics.AddError("Error disabling BGP peer", err.Error())
			return
		}
	}

	peer, err := r.client.Networks.GetBGPPeer(ctx, networkID, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading BGP peer", err.Error())
		return
	}
	mapBGPPeer(&plan, peer)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *networkBGPPeerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkBGPPeerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	peer, err := r.client.Networks.GetBGPPeer(ctx, int(state.NetworkID.ValueInt64()), state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading BGP peer", err.Error())
		return
	}
	mapBGPPeer(&state, peer)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkBGPPeerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state networkBGPPeerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	networkID := int(state.NetworkID.ValueInt64())

	description := plan.Description.ValueString() // null clears it
	updateReq := &client.UpdateBGPPeerRequest{
		MaxPrefix:   knownInt(plan.MaxPrefix),
		Enabled:     knownBool(plan.Enabled),
		Description: &description,
	}
	if err := r.client.Networks.UpdateBGPPeer(ctx, networkID, state.ID.ValueString(), updateReq); err != nil {
		resp.Diagnostics.AddError("Error updating BGP peer", err.Error())
		return
	}

	peer, err := r.client.Networks.GetBGPPeer(ctx, networkID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading BGP peer", err.Error())
		return
	}
	plan.ID = state.ID
	mapBGPPeer(&plan, peer)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *networkBGPPeerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkBGPPeerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.Networks.DeleteBGPPeer(ctx, int(state.NetworkID.ValueInt64()), state.ID.ValueString())
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting BGP peer", err.Error())
	}
}

func (r *networkBGPPeerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Import ID must be in the format "networkID:peerID".`)
		return
	}
	networkID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "networkID must be a number: "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), networkID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func mapBGPPeer(m *networkBGPPeerModel, p *client.BGPPeer) {
	m.ID = types.StringValue(p.ID)
	m.NetworkID = types.Int64Value(int64(p.NetworkID))
	m.PeerType = types.StringValue(p.PeerType)
	m.PeerTarget = types.StringValue(p.PeerTarget)
	m.RemoteASN = types.Int64Value(p.RemoteASN)
	m.MaxPrefix = types.Int64Value(int64(p.MaxPrefix))
	m.Description = stringOrNull(p.Description)
	m.Enabled = types.BoolValue(p.Enabled)
	m.ResolvedPeerIP = stringOrNull(p.ResolvedPeerIP)
	m.State = stringOrNull(p.LastState)
}
