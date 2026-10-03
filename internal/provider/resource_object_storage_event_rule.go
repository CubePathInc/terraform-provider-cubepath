package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &objectStorageEventRuleResource{}
	_ resource.ResourceWithConfigure      = &objectStorageEventRuleResource{}
	_ resource.ResourceWithImportState    = &objectStorageEventRuleResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageEventRuleResource{}
)

var storageEventTypes = map[string]bool{"object.created": true, "object.removed": true, "object.tagging": true}

func NewObjectStorageEventRuleResource() resource.Resource {
	return &objectStorageEventRuleResource{}
}

type objectStorageEventRuleResource struct {
	client *client.Client
}

type objectStorageEventRuleModel struct {
	ID              types.String `tfsdk:"id"`
	BucketUUID      types.String `tfsdk:"bucket_uuid"`
	Name            types.String `tfsdk:"name"`
	DestinationUUID types.String `tfsdk:"destination_uuid"`
	Events          types.Set    `tfsdk:"events"`
	Prefix          types.String `tfsdk:"prefix"`
	Suffix          types.String `tfsdk:"suffix"`
	Enabled         types.Bool   `tfsdk:"enabled"`
	Status          types.String `tfsdk:"status"`
	ErrorMessage    types.String `tfsdk:"error_message"`
}

func (r *objectStorageEventRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_event_rule"
}

func (r *objectStorageEventRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sends the events of an Object Storage bucket (objects created, removed or tagged) whose key " +
			"matches prefix and suffix to a cubepath_object_storage_event_destination. Rules are applied in the " +
			"background; Terraform waits until the rule is active. Import with the rule UUID, or " +
			"<bucket_uuid>/<rule_uuid> to skip the search through every bucket " +
			"(terraform import cubepath_object_storage_event_rule.example <bucket_uuid>/<rule_uuid>).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the rule.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bucket_uuid": schema.StringAttribute{
				Description:   "UUID of the bucket. Changing it forces a new rule.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Rule name: 1 to 64 letters, digits, hyphens and spaces.",
				Required:    true,
			},
			"destination_uuid": schema.StringAttribute{
				Description: "UUID of the event destination. It must be enabled.",
				Required:    true,
			},
			"events": schema.SetAttribute{
				Description: "Event types: object.created, object.removed and/or object.tagging (1 to 3).",
				ElementType: types.StringType,
				Required:    true,
			},
			"prefix": schema.StringAttribute{
				Description: "Only keys starting with this prefix (up to 1024 characters). Defaults to all keys.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"suffix": schema.StringAttribute{
				Description: "Only keys ending with this suffix (up to 1024 characters). Defaults to all keys.",
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the rule sends events. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				Description: "pending, active or error.",
				Computed:    true,
			},
			"error_message": schema.StringAttribute{
				Description: "Why the rule could not be applied, when status is error.",
				Computed:    true,
			},
		},
	}
}

func (r *objectStorageEventRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *objectStorageEventRuleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var events types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("events"), &events)...)
	if resp.Diagnostics.HasError() || events.IsNull() || events.IsUnknown() {
		return
	}
	for _, e := range events.Elements() {
		if e.IsUnknown() {
			return
		}
	}
	values := stringsFromSet(ctx, events, &resp.Diagnostics)
	resp.Diagnostics.Append(validateStorageEvents(values)...)
}

func validateStorageEvents(values []string) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(values) == 0 {
		diags.AddAttributeError(path.Root("events"), "Invalid events", "Choose at least one event type.")
	}
	for _, v := range values {
		if !storageEventTypes[v] {
			diags.AddAttributeError(path.Root("events"), "Invalid events",
				fmt.Sprintf("Unknown event type %q: use object.created, object.removed or object.tagging.", v))
		}
	}
	return diags
}

func (r *objectStorageEventRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan objectStorageEventRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	events := stringsFromSet(ctx, plan.Events, &resp.Diagnostics)
	sort.Strings(events)
	bucket := plan.BucketUUID.ValueString()

	created, err := r.client.ObjectStorage.CreateEventRule(ctx, bucket, &client.CreateObjectStorageEventRuleRequest{
		Name:            plan.Name.ValueString(),
		DestinationUUID: plan.DestinationUUID.ValueString(),
		Events:          events,
		Prefix:          plan.Prefix.ValueString(),
		Suffix:          plan.Suffix.ValueString(),
		Enabled:         plan.Enabled.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage event rule", err.Error())
		return
	}
	plan.ID = types.StringValue(created.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket_uuid"), plan.BucketUUID)...)

	rule, err := r.waitApplied(ctx, bucket, created.UUID)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the event rule to be applied", err.Error())
		return
	}
	resp.Diagnostics.Append(mapEventRule(ctx, &plan, rule)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// waitApplied polls the bucket's rules until the rule leaves pending.
func (r *objectStorageEventRuleResource) waitApplied(ctx context.Context, bucket, ruleUUID string) (*client.ObjectStorageEventRule, error) {
	conf := &utils.StateChangeConf{
		Pending:      []string{"pending"},
		Target:       []string{"active"},
		Timeout:      accessKeyTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			rule, err := r.client.ObjectStorage.GetEventRule(ctx, bucket, ruleUUID)
			if err != nil {
				return nil, "", err
			}
			if rule == nil {
				return nil, "", fmt.Errorf("the event rule %s disappeared", ruleUUID)
			}
			if rule.Status == "error" {
				msg := "unknown error"
				if rule.ErrorMessage != nil {
					msg = *rule.ErrorMessage
				}
				return nil, "", fmt.Errorf("the event rule could not be applied: %s", msg)
			}
			return rule, rule.Status, nil
		},
	}
	result, err := conf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*client.ObjectStorageEventRule), nil
}

func (r *objectStorageEventRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state objectStorageEventRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rule, err := r.client.ObjectStorage.GetEventRule(ctx, state.BucketUUID.ValueString(), state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage event rule", err.Error())
		return
	}
	if rule == nil || rule.Status == "deleted" {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(mapEventRule(ctx, &state, rule)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *objectStorageEventRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state objectStorageEventRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	upd := eventRuleUpdate(ctx, plan, state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, id := state.BucketUUID.ValueString(), state.ID.ValueString()
	if _, err := r.client.ObjectStorage.UpdateEventRule(ctx, bucket, id, upd); err != nil {
		resp.Diagnostics.AddError("Error updating Object Storage event rule", err.Error())
		return
	}
	rule, err := r.waitApplied(ctx, bucket, id)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the event rule to be applied", err.Error())
		return
	}
	plan.ID = state.ID
	resp.Diagnostics.Append(mapEventRule(ctx, &plan, rule)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// eventRuleUpdate sends only what changed between the state and the plan.
func eventRuleUpdate(ctx context.Context, plan, state objectStorageEventRuleModel, diags *diag.Diagnostics) *client.UpdateObjectStorageEventRuleRequest {
	upd := &client.UpdateObjectStorageEventRuleRequest{}
	if !plan.Name.Equal(state.Name) {
		upd.Name = knownString(plan.Name)
	}
	if !plan.DestinationUUID.Equal(state.DestinationUUID) {
		upd.DestinationUUID = knownString(plan.DestinationUUID)
	}
	if !plan.Events.Equal(state.Events) {
		events := stringsFromSet(ctx, plan.Events, diags)
		sort.Strings(events)
		upd.Events = &events
	}
	if !plan.Prefix.Equal(state.Prefix) {
		upd.Prefix = knownString(plan.Prefix)
	}
	if !plan.Suffix.Equal(state.Suffix) {
		upd.Suffix = knownString(plan.Suffix)
	}
	if !plan.Enabled.Equal(state.Enabled) {
		upd.Enabled = knownBool(plan.Enabled)
	}
	return upd
}

func (r *objectStorageEventRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state objectStorageEventRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	bucket, id := state.BucketUUID.ValueString(), state.ID.ValueString()
	if err := r.client.ObjectStorage.DeleteEventRule(ctx, bucket, id); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting Object Storage event rule", err.Error())
		return
	}
	// The rule is removed from the bucket in the background; wait so that deleting its
	// destination right after does not find it still in use.
	conf := &utils.StateChangeConf{
		Pending:      []string{"pending", "active", "error", "deleting"},
		Target:       []string{"deleted"},
		Timeout:      accessKeyTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			rule, err := r.client.ObjectStorage.GetEventRule(ctx, bucket, id)
			if err != nil {
				if isNotFound(err) {
					return struct{}{}, "deleted", nil
				}
				return nil, "", err
			}
			if rule == nil {
				return struct{}{}, "deleted", nil
			}
			return rule, rule.Status, nil
		},
	}
	if _, err := conf.WaitForState(ctx); err != nil {
		resp.Diagnostics.AddError("Error waiting for the event rule to be deleted", err.Error())
	}
}

func (r *objectStorageEventRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var rule *client.ObjectStorageEventRule
	var err error
	if bucket, id, ok := strings.Cut(req.ID, "/"); ok {
		rule, err = r.client.ObjectStorage.GetEventRule(ctx, bucket, id)
	} else {
		rule, err = r.findEventRule(ctx, req.ID)
	}
	if err == nil && rule == nil {
		err = fmt.Errorf("event rule %s not found", req.ID)
	}
	if err != nil {
		resp.Diagnostics.AddError("Error importing Object Storage event rule", err.Error())
		return
	}
	state := objectStorageEventRuleModel{}
	resp.Diagnostics.Append(mapEventRule(ctx, &state, rule)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// findEventRule looks for a rule in every bucket of the organization.
func (r *objectStorageEventRuleResource) findEventRule(ctx context.Context, ruleUUID string) (*client.ObjectStorageEventRule, error) {
	buckets, err := r.client.ObjectStorage.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range buckets {
		rule, err := r.client.ObjectStorage.GetEventRule(ctx, b.UUID, ruleUUID)
		if err != nil {
			return nil, err
		}
		if rule != nil {
			return rule, nil
		}
	}
	return nil, nil
}

func mapEventRule(ctx context.Context, m *objectStorageEventRuleModel, rule *client.ObjectStorageEventRule) diag.Diagnostics {
	m.ID = types.StringValue(rule.UUID)
	m.BucketUUID = types.StringValue(rule.BucketUUID)
	m.Name = types.StringValue(rule.Name)
	if rule.Destination != nil {
		m.DestinationUUID = types.StringValue(rule.Destination.UUID)
	}
	events, diags := types.SetValueFrom(ctx, types.StringType, rule.Events)
	m.Events = events
	m.Prefix = types.StringValue(rule.Prefix)
	m.Suffix = types.StringValue(rule.Suffix)
	m.Enabled = types.BoolValue(rule.Enabled)
	m.Status = types.StringValue(rule.Status)
	m.ErrorMessage = stringOrNull(rule.ErrorMessage)
	return diags
}
