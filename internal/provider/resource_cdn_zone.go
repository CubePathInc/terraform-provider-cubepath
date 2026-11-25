package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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

	TokenAuthEnabled   types.Bool   `tfsdk:"token_auth_enabled"`
	TokenAuthIPBinding types.Bool   `tfsdk:"token_auth_ip_binding"`
	TokenAuthSecret    types.String `tfsdk:"token_auth_secret"`
	CORSEnabled        types.Bool   `tfsdk:"cors_enabled"`
	CORSAllowOrigins   types.String `tfsdk:"cors_allow_origins"`
	CacheTagHeader     types.String `tfsdk:"cache_tag_header"`
	CompressionEnabled types.Bool   `tfsdk:"compression_enabled"`
	ManagedWAF         types.Object `tfsdk:"managed_waf"`
}

// managedWAFModel is the managed_waf attribute: detection only, nothing is blocked.
type managedWAFModel struct {
	Mode              types.String `tfsdk:"mode"`
	Level             types.Int64  `tfsdk:"level"`
	ExcludeRuleIDs    types.Set    `tfsdk:"exclude_rule_ids"`
	ExcludeCategories types.Set    `tfsdk:"exclude_categories"`
}

var managedWAFAttrTypes = map[string]attr.Type{
	"mode":               types.StringType,
	"level":              types.Int64Type,
	"exclude_rule_ids":   types.SetType{ElemType: types.Int64Type},
	"exclude_categories": types.SetType{ElemType: types.StringType},
}

// managedWAFFromAPI maps what the API reports; a nil value (an API without the feature) is a null object.
func managedWAFFromAPI(ctx context.Context, w *client.CDNManagedWAF) (types.Object, diag.Diagnostics) {
	if w == nil {
		return types.ObjectNull(managedWAFAttrTypes), nil
	}
	ids := make([]attr.Value, 0, len(w.ExcludeRuleIDs))
	for _, id := range w.ExcludeRuleIDs {
		ids = append(ids, types.Int64Value(id))
	}
	cats := make([]attr.Value, 0, len(w.ExcludeCategories))
	for _, c := range w.ExcludeCategories {
		cats = append(cats, types.StringValue(c))
	}
	idSet, d1 := types.SetValue(types.Int64Type, ids)
	catSet, d2 := types.SetValue(types.StringType, cats)
	diags := append(d1, d2...)
	if diags.HasError() {
		return types.ObjectNull(managedWAFAttrTypes), diags
	}
	obj, d3 := types.ObjectValue(managedWAFAttrTypes, map[string]attr.Value{
		"mode":               types.StringValue(w.Mode),
		"level":              types.Int64Value(int64(w.Level)),
		"exclude_rule_ids":   idSet,
		"exclude_categories": catSet,
	})
	return obj, append(diags, d3...)
}

// managedWAFRequest builds the update from the plan; nil when the attribute is not configured. A part left out
// of the configuration takes the API default (mode off, level 1, empty lists).
func managedWAFRequest(ctx context.Context, o types.Object) (*client.UpdateCDNManagedWAF, diag.Diagnostics) {
	if o.IsNull() || o.IsUnknown() {
		return nil, nil
	}
	var m managedWAFModel
	diags := o.As(ctx, &m, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	req := &client.UpdateCDNManagedWAF{Mode: "off", Level: 1, ExcludeRuleIDs: []int64{}, ExcludeCategories: []string{}}
	if !m.Mode.IsNull() && !m.Mode.IsUnknown() {
		req.Mode = m.Mode.ValueString()
	}
	if !m.Level.IsNull() && !m.Level.IsUnknown() {
		req.Level = int(m.Level.ValueInt64())
	}
	if !m.ExcludeRuleIDs.IsNull() && !m.ExcludeRuleIDs.IsUnknown() {
		diags.Append(m.ExcludeRuleIDs.ElementsAs(ctx, &req.ExcludeRuleIDs, false)...)
		sort.Slice(req.ExcludeRuleIDs, func(i, j int) bool { return req.ExcludeRuleIDs[i] < req.ExcludeRuleIDs[j] })
	}
	if !m.ExcludeCategories.IsNull() && !m.ExcludeCategories.IsUnknown() {
		diags.Append(m.ExcludeCategories.ElementsAs(ctx, &req.ExcludeCategories, false)...)
		sort.Strings(req.ExcludeCategories)
	}
	if req.Mode != "off" && req.Mode != "log" {
		diags.AddAttributeError(path.Root("managed_waf").AtName("mode"), "Invalid managed_waf mode", "mode must be off or log.")
	}
	if req.Level != 1 && req.Level != 2 {
		diags.AddAttributeError(path.Root("managed_waf").AtName("level"), "Invalid managed_waf level", "level must be 1 or 2.")
	}
	if len(req.ExcludeRuleIDs) > 256 {
		diags.AddAttributeError(path.Root("managed_waf").AtName("exclude_rule_ids"), "Too many rule ids", "At most 256 rule ids.")
	}
	for _, id := range req.ExcludeRuleIDs {
		if id < 1 || id > 4294967295 {
			diags.AddAttributeError(path.Root("managed_waf").AtName("exclude_rule_ids"), "Invalid rule id", fmt.Sprintf("%d is not between 1 and 4294967295.", id))
		}
	}
	if len(req.ExcludeCategories) > 64 {
		diags.AddAttributeError(path.Root("managed_waf").AtName("exclude_categories"), "Too many categories", "At most 64 categories.")
	}
	for _, c := range req.ExcludeCategories {
		if !managedWAFCategory.MatchString(c) {
			diags.AddAttributeError(path.Root("managed_waf").AtName("exclude_categories"), "Invalid category", fmt.Sprintf("%q must be 1 to 32 characters of a-z, 0-9, - or _.", c))
		}
	}
	return req, diags
}

var managedWAFCategory = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

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
				Description: "The project ID. Changing it moves the zone in place.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"token_auth_enabled": schema.BoolAttribute{
				Description: "Token Auth: only serve requests with a valid signed URL. The first time it is enabled the " +
					"zone gets a signing secret (token_auth_secret). If omitted, the current setting is kept.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"token_auth_ip_binding": schema.BoolAttribute{
				Description: "Bind signed URLs to the client IP. If omitted, the current setting is kept.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"token_auth_secret": schema.StringAttribute{
				Description: "Secret used to sign URLs when Token Auth is enabled. Needs an API token with cdn:write.",
				Computed:    true,
				Sensitive:   true,
			},
			"cors_enabled": schema.BoolAttribute{
				Description: "Add CORS headers to responses. If omitted, the current setting is kept.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"cors_allow_origins": schema.StringAttribute{
				Description: "Allowed origins: \"*\" or a comma separated list of up to 50 origins (https://example.com).",
				Optional:    true,
			},
			"cache_tag_header": schema.StringAttribute{
				Description: "Name of the origin response header whose values are the cache tags of an object, so the " +
					"cache can be purged by tag (letters, digits, _ and -, up to 64 characters). An empty string turns tag " +
					"purges off. Files cached before it was set have no tags. If omitted, the current setting is kept.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"compression_enabled": schema.BoolAttribute{
				Description: "Compress text responses at the edge (Brotli, Zstandard or gzip, as the browser accepts). " +
					"Off by default: with it off the edge compresses nothing, gzip included, and responses the origin already compresses pass through unchanged. If omitted, the current setting is kept.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"managed_waf": schema.SingleNestedAttribute{
				Description: "Managed WAF in detection mode: a rule set CubePath runs for the zone that logs what it finds " +
					"and blocks nothing. It does not inspect the request body and does not evaluate responses served " +
					"from cache. Not the same as cubepath_cdn_waf_rule, which blocks traffic you choose. If omitted, " +
					"the current setting is kept.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"mode": schema.StringAttribute{
						Description: "off or log. Default off.",
						Optional:    true,
						Computed:    true,
					},
					"level": schema.Int64Attribute{
						Description: "Sensitivity level, 1 (fewest false positives) or 2. Default 1.",
						Optional:    true,
						Computed:    true,
					},
					"exclude_rule_ids": schema.SetAttribute{
						Description: "Rule ids the managed WAF ignores (up to 256, each from 1 to 4294967295).",
						ElementType: types.Int64Type,
						Optional:    true,
						Computed:    true,
					},
					"exclude_categories": schema.SetAttribute{
						Description: "Rule categories the managed WAF ignores (up to 64, lowercase letters, digits, - and _, 1 to 32 characters).",
						ElementType: types.StringType,
						Optional:    true,
						Computed:    true,
					},
				},
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

	// Token Auth and CORS are not part of the create request.
	settings := &client.UpdateCDNZoneRequest{
		TokenAuthEnabled:   knownBool(plan.TokenAuthEnabled),
		TokenAuthIPBinding: knownBool(plan.TokenAuthIPBinding),
		CORSEnabled:        knownBool(plan.CORSEnabled),
		CORSAllowOrigins:   knownString(plan.CORSAllowOrigins),
		CacheTagHeader:     knownString(plan.CacheTagHeader),
		CompressionEnabled: knownBool(plan.CompressionEnabled),
	}
	waf, d := managedWAFRequest(ctx, plan.ManagedWAF)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	settings.ManagedWAF = waf
	if settings.TokenAuthEnabled != nil || settings.TokenAuthIPBinding != nil || settings.CORSEnabled != nil || settings.CORSAllowOrigins != nil ||
		settings.CacheTagHeader != nil || settings.CompressionEnabled != nil || settings.ManagedWAF != nil {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), zone.UUID)...)
		if err := r.client.CDN.UpdateZone(ctx, zone.UUID, settings); err != nil {
			resp.Diagnostics.AddError("Error configuring CDN zone Token Auth and CORS", err.Error())
			return
		}
		project := zone.ProjectID
		if zone, err = r.client.CDN.GetZone(ctx, zone.UUID); err != nil {
			resp.Diagnostics.AddError("Error reading CDN zone", err.Error())
			return
		}
		if zone.ProjectID == 0 {
			zone.ProjectID = project
		}
	}

	resp.Diagnostics.Append(r.mapToState(ctx, &plan, zone)...)
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
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading CDN zone", err.Error())
		return
	}

	resp.Diagnostics.Append(r.mapToState(ctx, &state, zone)...)
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
	if !plan.TokenAuthEnabled.IsUnknown() && !plan.TokenAuthEnabled.Equal(state.TokenAuthEnabled) {
		updateReq.TokenAuthEnabled = knownBool(plan.TokenAuthEnabled)
		changed = true
	}
	if !plan.TokenAuthIPBinding.IsUnknown() && !plan.TokenAuthIPBinding.Equal(state.TokenAuthIPBinding) {
		updateReq.TokenAuthIPBinding = knownBool(plan.TokenAuthIPBinding)
		changed = true
	}
	if !plan.CORSEnabled.IsUnknown() && !plan.CORSEnabled.Equal(state.CORSEnabled) {
		updateReq.CORSEnabled = knownBool(plan.CORSEnabled)
		changed = true
	}
	if !plan.CORSAllowOrigins.Equal(state.CORSAllowOrigins) {
		v := plan.CORSAllowOrigins.ValueString() // "" clears the list
		updateReq.CORSAllowOrigins = &v
		changed = true
	}

	if !plan.CompressionEnabled.IsUnknown() && !plan.CompressionEnabled.Equal(state.CompressionEnabled) {
		updateReq.CompressionEnabled = knownBool(plan.CompressionEnabled)
		changed = true
	}
	if !plan.CacheTagHeader.IsUnknown() && !plan.CacheTagHeader.Equal(state.CacheTagHeader) {
		v := plan.CacheTagHeader.ValueString() // "" clears it
		updateReq.CacheTagHeader = &v
		changed = true
	}
	if !plan.ManagedWAF.IsUnknown() && !plan.ManagedWAF.Equal(state.ManagedWAF) {
		waf, d := managedWAFRequest(ctx, plan.ManagedWAF)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		if waf != nil {
			updateReq.ManagedWAF = waf
			changed = true
		}
	}

	if !plan.ProjectID.IsUnknown() && !plan.ProjectID.Equal(state.ProjectID) {
		if err := r.client.CDN.MoveZone(ctx, state.ID.ValueString(), int(plan.ProjectID.ValueInt64())); err != nil {
			resp.Diagnostics.AddError("Error moving CDN zone", err.Error())
			return
		}
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

	resp.Diagnostics.Append(r.mapToState(ctx, &plan, zone)...)
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

func (r *cdnZoneResource) mapToState(ctx context.Context, state *cdnZoneResourceModel, zone *client.CDNZone) diag.Diagnostics {
	state.ID = types.StringValue(zone.UUID)
	state.Name = types.StringValue(zone.Name)
	state.Domain = types.StringValue(zone.Domain)
	state.CustomDomain = types.StringValue(zone.CustomDomain)
	// The create response carries no project_id and the detail no plan_name: keep the known
	// value instead of overwriting it with an empty one (inconsistent result / perpetual diff).
	if zone.PlanName != "" || state.PlanName.IsNull() || state.PlanName.IsUnknown() {
		state.PlanName = types.StringValue(zone.PlanName)
	}
	state.SSLType = types.StringValue(zone.SSLType)
	if zone.ProjectID != 0 || state.ProjectID.IsNull() || state.ProjectID.IsUnknown() {
		state.ProjectID = types.Int64Value(int64(zone.ProjectID))
	}
	state.Status = types.StringValue(zone.Status)
	state.CreatedAt = types.StringValue(zone.CreatedAt)
	state.UpdatedAt = types.StringValue(zone.UpdatedAt)
	state.TokenAuthEnabled = types.BoolValue(zone.TokenAuthEnabled)
	state.TokenAuthIPBinding = types.BoolValue(zone.TokenAuthIPBinding)
	// The create response carries no secret: keep the one already known.
	if zone.TokenAuthSecret != nil || state.TokenAuthSecret.IsUnknown() {
		state.TokenAuthSecret = stringOrNull(zone.TokenAuthSecret)
	}
	state.CORSEnabled = types.BoolValue(zone.CORSEnabled)
	state.CORSAllowOrigins = stringOrNull(zone.CORSAllowOrigins)
	state.CacheTagHeader = types.StringValue("")
	if zone.CacheTagHeader != nil {
		state.CacheTagHeader = types.StringValue(*zone.CacheTagHeader)
	}
	state.CompressionEnabled = types.BoolValue(zone.CompressionEnabled)
	waf, diags := managedWAFFromAPI(ctx, zone.ManagedWAF)
	state.ManagedWAF = waf
	return diags
}
