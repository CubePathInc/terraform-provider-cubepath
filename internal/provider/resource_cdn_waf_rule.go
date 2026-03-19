package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &cdnWAFRuleResource{}
	_ resource.ResourceWithConfigure   = &cdnWAFRuleResource{}
	_ resource.ResourceWithImportState = &cdnWAFRuleResource{}
)

func NewCDNWAFRuleResource() resource.Resource {
	return &cdnWAFRuleResource{}
}

type cdnWAFRuleResource struct {
	client *client.Client
}

type cdnWAFRuleResourceModel struct {
	ID              types.String `tfsdk:"id"`
	ZoneUUID        types.String `tfsdk:"zone_uuid"`
	Name            types.String `tfsdk:"name"`
	RuleType        types.String `tfsdk:"rule_type"`
	Priority        types.Int64  `tfsdk:"priority"`
	MatchConditions types.String `tfsdk:"match_conditions"`
	ActionConfig    types.String `tfsdk:"action_config"`
	Enabled         types.Bool   `tfsdk:"enabled"`
}

func (r *cdnWAFRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cdn_waf_rule"
}

func (r *cdnWAFRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CDN WAF rule on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The UUID of the WAF rule.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone_uuid": schema.StringAttribute{
				Description: "The UUID of the CDN zone.",
				Required:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Rule name.",
				Required:    true,
			},
			"rule_type": schema.StringAttribute{
				Description: "WAF rule type: firewall_ip, firewall_country, firewall_ua, rate_limit, js_challenge, limit_download_speed, limit_requests, limit_connections, limit_bandwidth.",
				Required:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"priority": schema.Int64Attribute{
				Description: "Rule priority (1-10000).",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(100),
			},
			"match_conditions": schema.StringAttribute{
				Description: "Match conditions as JSON string.",
				Optional:    true,
			},
			"action_config": schema.StringAttribute{
				Description: "Action configuration as JSON string.",
				Required:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the rule is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
		},
	}
}

func (r *cdnWAFRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *cdnWAFRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan cdnWAFRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	actionConfig := json.RawMessage(plan.ActionConfig.ValueString())
	createReq := &client.CreateCDNRuleRequest{
		Name:         plan.Name.ValueString(),
		RuleType:     plan.RuleType.ValueString(),
		Priority:     int(plan.Priority.ValueInt64()),
		ActionConfig: actionConfig,
		Enabled:      plan.Enabled.ValueBool(),
	}

	if !plan.MatchConditions.IsNull() && !plan.MatchConditions.IsUnknown() {
		mc := json.RawMessage(plan.MatchConditions.ValueString())
		createReq.MatchConditions = mc
	}

	rule, err := r.client.CDN.CreateWAFRule(ctx, plan.ZoneUUID.ValueString(), createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating CDN WAF rule", err.Error())
		return
	}

	plan.ID = types.StringValue(rule.UUID)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cdnWAFRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state cdnWAFRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.CDN.GetWAFRule(ctx, state.ZoneUUID.ValueString(), state.ID.ValueString())
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading CDN WAF rule", err.Error())
		return
	}

	state.Name = types.StringValue(rule.Name)
	state.RuleType = types.StringValue(rule.RuleType)
	state.Priority = types.Int64Value(int64(rule.Priority))
	state.Enabled = types.BoolValue(rule.Enabled)

	if rule.ActionConfig != nil {
		state.ActionConfig = types.StringValue(string(rule.ActionConfig))
	}
	if rule.MatchConditions != nil && string(rule.MatchConditions) != "null" {
		state.MatchConditions = types.StringValue(string(rule.MatchConditions))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *cdnWAFRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state cdnWAFRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateCDNRuleRequest{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		updateReq.Name = &v
	}
	if !plan.Priority.Equal(state.Priority) {
		v := int(plan.Priority.ValueInt64())
		updateReq.Priority = &v
	}
	if !plan.ActionConfig.Equal(state.ActionConfig) {
		ac := json.RawMessage(plan.ActionConfig.ValueString())
		updateReq.ActionConfig = &ac
	}
	if !plan.MatchConditions.Equal(state.MatchConditions) {
		if !plan.MatchConditions.IsNull() {
			mc := json.RawMessage(plan.MatchConditions.ValueString())
			updateReq.MatchConditions = &mc
		}
	}
	if !plan.Enabled.Equal(state.Enabled) {
		v := plan.Enabled.ValueBool()
		updateReq.Enabled = &v
	}

	err := r.client.CDN.UpdateWAFRule(ctx, state.ZoneUUID.ValueString(), state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating CDN WAF rule", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *cdnWAFRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state cdnWAFRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.CDN.DeleteWAFRule(ctx, state.ZoneUUID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting CDN WAF rule", err.Error())
		return
	}
}

func (r *cdnWAFRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: zone_uuid/rule_uuid")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_uuid"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
