package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
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
	_ resource.Resource                = &networkRouteResource{}
	_ resource.ResourceWithConfigure   = &networkRouteResource{}
	_ resource.ResourceWithImportState = &networkRouteResource{}
)

func NewNetworkRouteResource() resource.Resource {
	return &networkRouteResource{}
}

type networkRouteResource struct {
	client *client.Client
}

type networkRouteResourceModel struct {
	ID            types.String `tfsdk:"id"`
	NetworkID     types.Int64  `tfsdk:"network_id"`
	Destination   types.String `tfsdk:"destination"`
	NextHopType   types.String `tfsdk:"next_hop_type"`
	NextHopTarget types.String `tfsdk:"next_hop_target"`
	Description   types.String `tfsdk:"description"`
}

func (r *networkRouteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_route"
}

func (r *networkRouteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a static route within a private network on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The ID of the network route.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.Int64Attribute{
				Description: "The ID of the network to add the route to. Requires replacement if changed.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"destination": schema.StringAttribute{
				Description: "The destination CIDR for this route (e.g., 192.168.10.0/24). Requires replacement if changed.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"next_hop_type": schema.StringAttribute{
				Description: "The next hop type. One of: ip, vps, baremetal. Requires replacement if changed.",
				Required:    true,
				Validators: []validator.String{
					stringOneOfValidator{valid: []string{"ip", "vps", "baremetal"}},
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"next_hop_target": schema.StringAttribute{
				Description: "The next hop target: an IP address when next_hop_type is 'ip', or the numeric VPS/baremetal ID when next_hop_type is 'vps'/'baremetal'. Requires replacement if changed.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Optional description for the route.",
				Optional:    true,
				Computed:    true,
			},
		},
	}
}

func (r *networkRouteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *networkRouteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan networkRouteResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	networkID := int(plan.NetworkID.ValueInt64())
	createReq := &client.CreateNetworkRouteRequest{
		Destination:   plan.Destination.ValueString(),
		NextHopType:   plan.NextHopType.ValueString(),
		NextHopTarget: plan.NextHopTarget.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		createReq.Description = &v
	}

	route, err := r.client.NetworkRoutes.Create(ctx, networkID, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating network route", err.Error())
		return
	}

	r.mapToState(&plan, route)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *networkRouteResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state networkRouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	networkID := int(state.NetworkID.ValueInt64())
	routeID := state.ID.ValueString()

	routes, err := r.client.NetworkRoutes.List(ctx, networkID)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading network routes", err.Error())
		return
	}

	for _, route := range routes {
		if route.ID == routeID {
			r.mapToState(&state, &route)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	// Route not found in list — remove from state
	resp.State.RemoveResource(ctx)
}

func (r *networkRouteResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All mutable attributes are RequiresReplace; description is the only optional
	// field and since it's also part of the resource identity we leave it as-is.
	// This method will not be called unless description changes, but all fields
	// are RequiresReplace so Terraform will destroy+create instead.
}

func (r *networkRouteResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state networkRouteResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	networkID := int(state.NetworkID.ValueInt64())
	routeID := state.ID.ValueString()

	if err := r.client.NetworkRoutes.Delete(ctx, networkID, routeID); err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Error deleting network route", err.Error())
	}
}

// ImportState supports "networkID:routeID" composite import ID.
func (r *networkRouteResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			`Import ID must be in the format "networkID:routeID" (e.g., "42:abc123").`,
		)
		return
	}

	networkID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Import ID",
			fmt.Sprintf("Network ID %q is not a valid integer: %s", parts[0], err))
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("network_id"), networkID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func (r *networkRouteResource) mapToState(state *networkRouteResourceModel, route *client.NetworkRoute) {
	state.ID = types.StringValue(route.ID)
	state.NetworkID = types.Int64Value(int64(route.NetworkID))
	state.Destination = types.StringValue(route.Destination)
	state.NextHopType = types.StringValue(route.NextHopType)
	state.NextHopTarget = types.StringValue(route.NextHopTarget)
	state.Description = types.StringValue(route.Description)
}

// stringOneOfValidator validates that a string is one of a set of allowed values.
type stringOneOfValidator struct {
	valid []string
}

func (v stringOneOfValidator) Description(_ context.Context) string {
	return fmt.Sprintf("value must be one of: %s", strings.Join(v.valid, ", "))
}

func (v stringOneOfValidator) MarkdownDescription(_ context.Context) string {
	return v.Description(nil)
}

func (v stringOneOfValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	val := req.ConfigValue.ValueString()
	for _, allowed := range v.valid {
		if val == allowed {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid Value",
		fmt.Sprintf("Expected one of [%s], got %q.", strings.Join(v.valid, ", "), val),
	)
}
