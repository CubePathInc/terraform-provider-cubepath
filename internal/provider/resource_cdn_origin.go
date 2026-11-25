package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                   = &cdnOriginResource{}
	_ resource.ResourceWithConfigure      = &cdnOriginResource{}
	_ resource.ResourceWithImportState    = &cdnOriginResource{}
	_ resource.ResourceWithValidateConfig = &cdnOriginResource{}
	_ resource.ResourceWithModifyPlan     = &cdnOriginResource{}
)

// bucketOriginFixedAttributes are set by the API from the bucket for an origin that serves a
// CubePath Object Storage bucket; the API refuses them next to object_storage_bucket_uuid.
// bucketOriginReadyTimeout bounds the wait for a bucket origin to start serving after create.
const bucketOriginReadyTimeout = 3 * time.Minute

var bucketOriginFixedAttributes = []string{
	"origin_url", "address", "port", "protocol", "health_check_enabled", "health_check_path",
	"health_check_type", "health_check_method", "health_check_expected_status", "health_check_interval",
	"health_check_threshold", "verify_ssl", "host_header", "base_path", "enabled",
}

func NewCDNOriginResource() resource.Resource {
	return &cdnOriginResource{}
}

type cdnOriginResource struct {
	client *client.Client
}

type cdnOriginResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	ZoneUUID             types.String `tfsdk:"zone_uuid"`
	Name                 types.String `tfsdk:"name"`
	OriginURL            types.String `tfsdk:"origin_url"`
	Address              types.String `tfsdk:"address"`
	Port                 types.Int64  `tfsdk:"port"`
	Protocol             types.String `tfsdk:"protocol"`
	Weight               types.Int64  `tfsdk:"weight"`
	Priority             types.Int64  `tfsdk:"priority"`
	IsBackup             types.Bool   `tfsdk:"is_backup"`
	HealthCheckEnabled   types.Bool   `tfsdk:"health_check_enabled"`
	HealthCheckPath      types.String `tfsdk:"health_check_path"`
	HealthCheckType      types.String `tfsdk:"health_check_type"`
	HealthCheckMethod    types.String `tfsdk:"health_check_method"`
	HealthCheckExpect    types.String `tfsdk:"health_check_expected_status"`
	HealthCheckInterval  types.Int64  `tfsdk:"health_check_interval"`
	HealthCheckThreshold types.Int64  `tfsdk:"health_check_threshold"`
	VerifySSL            types.Bool   `tfsdk:"verify_ssl"`
	HostHeader           types.String `tfsdk:"host_header"`
	BasePath             types.String `tfsdk:"base_path"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	HealthStatus         types.String `tfsdk:"health_status"`
	HealthDetail         types.String `tfsdk:"health_detail"`
	HealthChangedAt      types.String `tfsdk:"health_changed_at"`
	BucketUUID           types.String `tfsdk:"object_storage_bucket_uuid"`
}

func (r *cdnOriginResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cdn_origin"
}

func (r *cdnOriginResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CDN origin server on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the origin.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone_uuid": schema.StringAttribute{
				Description:   "The UUID of the CDN zone.",
				Required:      true,
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
				Description: "Path of an http health check (up to 255 characters, starting with one /). A tcp check does not use it. When you do not set it the API chooses: / for an http check, /health for a tcp one.",
				Optional:    true,
				Computed:    true,
			},
			"health_check_type": schema.StringAttribute{
				Description: "Health check type: tcp (default, the edge only opens a connection) or http " +
					"(the edge requests health_check_path and takes the origin out when the answer is not one " +
					"of health_check_expected_status; if it is the only origin of the zone, the zone answers 503). " +
					"All the origins of a zone share one check: the one of the first origin by priority that has " +
					"health_check_enabled. http needs health_check_enabled to be true.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("tcp"),
			},
			"health_check_method": schema.StringAttribute{
				Description: "Method of an http health check: GET (default) or HEAD.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("GET"),
			},
			"health_check_expected_status": schema.StringAttribute{
				Description: "Statuses (200) or classes (2xx) that an http health check accepts, up to 8 separated by spaces. Defaults to \"2xx 3xx\".",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("2xx 3xx"),
			},
			"health_check_interval": schema.Int64Attribute{
				Description: "Seconds between health checks from each edge node (5-300). Defaults to 10.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(10),
			},
			"health_check_threshold": schema.Int64Attribute{
				Description: "Results in a row that change the health status (1-10). Defaults to 3.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(3),
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
				Description: "Current health status: healthy, unhealthy or unknown. It is unknown while no edge node has checked the origin, and for ever when health_check_enabled is false (not checked).",
				Computed:    true,
			},
			"health_detail": schema.StringAttribute{
				Description: "Why the origin is down (the error of the last check), empty while it is not unhealthy.",
				Computed:    true,
			},
			"health_changed_at": schema.StringAttribute{
				Description: "When the health status last changed (ISO 8601, UTC), empty until it is known.",
				Computed:    true,
			},
			"object_storage_bucket_uuid": schema.StringAttribute{
				Description: "UUID of a CubePath Object Storage bucket to serve through this CDN zone. " +
					"The address, port, protocol, host header, health check and read only credentials are set " +
					"from the bucket, so only name, weight, priority and is_backup may be set next to it. " +
					"A bucket can be served by one origin at a time. Deleting the origin stops serving the bucket. " +
					"Requires the object_storage:write scope. Changing it forces a new origin.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
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

// ValidateConfig refuses connection attributes next to object_storage_bucket_uuid.
func (r *cdnOriginResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg cdnOriginResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateOriginHealthCheck(&cfg, &resp.Diagnostics)
	bucket := cfg.BucketUUID
	if bucket.IsNull() {
		return
	}
	for _, name := range bucketOriginFixedAttributes {
		var v attr.Value
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &v)...)
		if v != nil && !v.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Attribute not allowed with object_storage_bucket_uuid",
				fmt.Sprintf("%q is set from the bucket for an Object Storage origin. Only name, weight, priority and is_backup can be set.", name))
		}
	}
}

// validateOriginHealthCheck checks the values that are known at plan time: the type and method
// enums and the integer ranges. Everything else is left to the API.
func validateOriginHealthCheck(cfg *cdnOriginResourceModel, diags *diag.Diagnostics) {
	oneOf := func(name string, v types.String, allowed ...string) {
		if v.IsNull() || v.IsUnknown() {
			return
		}
		for _, a := range allowed {
			if v.ValueString() == a {
				return
			}
		}
		diags.AddAttributeError(path.Root(name), "Invalid value",
			fmt.Sprintf("%s must be one of %s, got %q.", name, strings.Join(allowed, ", "), v.ValueString()))
	}
	inRange := func(name string, v types.Int64, lo, hi int64) {
		if v.IsNull() || v.IsUnknown() {
			return
		}
		if n := v.ValueInt64(); n < lo || n > hi {
			diags.AddAttributeError(path.Root(name), "Invalid value",
				fmt.Sprintf("%s must be between %d and %d, got %d.", name, lo, hi, n))
		}
	}
	oneOf("health_check_type", cfg.HealthCheckType, "tcp", "http")
	oneOf("health_check_method", cfg.HealthCheckMethod, "GET", "HEAD")
	inRange("health_check_interval", cfg.HealthCheckInterval, 5, 300)
	inRange("health_check_threshold", cfg.HealthCheckThreshold, 1, 10)
	if !cfg.HealthCheckType.IsNull() && !cfg.HealthCheckType.IsUnknown() && cfg.HealthCheckType.ValueString() == "http" &&
		!cfg.HealthCheckEnabled.IsNull() && !cfg.HealthCheckEnabled.IsUnknown() && !cfg.HealthCheckEnabled.ValueBool() {
		diags.AddAttributeError(path.Root("health_check_type"), "Invalid combination",
			"health_check_type http needs health_check_enabled to be true.")
	}
}

// ModifyPlan keeps the bucket-managed attributes of an Object Storage origin as the API reports
// them, instead of the defaults meant for external origins, so they never show as a diff.
func (r *cdnOriginResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var bucket types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("object_storage_bucket_uuid"), &bucket)...)
	if resp.Diagnostics.HasError() || bucket.IsNull() {
		return
	}
	creating := req.State.Raw.IsNull()
	for _, name := range bucketOriginFixedAttributes {
		p := path.Root(name)
		if name == "origin_url" {
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, types.StringNull())...)
			continue
		}
		if creating {
			switch name {
			case "port", "health_check_interval", "health_check_threshold":
				resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, types.Int64Unknown())...)
			case "health_check_enabled", "verify_ssl", "enabled":
				resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, types.BoolUnknown())...)
			default:
				resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, types.StringUnknown())...)
			}
			continue
		}
		switch name {
		case "port", "health_check_interval", "health_check_threshold":
			var v types.Int64
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, p, &v)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, v)...)
		case "health_check_enabled", "verify_ssl", "enabled":
			var v types.Bool
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, p, &v)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, v)...)
		default:
			var v types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, p, &v)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, p, v)...)
		}
	}
	if !creating {
		// Serving starts once the bucket's read only access is ready; the API flips it itself.
		var status types.String
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("health_status"), &status)...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("health_status"), status)...)
		for _, name := range []string{"health_detail", "health_changed_at"} {
			var v types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(name), &v)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(name), v)...)
		}
	}
}

func (r *cdnOriginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cdnOriginResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !plan.BucketUUID.IsNull() && !plan.BucketUUID.IsUnknown() {
		r.createBucketOrigin(ctx, &plan, resp)
		return
	}

	createReq := &client.CreateCDNOriginRequest{
		Name:               plan.Name.ValueString(),
		Weight:             int(plan.Weight.ValueInt64()),
		Priority:           int(plan.Priority.ValueInt64()),
		IsBackup:           plan.IsBackup.ValueBool(),
		HealthCheckEnabled: plan.HealthCheckEnabled.ValueBool(),
		VerifySSL:          plan.VerifySSL.ValueBool(),
		Enabled:            plan.Enabled.ValueBool(),
	}
	if !plan.HealthCheckType.IsNull() && !plan.HealthCheckType.IsUnknown() {
		createReq.HealthCheckType = plan.HealthCheckType.ValueString()
	}
	if !plan.HealthCheckMethod.IsNull() && !plan.HealthCheckMethod.IsUnknown() {
		createReq.HealthCheckMethod = plan.HealthCheckMethod.ValueString()
	}
	if !plan.HealthCheckExpect.IsNull() && !plan.HealthCheckExpect.IsUnknown() {
		createReq.HealthCheckExpectedStatus = plan.HealthCheckExpect.ValueString()
	}
	if !plan.HealthCheckPath.IsNull() && !plan.HealthCheckPath.IsUnknown() {
		createReq.HealthCheckPath = plan.HealthCheckPath.ValueString()
	}
	if !plan.HealthCheckInterval.IsNull() && !plan.HealthCheckInterval.IsUnknown() {
		v := int(plan.HealthCheckInterval.ValueInt64())
		createReq.HealthCheckInterval = &v
	}
	if !plan.HealthCheckThreshold.IsNull() && !plan.HealthCheckThreshold.IsUnknown() {
		v := int(plan.HealthCheckThreshold.ValueInt64())
		createReq.HealthCheckThreshold = &v
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

// createBucketOrigin creates an origin that serves an Object Storage bucket and reads it back,
// since the create response only carries a few fields.
func (r *cdnOriginResource) createBucketOrigin(ctx context.Context, plan *cdnOriginResourceModel, resp *resource.CreateResponse) {
	createReq := &client.CreateCDNBucketOriginRequest{
		Name:                    plan.Name.ValueString(),
		ObjectStorageBucketUUID: plan.BucketUUID.ValueString(),
		Weight:                  int(plan.Weight.ValueInt64()),
		Priority:                int(plan.Priority.ValueInt64()),
		IsBackup:                plan.IsBackup.ValueBool(),
	}
	// When the origin is being replaced, the previous one releases the bucket a few seconds
	// after its delete: the API answers 409 until then.
	var created *client.CDNOrigin
	var err error
	retryUntil := time.Now().Add(bucketOriginReadyTimeout)
	for {
		created, err = r.client.CDN.CreateBucketOrigin(ctx, plan.ZoneUUID.ValueString(), createReq)
		var apiErr *client.APIError
		if err == nil || !errors.As(err, &apiErr) || !apiErr.IsConflict() || time.Now().After(retryUntil) {
			break
		}
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Error creating CDN origin for the bucket", ctx.Err().Error())
			return
		case <-time.After(5 * time.Second):
		}
	}
	if err != nil {
		resp.Diagnostics.AddError("Error creating CDN origin for the bucket", err.Error())
		return
	}

	// The origin starts serving once the bucket's read only access is ready (usually seconds).
	var origin *client.CDNOrigin
	deadline := time.Now().Add(bucketOriginReadyTimeout)
	for {
		origin, err = r.findOrigin(ctx, plan.ZoneUUID.ValueString(), created.UUID)
		if err != nil {
			resp.Diagnostics.AddError("Error reading the new CDN origin", err.Error())
			return
		}
		if origin == nil || origin.Enabled || time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			resp.Diagnostics.AddError("Error waiting for the CDN origin", ctx.Err().Error())
			return
		case <-time.After(5 * time.Second):
		}
	}
	if origin == nil {
		origin = created
	}
	if !origin.Enabled {
		resp.Diagnostics.AddWarning("CDN origin not serving yet",
			"The origin was created but the bucket's access is not ready yet. It starts serving on its own; run terraform refresh later.")
	}
	r.mapToState(plan, origin)
	plan.OriginURL = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// findOrigin returns the origin of the zone with that UUID, or nil when it does not exist.
func (r *cdnOriginResource) findOrigin(ctx context.Context, zoneUUID, originUUID string) (*client.CDNOrigin, error) {
	origins, err := r.client.CDN.ListOrigins(ctx, zoneUUID)
	if err != nil {
		return nil, err
	}
	for i := range origins {
		if origins[i].UUID == originUUID {
			return &origins[i], nil
		}
	}
	return nil, nil
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
	bucketOrigin := !state.BucketUUID.IsNull()
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Weight.Equal(state.Weight) {
		v := int(plan.Weight.ValueInt64())
		updateReq.Weight = &v
	}
	if !plan.Priority.Equal(state.Priority) {
		v := int(plan.Priority.ValueInt64())
		updateReq.Priority = &v
	}
	if !plan.IsBackup.Equal(state.IsBackup) {
		v := plan.IsBackup.ValueBool()
		updateReq.IsBackup = &v
	}
	// Only name, weight, priority and is_backup can change on a bucket origin.
	if !bucketOrigin {
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
		if !plan.HostHeader.Equal(state.HostHeader) {
			v := plan.HostHeader.ValueString()
			updateReq.HostHeader = &v
		}
		if !plan.BasePath.Equal(state.BasePath) {
			v := plan.BasePath.ValueString()
			updateReq.BasePath = &v
		}
		if !plan.HealthCheckEnabled.Equal(state.HealthCheckEnabled) {
			v := plan.HealthCheckEnabled.ValueBool()
			updateReq.HealthCheckEnabled = &v
		}
		if !plan.HealthCheckPath.IsNull() && !plan.HealthCheckPath.IsUnknown() && !plan.HealthCheckPath.Equal(state.HealthCheckPath) {
			v := plan.HealthCheckPath.ValueString()
			updateReq.HealthCheckPath = &v
		}
		if !plan.HealthCheckType.Equal(state.HealthCheckType) {
			v := plan.HealthCheckType.ValueString()
			updateReq.HealthCheckType = &v
		}
		if !plan.HealthCheckMethod.Equal(state.HealthCheckMethod) {
			v := plan.HealthCheckMethod.ValueString()
			updateReq.HealthCheckMethod = &v
		}
		if !plan.HealthCheckExpect.Equal(state.HealthCheckExpect) {
			v := plan.HealthCheckExpect.ValueString()
			updateReq.HealthCheckExpectedStatus = &v
		}
		if !plan.HealthCheckInterval.Equal(state.HealthCheckInterval) {
			v := int(plan.HealthCheckInterval.ValueInt64())
			updateReq.HealthCheckInterval = &v
		}
		if !plan.HealthCheckThreshold.Equal(state.HealthCheckThreshold) {
			v := int(plan.HealthCheckThreshold.ValueInt64())
			updateReq.HealthCheckThreshold = &v
		}
	}

	err := r.client.CDN.UpdateOrigin(ctx, state.ZoneUUID.ValueString(), state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating CDN origin", err.Error())
		return
	}

	plan.ID = state.ID
	plan.ZoneUUID = state.ZoneUUID
	plan.HealthStatus = state.HealthStatus
	plan.HealthDetail = state.HealthDetail
	plan.HealthChangedAt = state.HealthChangedAt
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
	state.HealthCheckType = types.StringValue(origin.HealthCheckType)
	state.HealthCheckMethod = types.StringValue(origin.HealthCheckMethod)
	state.HealthCheckExpect = types.StringValue(origin.HealthCheckExpectedStatus)
	state.HealthCheckInterval = types.Int64Value(int64(origin.HealthCheckInterval))
	state.HealthCheckThreshold = types.Int64Value(int64(origin.HealthCheckThreshold))
	state.HealthDetail = types.StringValue(derefString(origin.HealthDetail))
	state.HealthChangedAt = types.StringValue(derefString(origin.HealthChangedAt))
	if origin.ObjectStorageBucketUUID != nil && *origin.ObjectStorageBucketUUID != "" {
		state.BucketUUID = types.StringValue(*origin.ObjectStorageBucketUUID)
	} else {
		state.BucketUUID = types.StringNull()
	}
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
