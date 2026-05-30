package provider

import (
	"context"
	"fmt"
	"time"

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
	_ resource.Resource                = &loadBalancerResource{}
	_ resource.ResourceWithConfigure   = &loadBalancerResource{}
	_ resource.ResourceWithImportState = &loadBalancerResource{}
)

func NewLoadBalancerResource() resource.Resource {
	return &loadBalancerResource{}
}

type loadBalancerResource struct {
	client *client.Client
}

type loadBalancerResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Label        types.String `tfsdk:"label"`
	PlanName     types.String `tfsdk:"plan_name"`
	LocationName types.String `tfsdk:"location_name"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	NetworkID    types.Int64  `tfsdk:"network_id"`
	Status       types.String `tfsdk:"status"`
	IPAddress    types.String `tfsdk:"ip_address"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *loadBalancerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_loadbalancer"
}

func (r *loadBalancerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a load balancer on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the load balancer.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the load balancer.",
				Required:    true,
			},
			"label": schema.StringAttribute{
				Description: "Optional label for the load balancer.",
				Optional:    true,
				Computed:    true,
			},
			"plan_name": schema.StringAttribute{
				Description: "The plan name (e.g., lb.small).",
				Required:    true,
			},
			"location_name": schema.StringAttribute{
				Description: "The location name (e.g., us-mia-1).",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID. Uses default project if not specified.",
				Optional:    true,
				Computed:    true,
			},
			"network_id": schema.Int64Attribute{
				Description: "The private network ID to attach the load balancer to. Requires replacement if changed.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Current status of the load balancer.",
				Computed:    true,
			},
			"ip_address": schema.StringAttribute{
				Description: "The floating IP address assigned to the load balancer.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "When the load balancer was created.",
				Computed:    true,
			},
		},
	}
}

func (r *loadBalancerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *loadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan loadBalancerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateLoadBalancerRequest{
		Name:         plan.Name.ValueString(),
		PlanName:     plan.PlanName.ValueString(),
		LocationName: plan.LocationName.ValueString(),
	}
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
	}
	if !plan.Label.IsNull() && !plan.Label.IsUnknown() {
		createReq.Label = plan.Label.ValueString()
	}
	if !plan.NetworkID.IsNull() && !plan.NetworkID.IsUnknown() {
		nid := int(plan.NetworkID.ValueInt64())
		createReq.NetworkID = &nid
	}

	lb, err := r.client.LoadBalancer.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating load balancer", err.Error())
		return
	}

	// Wait for load balancer to be ready (exit deploying state)
	if lb.Status == "deploying" || lb.Status == "provisioning" {
		for i := 0; i < 60; i++ {
			time.Sleep(5 * time.Second)
			lb, err = r.client.LoadBalancer.Get(ctx, lb.UUID)
			if err != nil {
				resp.Diagnostics.AddError("Error reading load balancer after creation", err.Error())
				return
			}
			if lb.Status == "active" || lb.Status == "running" {
				break
			}
		}
	}

	r.mapToState(&plan, lb)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *loadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state loadBalancerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lb, err := r.client.LoadBalancer.Get(ctx, state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading load balancer", err.Error())
		return
	}

	r.mapToState(&state, lb)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *loadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state loadBalancerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Handle plan resize
	if !plan.PlanName.Equal(state.PlanName) {
		err := r.client.LoadBalancer.Resize(ctx, state.ID.ValueString(), plan.PlanName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error resizing load balancer", err.Error())
			return
		}
	}

	// Handle name/label update
	if !plan.Name.Equal(state.Name) || !plan.Label.Equal(state.Label) {
		updateReq := &client.UpdateLoadBalancerRequest{}
		if !plan.Name.Equal(state.Name) {
			v := plan.Name.ValueString()
			updateReq.Name = &v
		}
		if !plan.Label.Equal(state.Label) {
			v := plan.Label.ValueString()
			updateReq.Label = &v
		}
		_, err := r.client.LoadBalancer.Update(ctx, state.ID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating load balancer", err.Error())
			return
		}
	}

	// Re-read to get updated state
	lb, err := r.client.LoadBalancer.Get(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading load balancer", err.Error())
		return
	}

	r.mapToState(&plan, lb)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *loadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state loadBalancerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retry delete in case there are pending tasks (e.g. from a listener just deleted)
	var err error
	for attempt := 0; attempt < 12; attempt++ {
		err = r.client.LoadBalancer.Delete(ctx, state.ID.ValueString())
		if err == nil {
			return
		}
		if apiErr, ok := err.(*client.APIError); ok && apiErr.StatusCode == 400 {
			time.Sleep(10 * time.Second)
			continue
		}
		break
	}
	resp.Diagnostics.AddError("Error deleting load balancer", err.Error())
}

func (r *loadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *loadBalancerResource) mapToState(state *loadBalancerResourceModel, lb *client.LoadBalancer) {
	state.ID = types.StringValue(lb.UUID)
	state.Name = types.StringValue(lb.Name)
	state.Label = types.StringValue(lb.Label)
	state.Status = types.StringValue(lb.Status)
	state.LocationName = types.StringValue(lb.LocationName)
	state.ProjectID = types.Int64Value(int64(lb.ProjectID))
	state.CreatedAt = types.StringValue(lb.CreatedAt)

	if lb.PlanName != "" {
		state.PlanName = types.StringValue(lb.PlanName)
	} else if lb.Plan != nil {
		state.PlanName = types.StringValue(lb.Plan.Name)
	}

	// Extract IPv4 address from floating_ips array
	state.IPAddress = types.StringValue("")
	for _, fip := range lb.FloatingIPs {
		if fip.Type == "IPv4" {
			state.IPAddress = types.StringValue(fip.Address)
			break
		}
	}
}
