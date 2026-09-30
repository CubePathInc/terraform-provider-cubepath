package provider

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &lbTargetResource{}
	_ resource.ResourceWithConfigure   = &lbTargetResource{}
	_ resource.ResourceWithImportState = &lbTargetResource{}
	_ resource.Resource                = &lbHealthCheckResource{}
	_ resource.ResourceWithConfigure   = &lbHealthCheckResource{}
	_ resource.ResourceWithImportState = &lbHealthCheckResource{}
)

// A load balancer applies one configuration change at a time: while a previous change is
// pending it answers 409, and while it is deploying or updating it answers 400 ("... while
// Load Balancer is in 'updating' status"). Both happen when a listener, its targets and its
// health check are created in the same run.
const lbChangeRetryTimeout = 15 * time.Minute

func lbBusy(err error) bool {
	if isConflict(err) {
		return true
	}
	var apiErr *client.APIError
	return errors.As(err, &apiErr) && apiErr.IsBadRequest() && strings.Contains(apiErr.Detail, "Load Balancer is in '")
}

func lbRetry(ctx context.Context, fn func() error) error {
	return retryWhile(ctx, lbChangeRetryTimeout, 10*time.Second, lbBusy, fn)
}

// ---- cubepath_lb_target ----

func NewLBTargetResource() resource.Resource {
	return &lbTargetResource{}
}

type lbTargetResource struct {
	client *client.Client
}

type lbTargetModel struct {
	ID             types.String `tfsdk:"id"`
	LoadBalancerID types.String `tfsdk:"loadbalancer_id"`
	ListenerID     types.String `tfsdk:"listener_id"`
	TargetType     types.String `tfsdk:"target_type"`
	TargetID       types.String `tfsdk:"target_id"`
	Port           types.Int64  `tfsdk:"port"`
	Weight         types.Int64  `tfsdk:"weight"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	TargetName     types.String `tfsdk:"target_name"`
	HealthStatus   types.String `tfsdk:"health_status"`
}

func (r *lbTargetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_target"
}

func (r *lbTargetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a backend of a load balancer listener: a VPS, a baremetal server or a whole " +
			"availability group. Import with loadbalancer_id/listener_id/target_id (the target row UUID).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "UUID of the target.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"loadbalancer_id": schema.StringAttribute{
				Description:   "UUID of the load balancer. Changing it forces a new target.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"listener_id": schema.StringAttribute{
				Description:   "UUID of the listener. Changing it forces a new target.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"target_type": schema.StringAttribute{
				Description:   "Backend type: vps, baremetal or availability_group. Changing it forces a new target.",
				Required:      true,
				Validators:    []validator.String{StringOneOf("vps", "baremetal", "availability_group")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"target_id": schema.StringAttribute{
				Description:   "ID of the VPS or baremetal server, or UUID of the availability group. Changing it forces a new target.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"port": schema.Int64Attribute{
				Description: "Port on the backend. Omit it to use the listener's target_port (removing it replaces the target).",
				Optional:    true,
				Validators:  []validator.Int64{Int64Between(1, 65535)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplaceIf(requiresReplaceIfRemovedInt64,
					"Removing the port replaces the target.", "Removing the port replaces the target.")},
			},
			"weight": schema.Int64Attribute{
				Description: "Weight, 1 to 100. Defaults to 100.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(100),
				Validators:  []validator.Int64{Int64Between(1, 100)},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the load balancer sends traffic to this target. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"target_name": schema.StringAttribute{
				Description:   "Name of the backend.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"health_status": schema.StringAttribute{
				Description: "Health reported by the load balancer: healthy, unhealthy, draining or unknown.",
				Computed:    true,
			},
		},
	}
}

func (r *lbTargetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *lbTargetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lbTargetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enabled := plan.Enabled.ValueBool()
	addReq := &client.AddTargetRequest{
		TargetType: plan.TargetType.ValueString(),
		TargetUUID: plan.TargetID.ValueString(),
		Port:       knownInt(plan.Port),
		Weight:     int(plan.Weight.ValueInt64()),
		Enabled:    &enabled,
	}
	var target *client.LBTarget
	err := lbRetry(ctx, func() error {
		var err error
		target, err = r.client.LoadBalancer.AddTarget(ctx, plan.LoadBalancerID.ValueString(), plan.ListenerID.ValueString(), addReq)
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Error adding load balancer target", err.Error())
		return
	}
	plan.ID = types.StringValue(target.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	found, err := r.find(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error reading load balancer target", err.Error())
		return
	}
	mapLBTarget(&plan, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *lbTargetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lbTargetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	found, err := r.find(ctx, &state)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading load balancer target", err.Error())
		return
	}
	mapLBTarget(&state, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbTargetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state lbTargetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateTargetRequest{
		Weight:  knownInt(plan.Weight),
		Enabled: knownBool(plan.Enabled),
		Port:    knownInt(plan.Port),
	}
	err := lbRetry(ctx, func() error {
		_, err := r.client.LoadBalancer.UpdateTarget(ctx, state.LoadBalancerID.ValueString(), state.ListenerID.ValueString(), state.ID.ValueString(), updateReq)
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating load balancer target", err.Error())
		return
	}

	plan.ID = state.ID
	found, err := r.find(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error reading load balancer target", err.Error())
		return
	}
	mapLBTarget(&plan, found)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *lbTargetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lbTargetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := lbRetry(ctx, func() error {
		return r.client.LoadBalancer.RemoveTarget(ctx, state.LoadBalancerID.ValueString(), state.ListenerID.ValueString(), state.ID.ValueString())
	})
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error removing load balancer target", err.Error())
	}
}

func (r *lbTargetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: loadbalancer_id/listener_id/target_id")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("loadbalancer_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("listener_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

// find looks the target up in its listener. It returns a 404 APIError when any of the
// load balancer, listener or target is gone.
func (r *lbTargetResource) find(ctx context.Context, m *lbTargetModel) (*client.LBTarget, error) {
	listener, err := r.client.LoadBalancer.GetListener(ctx, m.LoadBalancerID.ValueString(), m.ListenerID.ValueString())
	if err != nil {
		return nil, err
	}
	for i := range listener.Targets {
		if listener.Targets[i].UUID == m.ID.ValueString() {
			return &listener.Targets[i], nil
		}
	}
	return nil, &client.APIError{StatusCode: 404, Message: "Not Found", Detail: "Target not found"}
}

func mapLBTarget(m *lbTargetModel, t *client.LBTarget) {
	m.TargetType = types.StringValue(t.TargetType)
	m.TargetID = types.StringValue(t.TargetUUID)
	if t.Port > 0 {
		m.Port = types.Int64Value(int64(t.Port))
	} else {
		m.Port = types.Int64Null()
	}
	m.Weight = types.Int64Value(int64(t.Weight))
	m.Enabled = types.BoolValue(t.Enabled)
	m.TargetName = types.StringValue(t.TargetName)
	m.HealthStatus = types.StringValue(t.HealthStatus)
}

// ---- cubepath_lb_health_check ----

func NewLBHealthCheckResource() resource.Resource {
	return &lbHealthCheckResource{}
}

type lbHealthCheckResource struct {
	client *client.Client
}

type lbHealthCheckModel struct {
	ID                 types.String `tfsdk:"id"`
	LoadBalancerID     types.String `tfsdk:"loadbalancer_id"`
	ListenerID         types.String `tfsdk:"listener_id"`
	Protocol           types.String `tfsdk:"protocol"`
	Path               types.String `tfsdk:"path"`
	Port               types.Int64  `tfsdk:"port"`
	IntervalSeconds    types.Int64  `tfsdk:"interval_seconds"`
	TimeoutSeconds     types.Int64  `tfsdk:"timeout_seconds"`
	HealthyThreshold   types.Int64  `tfsdk:"healthy_threshold"`
	UnhealthyThreshold types.Int64  `tfsdk:"unhealthy_threshold"`
	ExpectedCodes      types.String `tfsdk:"expected_codes"`
	HTTPMethod         types.String `tfsdk:"http_method"`
}

func (r *lbHealthCheckResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_health_check"
}

func (r *lbHealthCheckResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the health check of a load balancer listener (one per listener). Targets that fail " +
			"it stop receiving traffic. Import with loadbalancer_id/listener_id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "loadbalancer_id/listener_id.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"loadbalancer_id": schema.StringAttribute{
				Description:   "UUID of the load balancer.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"listener_id": schema.StringAttribute{
				Description:   "UUID of the listener.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"protocol": schema.StringAttribute{
				Description: "Check protocol: http, https or tcp. Defaults to http.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("http"),
				Validators:  []validator.String{StringOneOf("http", "https", "tcp")},
			},
			"path": schema.StringAttribute{
				Description: "Path requested by http and https checks. Defaults to /.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("/"),
			},
			"port": schema.Int64Attribute{
				Description: "Port to check. Omit it to check the target port.",
				Optional:    true,
				Validators:  []validator.Int64{Int64Between(1, 65535)},
			},
			"interval_seconds": schema.Int64Attribute{
				Description: "Seconds between checks, 5 to 300. Defaults to 30.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(30),
				Validators:  []validator.Int64{Int64Between(5, 300)},
			},
			"timeout_seconds": schema.Int64Attribute{
				Description: "Check timeout, 1 to 60 and lower than interval_seconds. Defaults to 5.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(5),
				Validators:  []validator.Int64{Int64Between(1, 60)},
			},
			"healthy_threshold": schema.Int64Attribute{
				Description: "Consecutive successes to mark a target healthy, 1 to 10. Defaults to 2.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(2),
				Validators:  []validator.Int64{Int64Between(1, 10)},
			},
			"unhealthy_threshold": schema.Int64Attribute{
				Description: "Consecutive failures to mark a target unhealthy, 1 to 10. Defaults to 3.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(3),
				Validators:  []validator.Int64{Int64Between(1, 10)},
			},
			"expected_codes": schema.StringAttribute{
				Description: "HTTP status codes that count as healthy: a code, a range or a comma list (200,301-302). Defaults to 200-399.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("200-399"),
			},
			"http_method": schema.StringAttribute{
				Description: "HTTP method: GET or HEAD. Defaults to GET.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("GET"),
				Validators:  []validator.String{StringOneOf("GET", "HEAD")},
			},
		},
	}
}

func (r *lbHealthCheckResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *lbHealthCheckResource) put(ctx context.Context, m *lbHealthCheckModel) error {
	cfg := &client.HealthCheckConfig{
		Protocol:           m.Protocol.ValueString(),
		Path:               m.Path.ValueString(),
		Port:               knownInt(m.Port),
		IntervalSeconds:    int(m.IntervalSeconds.ValueInt64()),
		TimeoutSeconds:     int(m.TimeoutSeconds.ValueInt64()),
		HealthyThreshold:   int(m.HealthyThreshold.ValueInt64()),
		UnhealthyThreshold: int(m.UnhealthyThreshold.ValueInt64()),
		ExpectedCodes:      m.ExpectedCodes.ValueString(),
		HTTPMethod:         m.HTTPMethod.ValueString(),
	}
	return lbRetry(ctx, func() error {
		return r.client.LoadBalancer.ConfigureHealthCheck(ctx, m.LoadBalancerID.ValueString(), m.ListenerID.ValueString(), cfg)
	})
}

func (r *lbHealthCheckResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lbHealthCheckModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error configuring load balancer health check", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.LoadBalancerID.ValueString() + "/" + plan.ListenerID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *lbHealthCheckResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lbHealthCheckModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hc, err := r.client.LoadBalancer.GetHealthCheck(ctx, state.LoadBalancerID.ValueString(), state.ListenerID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading load balancer health check", err.Error())
		return
	}
	if hc == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.ID = types.StringValue(state.LoadBalancerID.ValueString() + "/" + state.ListenerID.ValueString())
	state.Protocol = types.StringValue(hc.Protocol)
	state.Path = types.StringValue(hc.Path)
	if hc.Port != nil {
		state.Port = types.Int64Value(int64(*hc.Port))
	} else {
		state.Port = types.Int64Null()
	}
	state.IntervalSeconds = types.Int64Value(int64(hc.IntervalSeconds))
	state.TimeoutSeconds = types.Int64Value(int64(hc.TimeoutSeconds))
	state.HealthyThreshold = types.Int64Value(int64(hc.HealthyThreshold))
	state.UnhealthyThreshold = types.Int64Value(int64(hc.UnhealthyThreshold))
	state.ExpectedCodes = types.StringValue(hc.ExpectedCodes)
	if hc.HTTPMethod != "" {
		state.HTTPMethod = types.StringValue(hc.HTTPMethod)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *lbHealthCheckResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan lbHealthCheckModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Error updating load balancer health check", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.LoadBalancerID.ValueString() + "/" + plan.ListenerID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *lbHealthCheckResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lbHealthCheckModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := lbRetry(ctx, func() error {
		return r.client.LoadBalancer.DeleteHealthCheck(ctx, state.LoadBalancerID.ValueString(), state.ListenerID.ValueString())
	})
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting load balancer health check", err.Error())
	}
}

func (r *lbHealthCheckResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	lbID, listenerID, err := splitImportID(req.ID, "loadbalancer_id/listener_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("loadbalancer_id"), lbID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("listener_id"), listenerID)...)
}
