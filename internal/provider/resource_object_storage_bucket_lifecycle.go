package provider

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &objectStorageBucketLifecycleResource{}
	_ resource.ResourceWithConfigure      = &objectStorageBucketLifecycleResource{}
	_ resource.ResourceWithImportState    = &objectStorageBucketLifecycleResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageBucketLifecycleResource{}
)

// The API applies a change within seconds, or up to about 12 minutes after a previous change of the
// same bucket (one write to the storage service per bucket every 10 minutes).
const lifecycleApplyTimeout = 20 * time.Minute

var (
	lifecycleRuleIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	lifecycleDateRe   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func NewObjectStorageBucketLifecycleResource() resource.Resource {
	return &objectStorageBucketLifecycleResource{}
}

type objectStorageBucketLifecycleResource struct {
	client *client.Client
}

type lifecycleResourceModel struct {
	ID         types.String         `tfsdk:"id"`
	BucketUUID types.String         `tfsdk:"bucket_uuid"`
	Status     types.String         `tfsdk:"status"`
	Rules      []lifecycleRuleModel `tfsdk:"rule"`
}

type lifecycleRuleModel struct {
	ID                                 types.String `tfsdk:"id"`
	Enabled                            types.Bool   `tfsdk:"enabled"`
	Prefix                             types.String `tfsdk:"prefix"`
	Tags                               types.Map    `tfsdk:"tags"`
	ObjectSizeGreaterThan              types.Int64  `tfsdk:"object_size_greater_than"`
	ObjectSizeLessThan                 types.Int64  `tfsdk:"object_size_less_than"`
	ExpirationDays                     types.Int64  `tfsdk:"expiration_days"`
	ExpirationDate                     types.String `tfsdk:"expiration_date"`
	ExpiredObjectDeleteMarker          types.Bool   `tfsdk:"expired_object_delete_marker"`
	NoncurrentDays                     types.Int64  `tfsdk:"noncurrent_days"`
	NewerNoncurrentVersions            types.Int64  `tfsdk:"newer_noncurrent_versions"`
	AbortIncompleteMultipartUploadDays types.Int64  `tfsdk:"abort_incomplete_multipart_upload_days"`
}

func (r *objectStorageBucketLifecycleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_bucket_lifecycle"
}

func (r *objectStorageBucketLifecycleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages every lifecycle rule of a CubePath Object Storage bucket (one resource per bucket: " +
			"it replaces any rule set elsewhere). Rules delete objects in the background, permanently: current " +
			"objects after a number of days or on a date, noncurrent versions, orphan delete markers and " +
			"incomplete multipart uploads. Objects are removed within 48 hours of their due date. In a " +
			"versioned bucket an expiration only adds a delete marker: add noncurrent_days to free the space. " +
			"Create and update wait until the rules are applied (seconds, up to about 12 minutes after a previous " +
			"change of the same bucket). Destroying it removes every rule. Import with the bucket UUID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the bucket.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bucket_uuid": schema.StringAttribute{
				Description:   "UUID of the bucket (cubepath_object_storage_bucket.<name>.id). Changing it forces a new resource.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				Description: "Lifecycle status: active, paused (the bucket is blocked or on hold: rules do not run), " +
					"pending or error.",
				Computed: true,
			},
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				Description: "A lifecycle rule (1 to 100). Each needs at least one action: expiration_days, " +
					"expiration_date, expired_object_delete_marker, noncurrent_days or abort_incomplete_multipart_upload_days.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Description: "Rule ID: 1 to 64 letters, numbers, dots, hyphens and underscores, unique in the bucket, " +
								"not starting with cubepath-.",
							Required: true,
						},
						"enabled": schema.BoolAttribute{
							Description: "Whether the rule runs. Defaults to true.",
							Optional:    true,
							Computed:    true,
							Default:     booldefault.StaticBool(true),
						},
						"prefix": schema.StringAttribute{
							Description: "Only objects whose key starts with this prefix (no leading slash). Without prefix, tags " +
								"or sizes the rule covers the whole bucket.",
							Optional: true,
						},
						"tags": schema.MapAttribute{
							Description: "Only objects with all these tags (up to 10). Not with expired_object_delete_marker.",
							ElementType: types.StringType,
							Optional:    true,
						},
						"object_size_greater_than": schema.Int64Attribute{
							Description: "Only objects larger than this many bytes.",
							Optional:    true,
						},
						"object_size_less_than": schema.Int64Attribute{
							Description: "Only objects smaller than this many bytes.",
							Optional:    true,
						},
						"expiration_days": schema.Int64Attribute{
							Description: "Delete current objects this many days after they are written (1 to 36500).",
							Optional:    true,
						},
						"expiration_date": schema.StringAttribute{
							Description: "Delete current objects on this date (YYYY-MM-DD, after today UTC). Not with expiration_days.",
							Optional:    true,
						},
						"expired_object_delete_marker": schema.BoolAttribute{
							Description: "Remove delete markers left without versions. Not with expiration_days, expiration_date or tags.",
							Optional:    true,
						},
						"noncurrent_days": schema.Int64Attribute{
							Description: "Delete noncurrent versions this many days after they stop being current (1 to 36500).",
							Optional:    true,
						},
						"newer_noncurrent_versions": schema.Int64Attribute{
							Description: "With noncurrent_days: keep this many newest noncurrent versions (1 to 100).",
							Optional:    true,
						},
						"abort_incomplete_multipart_upload_days": schema.Int64Attribute{
							Description: "Abort incomplete multipart uploads after this many days (1 to 7; CubePath aborts them " +
								"after 7 days anyway).",
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func (r *objectStorageBucketLifecycleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *objectStorageBucketLifecycleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var rules []lifecycleRuleModel
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("rule"), &rules)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateLifecycleRules(rules)...)
}

// validateLifecycleRules checks what can be checked without the API (unknown values are skipped).
func validateLifecycleRules(rules []lifecycleRuleModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if len(rules) == 0 || len(rules) > 100 {
		diags.AddAttributeError(path.Root("rule"), "Invalid rules", "A lifecycle configuration needs between 1 and 100 rule blocks.")
		return diags
	}
	seen := map[string]bool{}
	for i, rule := range rules {
		at := path.Root("rule").AtListIndex(i)
		if known(rule.ID) {
			id := rule.ID.ValueString()
			if !lifecycleRuleIDRe.MatchString(id) || strings.HasPrefix(id, "cubepath-") {
				diags.AddAttributeError(at.AtName("id"), "Invalid rule ID",
					"Rule IDs are 1 to 64 letters, numbers, dots, hyphens and underscores, and cannot start with cubepath-.")
			}
			if seen[id] {
				diags.AddAttributeError(at.AtName("id"), "Repeated rule ID", fmt.Sprintf("Rule ID %q is used twice.", id))
			}
			seen[id] = true
		}
		if known(rule.Prefix) && strings.HasPrefix(rule.Prefix.ValueString(), "/") {
			diags.AddAttributeError(at.AtName("prefix"), "Invalid prefix",
				"Remove the leading slash: object keys are stored without it.")
		}
		if known(rule.ExpirationDate) && !lifecycleDateRe.MatchString(rule.ExpirationDate.ValueString()) {
			diags.AddAttributeError(at.AtName("expiration_date"), "Invalid date", "Use YYYY-MM-DD.")
		}
		if !rule.ExpirationDays.IsNull() && !rule.ExpirationDate.IsNull() {
			diags.AddAttributeError(at, "Invalid expiration", "Use either expiration_days or expiration_date, not both.")
		}
		if !rule.NewerNoncurrentVersions.IsNull() && rule.NoncurrentDays.IsNull() {
			diags.AddAttributeError(at.AtName("newer_noncurrent_versions"), "Missing noncurrent_days",
				"newer_noncurrent_versions needs noncurrent_days.")
		}
		hasAction := !rule.ExpirationDays.IsNull() || !rule.ExpirationDate.IsNull() ||
			(!rule.ExpiredObjectDeleteMarker.IsNull() && !(known(rule.ExpiredObjectDeleteMarker) && !rule.ExpiredObjectDeleteMarker.ValueBool())) ||
			!rule.NoncurrentDays.IsNull() || !rule.AbortIncompleteMultipartUploadDays.IsNull()
		if !hasAction {
			diags.AddAttributeError(at, "Rule without action",
				"Set at least one of expiration_days, expiration_date, expired_object_delete_marker, noncurrent_days or abort_incomplete_multipart_upload_days.")
		}
	}
	return diags
}

func known(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

func int64Ptr(v types.Int64) *int64 {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	n := v.ValueInt64()
	return &n
}

// rulesFromModel builds the API rules of the plan.
func rulesFromModel(ctx context.Context, models []lifecycleRuleModel) ([]client.ObjectStorageLifecycleRule, diag.Diagnostics) {
	var diags diag.Diagnostics
	rules := make([]client.ObjectStorageLifecycleRule, 0, len(models))
	for _, m := range models {
		rule := client.ObjectStorageLifecycleRule{ID: m.ID.ValueString(), Enabled: m.Enabled.IsNull() || m.Enabled.ValueBool()}

		filter := &client.ObjectStorageLifecycleFilter{
			ObjectSizeGreaterThan: int64Ptr(m.ObjectSizeGreaterThan),
			ObjectSizeLessThan:    int64Ptr(m.ObjectSizeLessThan),
		}
		if known(m.Prefix) && m.Prefix.ValueString() != "" {
			p := m.Prefix.ValueString()
			filter.Prefix = &p
		}
		if known(m.Tags) {
			tags := map[string]string{}
			diags.Append(m.Tags.ElementsAs(ctx, &tags, false)...)
			keys := make([]string, 0, len(tags))
			for k := range tags {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				filter.Tags = append(filter.Tags, client.ObjectStorageLifecycleTag{Key: k, Value: tags[k]})
			}
		}
		if filter.Prefix != nil || len(filter.Tags) > 0 || filter.ObjectSizeGreaterThan != nil || filter.ObjectSizeLessThan != nil {
			rule.Filter = filter
		}

		exp := &client.ObjectStorageLifecycleExpiration{Days: int64Ptr(m.ExpirationDays)}
		if known(m.ExpirationDate) {
			d := m.ExpirationDate.ValueString()
			exp.Date = &d
		}
		if known(m.ExpiredObjectDeleteMarker) && m.ExpiredObjectDeleteMarker.ValueBool() {
			t := true
			exp.ExpiredObjectDeleteMarker = &t
		}
		if exp.Days != nil || exp.Date != nil || exp.ExpiredObjectDeleteMarker != nil {
			rule.Expiration = exp
		}
		if days := int64Ptr(m.NoncurrentDays); days != nil {
			rule.NoncurrentVersionExpiration = &client.ObjectStorageLifecycleNoncurrentExpiration{
				NoncurrentDays: *days, NewerNoncurrentVersions: int64Ptr(m.NewerNoncurrentVersions),
			}
		}
		if days := int64Ptr(m.AbortIncompleteMultipartUploadDays); days != nil {
			rule.AbortIncompleteMultipartUpload = &client.ObjectStorageLifecycleAbortUpload{DaysAfterInitiation: *days}
		}
		rules = append(rules, rule)
	}
	return rules, diags
}

func optionalInt64(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}

// modelFromRules maps the API rules to the state.
func modelFromRules(rules []client.ObjectStorageLifecycleRule) []lifecycleRuleModel {
	out := make([]lifecycleRuleModel, 0, len(rules))
	for _, rule := range rules {
		m := lifecycleRuleModel{
			ID:                                 types.StringValue(rule.ID),
			Enabled:                            types.BoolValue(rule.Enabled),
			Prefix:                             types.StringNull(),
			Tags:                               types.MapNull(types.StringType),
			ObjectSizeGreaterThan:              types.Int64Null(),
			ObjectSizeLessThan:                 types.Int64Null(),
			ExpirationDays:                     types.Int64Null(),
			ExpirationDate:                     types.StringNull(),
			ExpiredObjectDeleteMarker:          types.BoolNull(),
			NoncurrentDays:                     types.Int64Null(),
			NewerNoncurrentVersions:            types.Int64Null(),
			AbortIncompleteMultipartUploadDays: types.Int64Null(),
		}
		if f := rule.Filter; f != nil {
			if f.Prefix != nil && *f.Prefix != "" {
				m.Prefix = types.StringValue(*f.Prefix)
			}
			if len(f.Tags) > 0 {
				tags := map[string]attr.Value{}
				for _, t := range f.Tags {
					tags[t.Key] = types.StringValue(t.Value)
				}
				m.Tags = types.MapValueMust(types.StringType, tags)
			}
			m.ObjectSizeGreaterThan = optionalInt64(f.ObjectSizeGreaterThan)
			m.ObjectSizeLessThan = optionalInt64(f.ObjectSizeLessThan)
		}
		if e := rule.Expiration; e != nil {
			m.ExpirationDays = optionalInt64(e.Days)
			if e.Date != nil {
				m.ExpirationDate = types.StringValue(*e.Date)
			}
			if e.ExpiredObjectDeleteMarker != nil && *e.ExpiredObjectDeleteMarker {
				m.ExpiredObjectDeleteMarker = types.BoolValue(true)
			}
		}
		if n := rule.NoncurrentVersionExpiration; n != nil {
			m.NoncurrentDays = types.Int64Value(n.NoncurrentDays)
			m.NewerNoncurrentVersions = optionalInt64(n.NewerNoncurrentVersions)
		}
		if a := rule.AbortIncompleteMultipartUpload; a != nil {
			m.AbortIncompleteMultipartUploadDays = types.Int64Value(a.DaysAfterInitiation)
		}
		out = append(out, m)
	}
	return out
}

func (r *objectStorageBucketLifecycleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan lifecycleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *objectStorageBucketLifecycleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan lifecycleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.State, &resp.Diagnostics)
}

// apply sends the rules, waits until they are applied and saves the state.
func (r *objectStorageBucketLifecycleResource) apply(ctx context.Context, plan *lifecycleResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	uuid := plan.BucketUUID.ValueString()
	rules, d := rulesFromModel(ctx, plan.Rules)
	diags.Append(d...)
	if diags.HasError() {
		return
	}
	change, err := r.client.ObjectStorage.PutBucketLifecycle(ctx, uuid, rules)
	if err != nil {
		diags.AddError("Error setting the bucket lifecycle rules", err.Error())
		return
	}
	lifecycle, err := r.waitApplied(ctx, uuid, change.Generation)
	if err != nil {
		diags.AddError("Error waiting for the lifecycle rules to be applied", err.Error())
		return
	}
	plan.ID = types.StringValue(uuid)
	plan.Status = types.StringValue(lifecycle.Status)
	diags.Append(state.Set(ctx, plan)...)
}

// waitApplied polls the lifecycle until generation is applied (nil = nothing changed).
func (r *objectStorageBucketLifecycleResource) waitApplied(ctx context.Context, uuid string, generation *int64) (*client.ObjectStorageLifecycle, error) {
	conf := &utils.StateChangeConf{
		Pending:      []string{"applying"},
		Target:       []string{"applied"},
		Timeout:      lifecycleApplyTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			lifecycle, err := r.client.ObjectStorage.GetBucketLifecycle(ctx, uuid)
			if err != nil {
				return nil, "", err
			}
			if lifecycle.Status == "error" {
				msg := "the storage service rejected the rules"
				if lifecycle.Error != nil && *lifecycle.Error != "" {
					msg = *lifecycle.Error
				}
				return nil, "", errors.New(msg)
			}
			if generation == nil || lifecycle.AppliedGeneration >= *generation {
				return lifecycle, "applied", nil
			}
			return lifecycle, "applying", nil
		},
	}
	result, err := conf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*client.ObjectStorageLifecycle), nil
}

func (r *objectStorageBucketLifecycleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state lifecycleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	lifecycle, err := r.client.ObjectStorage.GetBucketLifecycle(ctx, state.BucketUUID.ValueString())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading the bucket lifecycle rules", err.Error())
		return
	}
	if len(lifecycle.Rules) == 0 {
		// The rules were removed outside Terraform
		resp.State.RemoveResource(ctx)
		return
	}
	state.ID = types.StringValue(state.BucketUUID.ValueString())
	state.Status = types.StringValue(lifecycle.Status)
	state.Rules = modelFromRules(lifecycle.Rules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *objectStorageBucketLifecycleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state lifecycleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.ObjectStorage.DeleteBucketLifecycle(ctx, state.BucketUUID.ValueString()); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		resp.Diagnostics.AddError("Error removing the bucket lifecycle rules", err.Error())
	}
}

func (r *objectStorageBucketLifecycleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("bucket_uuid"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
