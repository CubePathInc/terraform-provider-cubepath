package provider

import (
	"context"
	"sort"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                   = &alertChannelResource{}
	_ resource.ResourceWithConfigure      = &alertChannelResource{}
	_ resource.ResourceWithImportState    = &alertChannelResource{}
	_ resource.ResourceWithValidateConfig = &alertChannelResource{}
	_ resource.Resource                   = &alertRuleResource{}
	_ resource.ResourceWithConfigure      = &alertRuleResource{}
	_ resource.ResourceWithImportState    = &alertRuleResource{}
)

// ---- cubepath_alert_channel ----

func NewAlertChannelResource() resource.Resource {
	return &alertChannelResource{}
}

type alertChannelResource struct {
	client *client.Client
}

type alertChannelModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	WebhookURL types.String `tfsdk:"webhook_url"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Email      types.String `tfsdk:"email"`
}

func (r *alertChannelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_channel"
}

func (r *alertChannelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloud Alerts notification channel: a Slack or Discord webhook, or the email of the " +
			"account that creates it. Alert rules send their notifications to channels. The API never returns " +
			"a webhook URL after it is set, so changes made outside Terraform to it are not detected. " +
			"Import with the channel ID (webhook_url imports empty).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The ID of the channel.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Name, unique in the organization: up to 64 letters, numbers, spaces and hyphens.",
				Required:    true,
			},
			"type": schema.StringAttribute{
				Description:   "Channel type: slack, discord or email. Changing it forces a new channel.",
				Required:      true,
				Validators:    []validator.String{StringOneOf("slack", "discord", "email")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"webhook_url": schema.StringAttribute{
				Description: "Incoming webhook URL, required for slack (https://hooks.slack.com/services/...) and " +
					"discord (https://discord.com/api/webhooks/...). Must not be set for email.",
				Optional:  true,
				Sensitive: true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the channel is enabled. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"email": schema.StringAttribute{
				Description:   "For email channels, the address notifications go to (the creator's account email).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *alertChannelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *alertChannelResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var cfg alertChannelModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Type.IsUnknown() || cfg.WebhookURL.IsUnknown() {
		return
	}
	switch cfg.Type.ValueString() {
	case "email":
		if !cfg.WebhookURL.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("webhook_url"), "webhook_url not allowed",
				"Email channels send to the account email; remove webhook_url.")
		}
	case "slack", "discord":
		if cfg.WebhookURL.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("webhook_url"), "Missing webhook_url",
				"Slack and Discord channels need a webhook_url.")
		}
	}
}

func (r *alertChannelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enabled := plan.Enabled.ValueBool()
	createReq := &client.CreateAlertChannelRequest{
		Name:    plan.Name.ValueString(),
		Type:    plan.Type.ValueString(),
		Enabled: &enabled,
	}
	if url := knownString(plan.WebhookURL); url != nil {
		createReq.Config = map[string]string{"webhook_url": *url}
	}

	ch, err := r.client.Alerts.CreateChannel(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert channel", err.Error())
		return
	}
	mapAlertChannel(&plan, ch)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertChannelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ch, err := r.client.Alerts.GetChannel(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert channel", err.Error())
		return
	}
	mapAlertChannel(&state, ch)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertChannelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertChannelModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateAlertChannelRequest{}
	if !plan.Name.Equal(state.Name) {
		updateReq.Name = knownString(plan.Name)
	}
	if !plan.Enabled.Equal(state.Enabled) {
		updateReq.Enabled = knownBool(plan.Enabled)
	}
	if url := knownString(plan.WebhookURL); url != nil && !plan.WebhookURL.Equal(state.WebhookURL) {
		updateReq.Config = map[string]string{"webhook_url": *url}
	}

	ch, err := r.client.Alerts.UpdateChannel(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert channel", err.Error())
		return
	}
	mapAlertChannel(&plan, ch)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertChannelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertChannelModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Alerts.DeleteChannel(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting alert channel", err.Error())
	}
}

func (r *alertChannelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapAlertChannel copies the API view into state. The webhook URL is kept from the
// configuration: reads return it masked.
func mapAlertChannel(state *alertChannelModel, ch *client.AlertChannel) {
	state.ID = types.StringValue(ch.ID)
	state.Name = types.StringValue(ch.Name)
	state.Type = types.StringValue(ch.Type)
	state.Enabled = types.BoolValue(ch.Enabled)
	if email, ok := ch.Config["email"]; ok && email != "" {
		state.Email = types.StringValue(email)
	} else {
		state.Email = types.StringNull()
	}
}

// ---- cubepath_alert_rule ----

func NewAlertRuleResource() resource.Resource {
	return &alertRuleResource{}
}

type alertRuleResource struct {
	client *client.Client
}

type alertRuleModel struct {
	ID              types.String  `tfsdk:"id"`
	ProjectID       types.Int64   `tfsdk:"project_id"`
	Name            types.String  `tfsdk:"name"`
	Description     types.String  `tfsdk:"description"`
	TargetType      types.String  `tfsdk:"target_type"`
	TargetID        types.String  `tfsdk:"target_id"`
	MetricType      types.String  `tfsdk:"metric_type"`
	Operator        types.String  `tfsdk:"operator"`
	Threshold       types.Float64 `tfsdk:"threshold"`
	DurationSeconds types.Int64   `tfsdk:"duration_seconds"`
	CooldownSeconds types.Int64   `tfsdk:"cooldown_seconds"`
	ChannelIDs      types.Set     `tfsdk:"channel_ids"`
	Enabled         types.Bool    `tfsdk:"enabled"`
	Status          types.String  `tfsdk:"status"`
}

func (r *alertRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alert_rule"
}

func (r *alertRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Cloud Alerts rule: notifies the given channels when a metric of a VPS, baremetal " +
			"server or availability group crosses a threshold for a sustained time. Import with the rule ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The ID of the rule.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.Int64Attribute{
				Description:   "Project ID. Changing it forces a new rule.",
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Name, unique in the organization: up to 128 letters, numbers, spaces and hyphens.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Description, up to 500 characters.",
				Optional:    true,
			},
			"target_type": schema.StringAttribute{
				Description: "What the rule watches: vps, baremetal or availability_group.",
				Required:    true,
				Validators:  []validator.String{StringOneOf("vps", "baremetal", "availability_group")},
			},
			"target_id": schema.StringAttribute{
				Description: "ID of the VPS or baremetal server, or UUID of the availability group.",
				Required:    true,
			},
			"metric_type": schema.StringAttribute{
				Description: "Metric: cpu, ram, disk, network_in or network_out. Baremetal servers only support " +
					"network_in and network_out.",
				Required:   true,
				Validators: []validator.String{StringOneOf("cpu", "ram", "disk", "network_in", "network_out")},
			},
			"operator": schema.StringAttribute{
				Description: "Comparison: gt, gte, lt, lte or eq.",
				Required:    true,
				Validators:  []validator.String{StringOneOf("gt", "gte", "lt", "lte", "eq")},
			},
			"threshold": schema.Float64Attribute{
				Description: "Threshold, 0 to 1000000 (percent for cpu, ram and disk).",
				Required:    true,
			},
			"duration_seconds": schema.Int64Attribute{
				Description: "How long the condition must hold before the rule fires, 60 to 3600. Defaults to 300.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(300),
				Validators:  []validator.Int64{Int64Between(60, 3600)},
			},
			"cooldown_seconds": schema.Int64Attribute{
				Description: "Minimum time between two notifications, 60 to 86400. Defaults to 600.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(600),
				Validators:  []validator.Int64{Int64Between(60, 86400)},
			},
			"channel_ids": schema.SetAttribute{
				Description: "IDs of the channels to notify (cubepath_alert_channel), 1 to 10.",
				Required:    true,
				ElementType: types.StringType,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the rule is evaluated. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				Description: "Current status: enabled, disabled, triggered or resolved.",
				Computed:    true,
			},
		},
	}
}

func (r *alertRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

// alertActions builds one notify action per channel, in a stable order.
func alertActions(channelIDs []string) []client.AlertRuleAction {
	ids := append([]string(nil), channelIDs...)
	sort.Strings(ids)
	actions := make([]client.AlertRuleAction, 0, len(ids))
	for i, id := range ids {
		actions = append(actions, client.AlertRuleAction{ActionType: "notify", NotificatorID: id, Order: i, Enabled: true})
	}
	return actions
}

func (r *alertRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan alertRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	channels := stringsFromSet(ctx, plan.ChannelIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.Alerts.CreateRule(ctx, &client.CreateAlertRuleRequest{
		ProjectID:       int(plan.ProjectID.ValueInt64()),
		Name:            plan.Name.ValueString(),
		Description:     knownString(plan.Description),
		TargetType:      plan.TargetType.ValueString(),
		TargetID:        plan.TargetID.ValueString(),
		MetricType:      plan.MetricType.ValueString(),
		Operator:        plan.Operator.ValueString(),
		Threshold:       plan.Threshold.ValueFloat64(),
		DurationSeconds: int(plan.DurationSeconds.ValueInt64()),
		CooldownSeconds: int(plan.CooldownSeconds.ValueInt64()),
		Actions:         alertActions(channels),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating alert rule", err.Error())
		return
	}

	if !plan.Enabled.ValueBool() {
		status := "disabled"
		rule, err = r.client.Alerts.UpdateRule(ctx, rule.ID, &client.UpdateAlertRuleRequest{Status: &status})
		if err != nil {
			resp.Diagnostics.AddError("Error disabling alert rule", err.Error())
			return
		}
	}

	mapAlertRule(ctx, &plan, rule, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state alertRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.Alerts.GetRule(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading alert rule", err.Error())
		return
	}
	mapAlertRule(ctx, &state, rule, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *alertRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state alertRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	threshold := plan.Threshold.ValueFloat64()
	updateReq := &client.UpdateAlertRuleRequest{
		Name:            knownString(plan.Name),
		TargetType:      knownString(plan.TargetType),
		TargetID:        knownString(plan.TargetID),
		MetricType:      knownString(plan.MetricType),
		Operator:        knownString(plan.Operator),
		Threshold:       &threshold,
		DurationSeconds: knownInt(plan.DurationSeconds),
		CooldownSeconds: knownInt(plan.CooldownSeconds),
	}
	if !plan.Description.Equal(state.Description) {
		description := plan.Description.ValueString() // null clears it
		updateReq.Description = &description
	}
	if !plan.ChannelIDs.Equal(state.ChannelIDs) {
		channels := stringsFromSet(ctx, plan.ChannelIDs, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Actions = alertActions(channels)
	}
	if !plan.Enabled.Equal(state.Enabled) {
		status := "disabled"
		if plan.Enabled.ValueBool() {
			status = "enabled"
		}
		updateReq.Status = &status
	}

	rule, err := r.client.Alerts.UpdateRule(ctx, state.ID.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating alert rule", err.Error())
		return
	}
	mapAlertRule(ctx, &plan, rule, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *alertRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state alertRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Alerts.DeleteRule(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting alert rule", err.Error())
	}
}

func (r *alertRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// mapAlertRule copies the API view of a rule into state.
func mapAlertRule(ctx context.Context, state *alertRuleModel, rule *client.AlertRule, diags *diag.Diagnostics) {
	channels := []string{}
	for _, a := range rule.Actions {
		if a.ActionType == "notify" && a.NotificatorID != "" {
			channels = append(channels, a.NotificatorID)
		}
	}
	set, d := types.SetValueFrom(ctx, types.StringType, channels)
	diags.Append(d...)
	state.ChannelIDs = set
	fillAlertRule(state, rule)
}

func fillAlertRule(state *alertRuleModel, rule *client.AlertRule) {
	state.ID = types.StringValue(rule.ID)
	state.ProjectID = types.Int64Value(int64(rule.ProjectID))
	state.Name = types.StringValue(rule.Name)
	state.Description = stringOrNull(rule.Description)
	state.TargetType = types.StringValue(rule.TargetType)
	state.TargetID = types.StringValue(rule.TargetID)
	state.MetricType = types.StringValue(rule.MetricType)
	state.Operator = types.StringValue(rule.Operator)
	state.Threshold = types.Float64Value(rule.Threshold)
	state.DurationSeconds = types.Int64Value(int64(rule.DurationSeconds))
	state.CooldownSeconds = types.Int64Value(int64(rule.CooldownSeconds))
	state.Status = types.StringValue(rule.Status)
	state.Enabled = types.BoolValue(rule.Status != "disabled")
}
