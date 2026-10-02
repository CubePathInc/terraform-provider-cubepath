package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ resource.Resource                   = &objectStorageBucketResource{}
	_ resource.ResourceWithConfigure      = &objectStorageBucketResource{}
	_ resource.ResourceWithImportState    = &objectStorageBucketResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageBucketResource{}
	_ resource.ResourceWithModifyPlan     = &objectStorageBucketResource{}
)

const (
	bucketCreateTimeout = 10 * time.Minute
	bucketDeleteTimeout = 60 * time.Minute
	bucketPollInterval  = 5 * time.Second
)

func NewObjectStorageBucketResource() resource.Resource {
	return &objectStorageBucketResource{}
}

type objectStorageBucketResource struct {
	client *client.Client
}

type objectStorageBucketResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Tier         types.String `tfsdk:"tier"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	Versioning   types.String `tfsdk:"versioning"`
	Protected    types.Bool   `tfsdk:"protected"`
	ForceDestroy types.Bool   `tfsdk:"force_destroy"`
	Status       types.String `tfsdk:"status"`
	Region       types.String `tfsdk:"region"`
	Endpoint     types.String `tfsdk:"endpoint"`
	LocationName types.String `tfsdk:"location_name"`
	Tags         types.Map    `tfsdk:"tags"`

	ObjectLockEnabled          types.Bool   `tfsdk:"object_lock_enabled"`
	ObjectLockDefaultRetention types.Object `tfsdk:"object_lock_default_retention"`
	AcceptObjectLockTerms      types.Bool   `tfsdk:"accept_object_lock_terms"`
	BypassGovernanceOnDestroy  types.Bool   `tfsdk:"bypass_governance_on_destroy"`
	LockedContentKept          types.Bool   `tfsdk:"locked_content_kept"`
}

// lockRetentionAttrTypes is the shape of object_lock_default_retention.
var lockRetentionAttrTypes = map[string]attr.Type{
	"mode":  types.StringType,
	"days":  types.Int64Type,
	"years": types.Int64Type,
}

type lockRetentionModel struct {
	Mode  types.String `tfsdk:"mode"`
	Days  types.Int64  `tfsdk:"days"`
	Years types.Int64  `tfsdk:"years"`
}

func (r *objectStorageBucketResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_bucket"
}

func (r *objectStorageBucketResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CubePath Object Storage (S3 compatible) bucket. Buckets are private: " +
			"use cubepath_object_storage_access_key for S3 clients, and cubepath_cdn_origin with " +
			"object_storage_bucket_uuid to serve the bucket publicly through the CDN. " +
			"Import with the bucket UUID (terraform import cubepath_object_storage_bucket.example <uuid>); " +
			"force_destroy, bypass_governance_on_destroy and accept_object_lock_terms are not stored by the API " +
			"and import as false or null.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the bucket.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description: "Bucket name, unique across all CubePath customers: 3 to 63 lowercase letters, " +
					"numbers and hyphens, starting and ending with a letter or number. Changing it forces a new bucket.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"tier": schema.StringAttribute{
				Description: "Storage tier, by slug (for example infrequent_access) or UUID. " +
					"See the cubepath_object_storage_tiers data source. Changing it forces a new bucket.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"project_id": schema.Int64Attribute{
				Description: "Project ID. Defaults to the organization's first project. Changing it forces a new bucket.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			"versioning": schema.StringAttribute{
				Description: "Object versioning: off, enabled or suspended. A new bucket can only start as off or enabled, " +
					"and once enabled it cannot go back to off (suspend it instead).",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("off"),
			},
			"protected": schema.BoolAttribute{
				Description: "Deletion protection. A protected bucket cannot be deleted until this is set to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"tags": schema.MapAttribute{
				Description: "Labels to organize and filter buckets, as key = value. At most 50; keys 1 to 128 and values " +
					"0 to 256 characters of letters, numbers, spaces and _ . : / = + - @. Keys cannot contain =, start or end " +
					"with a space, or start with aws:, cp: or cubepath:. Changed in place; when set, tags edited outside Terraform show " +
					"as drift. Without tags in the configuration Terraform leaves the bucket's tags alone (they may be set " +
					"from the dashboard); with tags it manages every tag, and {} removes them all. Bucket tags are not " +
					"visible through S3 (GetBucketTagging and PutBucketTagging answer 403).",
				ElementType: types.StringType,
				Optional:    true,
				// Computed without a default: an omitted tags keeps whatever the bucket has instead of
				// planning {} and wiping tags set outside Terraform.
				Computed:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()},
			},
			"force_destroy": schema.BoolAttribute{
				Description: "Delete every object, version and unfinished upload when the bucket is destroyed. " +
					"Without it, destroying a bucket that is not empty fails.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"object_lock_enabled": schema.BoolAttribute{
				Description: "Create the bucket with Object Lock (WORM): object versions can be protected from " +
					"deletion and overwrite until a retention date. It can only be chosen when the bucket is created " +
					"and never turned off: changing it forces a new bucket. Requires versioning = \"enabled\" and " +
					"accept_object_lock_terms = true. When protected is not set, a bucket with Object Lock is created " +
					"with deletion protection on.",
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"object_lock_default_retention": schema.SingleNestedAttribute{
				Description: "Default retention applied to every object version that has no retention of its own " +
					"(only with object_lock_enabled). Changed in place; remove it to drop the rule. A compliance rule " +
					"can only be kept or lengthened, never removed, shortened or turned into governance. Setting " +
					"compliance or lengthening the rule needs accept_object_lock_terms = true.",
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"mode": schema.StringAttribute{
						Description: "governance (keys with bypass_governance can still delete) or compliance " +
							"(nobody can delete or shorten it before the date, CubePath included; only for " +
							"organizations with compliance enabled by support).",
						Required: true,
					},
					"days": schema.Int64Attribute{
						Description: "Retention in days. Set days or years, not both.",
						Optional:    true,
					},
					"years": schema.Int64Attribute{
						Description: "Retention in years. Set days or years, not both.",
						Optional:    true,
					},
				},
			},
			"accept_object_lock_terms": schema.BoolAttribute{
				Description: "Accept the Object Lock terms. Required (true) to create a bucket with Object Lock and to " +
					"set a compliance rule or lengthen the default retention. Sent to the API only; never read back.",
				Optional: true,
			},
			"bypass_governance_on_destroy": schema.BoolAttribute{
				Description: "With force_destroy on a bucket with Object Lock, also delete the versions under " +
					"governance retention when the bucket is destroyed. Versions under compliance retention or legal " +
					"hold are always kept. Not stored by the API.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
			},
			"locked_content_kept": schema.BoolAttribute{
				Description: "True when the last delete left object versions protected by Object Lock (retention " +
					"or legal hold) in the bucket; it keeps being billed until they expire and it is deleted again.",
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Description: "Bucket status: pending, active, suspended, blocked, error or deleting.",
				Computed:    true,
			},
			"region": schema.StringAttribute{
				Description:   "S3 region to configure in clients (for example eu).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint": schema.StringAttribute{
				Description:   "S3 endpoint to configure in clients (for example https://eu.cubestorage.io).",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"location_name": schema.StringAttribute{
				Description:   "Location of the tier's storage cluster.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *objectStorageBucketResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *objectStorageBucketResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var tags types.Map
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("tags"), &tags)...)
	if !tags.IsNull() && !tags.IsUnknown() {
		elems := make(map[string]types.String, len(tags.Elements()))
		resp.Diagnostics.Append(tags.ElementsAs(ctx, &elems, false)...)
		known := make(map[string]*string, len(elems))
		for k, v := range elems {
			if v.IsUnknown() || v.IsNull() {
				known[k] = nil
				continue
			}
			s := v.ValueString()
			known[k] = &s
		}
		for _, msg := range validateBucketTags(known) {
			resp.Diagnostics.AddAttributeError(path.Root("tags"), "Invalid tags", msg)
		}
	}

	var versioning types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("versioning"), &versioning)...)
	if !versioning.IsNull() && !versioning.IsUnknown() {
		switch versioning.ValueString() {
		case "off", "enabled", "suspended":
		default:
			resp.Diagnostics.AddAttributeError(path.Root("versioning"), "Invalid versioning",
				"versioning must be off, enabled or suspended.")
		}
	}

	var lockEnabled, acceptTerms types.Bool
	var retention types.Object
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("object_lock_enabled"), &lockEnabled)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("accept_object_lock_terms"), &acceptTerms)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("object_lock_default_retention"), &retention)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if lockEnabled.ValueBool() {
		// versioning is null (default off) or known and not enabled: the API refuses both.
		if !versioning.IsUnknown() && versioning.ValueString() != "enabled" {
			resp.Diagnostics.AddAttributeError(path.Root("versioning"), "Object Lock requires versioning",
				"A bucket with Object Lock must have versioning = \"enabled\", and it cannot be suspended.")
		}
		if !acceptTerms.IsUnknown() && !acceptTerms.ValueBool() {
			resp.Diagnostics.AddAttributeError(path.Root("accept_object_lock_terms"), "Object Lock terms not accepted",
				"Set accept_object_lock_terms = true to create a bucket with Object Lock.")
		}
	}
	if retention.IsNull() || retention.IsUnknown() {
		return
	}
	if !lockEnabled.IsUnknown() && !lockEnabled.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("object_lock_default_retention"), "Object Lock is not enabled",
			"object_lock_default_retention can only be set when object_lock_enabled is true.")
	}
	var rule lockRetentionModel
	resp.Diagnostics.Append(retention.As(ctx, &rule, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}
	for _, msg := range validateLockRetention(rule) {
		resp.Diagnostics.AddAttributeError(path.Root("object_lock_default_retention"), "Invalid default retention", msg)
	}
}

// validateLockRetention checks a default retention rule like the API does (the maximum period
// is left to the API, it is configurable there). Unknown values are skipped.
func validateLockRetention(rule lockRetentionModel) []string {
	var errs []string
	if !rule.Mode.IsNull() && !rule.Mode.IsUnknown() {
		if m := rule.Mode.ValueString(); m != "governance" && m != "compliance" {
			errs = append(errs, "mode must be governance or compliance.")
		}
	}
	if rule.Days.IsUnknown() || rule.Years.IsUnknown() {
		return errs
	}
	switch {
	case rule.Days.IsNull() == rule.Years.IsNull():
		errs = append(errs, "Set either days or years for the default retention, not both.")
	case !rule.Days.IsNull() && rule.Days.ValueInt64() < 1,
		!rule.Years.IsNull() && rule.Years.ValueInt64() < 1:
		errs = append(errs, "The default retention must be a positive whole number of days or years.")
	}
	return errs
}

// ModifyPlan makes deletion protection default to on for a bucket with Object Lock (the API
// creates it protected): when protected is not in the configuration, it follows object_lock_enabled.
func (r *objectStorageBucketResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var configProtected types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("protected"), &configProtected)...)
	if resp.Diagnostics.HasError() || !configProtected.IsNull() {
		return
	}
	var lockEnabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("object_lock_enabled"), &lockEnabled)...)
	if resp.Diagnostics.HasError() || !lockEnabled.ValueBool() {
		return
	}
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("protected"), types.BoolValue(true))...)
}

// retentionFromModel converts the planned rule to the API shape; nil when there is none.
func retentionFromModel(ctx context.Context, o types.Object, diags *diag.Diagnostics) *client.ObjectStorageLockRetention {
	if o.IsNull() || o.IsUnknown() {
		return nil
	}
	var rule lockRetentionModel
	diags.Append(o.As(ctx, &rule, basetypes.ObjectAsOptions{})...)
	out := &client.ObjectStorageLockRetention{Mode: rule.Mode.ValueString()}
	if !rule.Days.IsNull() && !rule.Days.IsUnknown() {
		v := int(rule.Days.ValueInt64())
		out.Days = &v
	}
	if !rule.Years.IsNull() && !rule.Years.IsUnknown() {
		v := int(rule.Years.ValueInt64())
		out.Years = &v
	}
	return out
}

// retentionToObject converts the API rule to the Terraform object; null when there is none.
func retentionToObject(rule *client.ObjectStorageLockRetention) types.Object {
	if rule == nil {
		return types.ObjectNull(lockRetentionAttrTypes)
	}
	toInt := func(p *int) types.Int64 {
		if p == nil {
			return types.Int64Null()
		}
		return types.Int64Value(int64(*p))
	}
	return types.ObjectValueMust(lockRetentionAttrTypes, map[string]attr.Value{
		"mode":  types.StringValue(rule.Mode),
		"days":  toInt(rule.Days),
		"years": toInt(rule.Years),
	})
}

// Bucket tag rules, the same the API applies.
const (
	bucketTagsMax   = 50
	bucketTagKeyMax = 128 // UTF-16 code units
	bucketTagValMax = 256 // UTF-16 code units
)

var reservedTagPrefixes = []string{"aws:", "cp:", "cubepath:"}

// validTagChars reports whether s only has letters, numbers, spaces and _ . : / = + - @.
func validTagChars(s string) bool {
	for _, c := range s {
		if unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.In(c, unicode.Z) || strings.ContainsRune("_.:/=+-@", c) {
			continue
		}
		return false
	}
	return true
}

// validateBucketTags returns one message per broken rule. A nil value is unknown at plan time
// and only its key is checked.
func validateBucketTags(tags map[string]*string) []string {
	var errs []string
	if len(tags) > bucketTagsMax {
		errs = append(errs, fmt.Sprintf("A bucket can have at most %d tags, got %d.", bucketTagsMax, len(tags)))
	}
	for k, v := range tags {
		switch n := len(utf16.Encode([]rune(k))); {
		case n == 0:
			errs = append(errs, "Tag keys cannot be empty.")
			continue
		case n > bucketTagKeyMax:
			errs = append(errs, fmt.Sprintf("Tag key %q is longer than %d characters.", k, bucketTagKeyMax))
		}
		if !validTagChars(k) {
			errs = append(errs, fmt.Sprintf("Tag key %q has characters other than letters, numbers, spaces and _ . : / = + - @.", k))
		}
		if strings.Contains(k, "=") {
			errs = append(errs, fmt.Sprintf("Tag key %q cannot contain =.", k))
		}
		if k != strings.TrimSpace(k) {
			errs = append(errs, fmt.Sprintf("Tag key %q cannot start or end with a space.", k))
		}
		lower := strings.ToLower(k)
		for _, p := range reservedTagPrefixes {
			if strings.HasPrefix(lower, p) {
				errs = append(errs, fmt.Sprintf("Tag key %q uses the reserved prefix %s.", k, p))
			}
		}
		if v == nil {
			continue
		}
		if len(utf16.Encode([]rune(*v))) > bucketTagValMax {
			errs = append(errs, fmt.Sprintf("The value of tag %q is longer than %d characters.", k, bucketTagValMax))
		}
		if !validTagChars(*v) {
			errs = append(errs, fmt.Sprintf("The value of tag %q has characters other than letters, numbers, spaces and _ . : / = + - @.", k))
		}
	}
	sort.Strings(errs)
	return errs
}

// tagsFromModel reads a known tags map from the plan.
func tagsFromModel(ctx context.Context, m types.Map, diags *diag.Diagnostics) map[string]string {
	tags := map[string]string{}
	if m.IsNull() || m.IsUnknown() {
		return tags
	}
	diags.Append(m.ElementsAs(ctx, &tags, false)...)
	return tags
}

func (r *objectStorageBucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan objectStorageBucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	versioning := plan.Versioning.ValueString()
	if versioning == "suspended" {
		resp.Diagnostics.AddAttributeError(path.Root("versioning"), "Invalid versioning for a new bucket",
			"A new bucket starts with versioning off or enabled. Create it enabled and change it to suspended afterwards.")
		return
	}

	createReq := &client.CreateObjectStorageBucketRequest{
		Name:       plan.Name.ValueString(),
		Tier:       plan.Tier.ValueString(),
		Versioning: versioning == "enabled",
	}
	if plan.ObjectLockEnabled.ValueBool() {
		// ValidateConfig already requires versioning enabled; never send false with the lock.
		createReq.Versioning = true
		createReq.ObjectLock = true
		createReq.AcceptObjectLockTerms = plan.AcceptObjectLockTerms.ValueBool()
		createReq.ObjectLockDefault = retentionFromModel(ctx, plan.ObjectLockDefaultRetention, &resp.Diagnostics)
	}
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
	}
	if tags := tagsFromModel(ctx, plan.Tags, &resp.Diagnostics); len(tags) > 0 {
		createReq.Tags = tags
	}
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.ObjectStorage.CreateBucket(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage bucket", err.Error())
		return
	}

	// Save the ID right away so a failed wait does not leave an untracked bucket behind.
	plan.ID = types.StringValue(created.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	bucket, err := r.waitForBucket(ctx, created.UUID, []string{"pending"}, []string{"active"}, bucketCreateTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the bucket to become active", err.Error())
		return
	}

	// A bucket with Object Lock is created protected; align it with the plan either way.
	if want := plan.Protected.ValueBool(); bucket.Protected != want {
		if err := r.client.ObjectStorage.UpdateBucket(ctx, created.UUID, &client.UpdateObjectStorageBucketRequest{Protected: &want}); err != nil {
			resp.Diagnostics.AddError("Error changing deletion protection", err.Error())
			return
		}
		bucket.Protected = want
	}

	r.mapToState(&plan, bucket)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageBucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state objectStorageBucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bucket, err := r.client.ObjectStorage.GetBucket(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage bucket", err.Error())
		return
	}

	r.mapToState(&state, bucket)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *objectStorageBucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state objectStorageBucketResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &client.UpdateObjectStorageBucketRequest{}
	changed := false
	if !plan.Versioning.Equal(state.Versioning) {
		v := plan.Versioning.ValueString()
		updateReq.Versioning = &v
		changed = true
	}
	if !plan.Protected.Equal(state.Protected) {
		v := plan.Protected.ValueBool()
		updateReq.Protected = &v
		changed = true
	}
	if !plan.Tags.IsUnknown() && !plan.Tags.Equal(state.Tags) {
		tags := tagsFromModel(ctx, plan.Tags, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Tags = &tags
		changed = true
	}

	if changed {
		if err := r.client.ObjectStorage.UpdateBucket(ctx, state.ID.ValueString(), updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating Object Storage bucket", err.Error())
			return
		}
	}

	if plan.ObjectLockEnabled.ValueBool() && !plan.ObjectLockDefaultRetention.Equal(state.ObjectLockDefaultRetention) {
		lockReq := &client.SetObjectStorageObjectLockRequest{
			DefaultRetention:      retentionFromModel(ctx, plan.ObjectLockDefaultRetention, &resp.Diagnostics),
			AcceptObjectLockTerms: plan.AcceptObjectLockTerms.ValueBool(),
		}
		if resp.Diagnostics.HasError() {
			return
		}
		if err := r.client.ObjectStorage.SetBucketObjectLock(ctx, state.ID.ValueString(), lockReq); err != nil {
			resp.Diagnostics.AddError("Error changing the default retention", err.Error())
			return
		}
	}

	bucket, err := r.client.ObjectStorage.GetBucket(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading Object Storage bucket", err.Error())
		return
	}
	r.mapToState(&plan, bucket)
	// The API applies a versioning change asynchronously; keep the requested value.
	if updateReq.Versioning != nil {
		plan.Versioning = types.StringValue(*updateReq.Versioning)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageBucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state objectStorageBucketResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	if state.ObjectLockEnabled.ValueBool() {
		resp.Diagnostics.AddWarning("Bucket with Object Lock",
			"Object versions still under compliance retention, under governance retention (unless force_destroy "+
				"and bypass_governance_on_destroy are set) or under a legal hold cannot be deleted. If any are left, "+
				"the bucket is kept with locked_content_kept = true, keeps being billed, and the destroy fails until "+
				"their retention ends.")
	}

	// A CDN origin removed in the same run releases the bucket a few seconds later, and a
	// bucket still finishing another operation answers 409: retry both for a while.
	deadline := time.Now().Add(3 * time.Minute)
	for {
		force := state.ForceDestroy.ValueBool()
		bypass := force && state.ObjectLockEnabled.ValueBool() && state.BypassGovernanceOnDestroy.ValueBool()
		err := r.client.ObjectStorage.DeleteBucket(ctx, id, force, bypass)
		if err == nil {
			break
		}
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			if apiErr.IsNotFound() {
				return
			}
			retryable := apiErr.IsConflict() ||
				(apiErr.IsBadRequest() && strings.Contains(apiErr.Detail, "CDN origin"))
			if retryable && time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					resp.Diagnostics.AddError("Error deleting Object Storage bucket", ctx.Err().Error())
					return
				case <-time.After(10 * time.Second):
				}
				continue
			}
		}
		resp.Diagnostics.AddError("Error deleting Object Storage bucket", err.Error())
		return
	}

	conf := &utils.StateChangeConf{
		Pending:      []string{"deleting"},
		Target:       []string{"deleted"},
		Timeout:      bucketDeleteTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			bucket, err := r.client.ObjectStorage.GetBucket(ctx, id)
			if err != nil {
				var apiErr *client.APIError
				if errors.As(err, &apiErr) && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			if bucket.Status != "deleting" && bucket.ErrorMessage != nil && *bucket.ErrorMessage != "" {
				return nil, "", fmt.Errorf("the bucket was not deleted: %s", *bucket.ErrorMessage)
			}
			return bucket, bucket.Status, nil
		},
	}
	if _, err := conf.WaitForState(ctx); err != nil {
		resp.Diagnostics.AddError("Error waiting for the bucket to be deleted", err.Error())
	}
}

func (r *objectStorageBucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	bucket, err := r.client.ObjectStorage.GetBucket(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Object Storage bucket", err.Error())
		return
	}
	var state objectStorageBucketResourceModel
	state.Tier = types.StringValue(bucket.Tier.Slug)
	state.ForceDestroy = types.BoolValue(false)
	state.BypassGovernanceOnDestroy = types.BoolValue(false)
	state.AcceptObjectLockTerms = types.BoolNull()
	r.mapToState(&state, bucket)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// waitForBucket polls a bucket until it reaches a target status. An error status fails with
// the bucket's error message.
func (r *objectStorageBucketResource) waitForBucket(ctx context.Context, id string, pending, target []string, timeout time.Duration) (*client.ObjectStorageBucket, error) {
	conf := &utils.StateChangeConf{
		Pending:      pending,
		Target:       target,
		Timeout:      timeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			bucket, err := r.client.ObjectStorage.GetBucket(ctx, id)
			if err != nil {
				return nil, "", err
			}
			if bucket.Status == "error" {
				msg := "unknown error"
				if bucket.ErrorMessage != nil && *bucket.ErrorMessage != "" {
					msg = *bucket.ErrorMessage
				}
				return nil, "", fmt.Errorf("the bucket ended in error: %s", msg)
			}
			return bucket, bucket.Status, nil
		},
	}
	result, err := conf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*client.ObjectStorageBucket), nil
}

func (r *objectStorageBucketResource) mapToState(state *objectStorageBucketResourceModel, bucket *client.ObjectStorageBucket) {
	state.ID = types.StringValue(bucket.UUID)
	state.Name = types.StringValue(bucket.Name)
	if bucket.ProjectID != nil {
		state.ProjectID = types.Int64Value(int64(*bucket.ProjectID))
	} else {
		state.ProjectID = types.Int64Null()
	}
	// The tier keeps the value from the configuration (slug or UUID); only fill it when missing.
	if state.Tier.IsNull() || state.Tier.IsUnknown() || state.Tier.ValueString() == "" {
		state.Tier = types.StringValue(bucket.Tier.Slug)
	}
	state.Versioning = types.StringValue(bucket.Versioning)
	state.Protected = types.BoolValue(bucket.Protected)
	if state.ForceDestroy.IsNull() || state.ForceDestroy.IsUnknown() {
		state.ForceDestroy = types.BoolValue(false)
	}
	state.Status = types.StringValue(bucket.Status)
	state.Region = types.StringValue(bucket.Region)
	state.Endpoint = types.StringValue(bucket.Endpoint)
	state.LocationName = types.StringValue(bucket.LocationName)
	state.Tags = tagsToMap(bucket.Tags)
	state.ObjectLockEnabled = types.BoolValue(bucket.ObjectLock.Enabled)
	state.ObjectLockDefaultRetention = retentionToObject(bucket.ObjectLock.DefaultRetention)
	state.LockedContentKept = types.BoolValue(bucket.LockedContentKept)
	if state.BypassGovernanceOnDestroy.IsNull() || state.BypassGovernanceOnDestroy.IsUnknown() {
		state.BypassGovernanceOnDestroy = types.BoolValue(false)
	}
	// accept_object_lock_terms is never returned: it keeps the configured value.
}

// tagsToMap converts API tags to a Terraform map; no tags is an empty map, never null.
func tagsToMap(tags map[string]string) types.Map {
	elems := make(map[string]attr.Value, len(tags))
	for k, v := range tags {
		elems[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, elems)
}
