package provider

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &objectStorageReplicationResource{}
	_ resource.ResourceWithConfigure      = &objectStorageReplicationResource{}
	_ resource.ResourceWithImportState    = &objectStorageReplicationResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageReplicationResource{}
)

const (
	replicationCreateTimeout = 20 * time.Minute
	replicationBusyTimeout   = 10 * time.Minute
)

var (
	replicationRegionRe = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
	replicationBucketRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)
)

func NewObjectStorageReplicationResource() resource.Resource {
	return &objectStorageReplicationResource{}
}

type objectStorageReplicationResource struct {
	client *client.Client
}

type replicationResourceModel struct {
	ID               types.String                 `tfsdk:"id"`
	SourceBucketUUID types.String                 `tfsdk:"source_bucket_uuid"`
	Enabled          types.Bool                   `tfsdk:"enabled"`
	Destination      *replicationDestinationModel `tfsdk:"destination"`
	Rule             *replicationRuleModel        `tfsdk:"rule"`
	Status           types.String                 `tfsdk:"status"`
	PauseReason      types.String                 `tfsdk:"pause_reason"`
	Health           types.String                 `tfsdk:"health"`
	HealthReason     types.String                 `tfsdk:"health_reason"`
	BackfillStatus   types.String                 `tfsdk:"backfill_status"`
	ErrorMessage     types.String                 `tfsdk:"error_message"`
}

type replicationDestinationModel struct {
	Type                   types.String `tfsdk:"type"`
	BucketUUID             types.String `tfsdk:"bucket_uuid"`
	GrantToken             types.String `tfsdk:"grant_token"`
	Provider               types.String `tfsdk:"provider"`
	Endpoint               types.String `tfsdk:"endpoint"`
	Region                 types.String `tfsdk:"region"`
	Bucket                 types.String `tfsdk:"bucket"`
	PathStyle              types.String `tfsdk:"path_style"`
	AccessKeyID            types.String `tfsdk:"access_key_id"`
	SecretAccessKey        types.String `tfsdk:"secret_access_key"`
	SecretAccessKeyVersion types.Int64  `tfsdk:"secret_access_key_version"`
}

type replicationRuleModel struct {
	Prefix                  types.String `tfsdk:"prefix"`
	Tags                    types.Map    `tfsdk:"tags"`
	DeleteMarkerReplication types.Bool   `tfsdk:"delete_marker_replication"`
	DeleteReplication       types.Bool   `tfsdk:"delete_replication"`
	ExistingObjects         types.Bool   `tfsdk:"existing_objects"`
}

// replicationRules is the comparable form of the rules (a missing rule block means the API defaults).
type replicationRules struct {
	Prefix                  *string
	Tags                    []client.ObjectStorageLifecycleTag
	DeleteMarkerReplication bool
	DeleteReplication       bool
	ExistingObjects         bool
}

func (r *objectStorageReplicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_replication"
}

func (r *objectStorageReplicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Replicates a CubePath Object Storage bucket, object by object and asynchronously, to one destination: " +
			"another CubePath bucket of the same tier (of your organization, or of another one that gives you a grant, " +
			"see cubepath_object_storage_replication_grant) or an external S3 compatible bucket (AWS S3, Wasabi or another " +
			"provider with versioning) over HTTPS on port 443. Versioning must be enabled on the source and on a CubePath " +
			"destination; buckets with Object Lock cannot be sources. A CubePath destination lives on the same storage " +
			"cluster as the source, so it is not a disaster recovery copy: use an external destination for an off site copy. " +
			"Replication to an external destination is billed as egress of the source bucket. The external secret is " +
			"sent to the API and kept in the Terraform state, marked sensitive (it is never read back from the API): " +
			"protect your state. Changing source_bucket_uuid or the identity of the destination (type, bucket_uuid, " +
			"endpoint, region, bucket, provider, path_style) forces a new replication; rule, enabled and the external " +
			"credentials are changed in place. Destroying it stops the replication; the data already replicated stays " +
			"in the destination. Import with the replication UUID; an imported external destination has no credentials " +
			"until you set them, and the next apply sends them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the replication.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"source_bucket_uuid": schema.StringAttribute{
				Description:   "UUID of the source bucket (cubepath_object_storage_bucket.<name>.id). A bucket has at most one replication.",
				Required:      true,
				PlanModifiers: replace,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the replication runs. false pauses it (new writes are not replicated while paused). Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"status": schema.StringAttribute{
				Description: "Replication status: pending, active, paused (see pause_reason), suspended, error (see error_message) or deleting.",
				Computed:    true,
			},
			"pause_reason": schema.StringAttribute{
				Description: "Why it is paused: customer, org, abuse, admin, source_blocked or dest_revoked (the destination owner revoked it: destroy it).",
				Computed:    true,
			},
			"health": schema.StringAttribute{
				Description: "unknown, ok, lagging or failing, measured every 15 minutes.",
				Computed:    true,
			},
			"health_reason": schema.StringAttribute{
				Description: "Reason of a failing health, for example credentials rejected or endpoint unreachable.",
				Computed:    true,
			},
			"backfill_status": schema.StringAttribute{
				Description: "Copy of the objects the source already held: none, queued, running, completed or failed.",
				Computed:    true,
			},
			"error_message": schema.StringAttribute{
				Description: "Why the configuration failed, when status is error.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"destination": schema.SingleNestedBlock{
				Description: "Where to replicate (required).",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Description:   "cubepath or external.",
						Optional:      true,
						Validators:    []validator.String{StringOneOf("cubepath", "external")},
						PlanModifiers: replace,
					},
					"bucket_uuid": schema.StringAttribute{
						Description:   "cubepath: UUID of the destination bucket, same tier as the source, with versioning enabled.",
						Optional:      true,
						PlanModifiers: replace,
					},
					"grant_token": schema.StringAttribute{
						Description: "cubepath: grant token of the destination owner, required when the destination bucket belongs " +
							"to another organization. Only used when the replication is created (a grant is used once).",
						Optional:  true,
						Sensitive: true,
					},
					"provider": schema.StringAttribute{
						Description:   "external: aws, wasabi or other (informative). Defaults to other.",
						Optional:      true,
						Validators:    []validator.String{StringOneOf("aws", "wasabi", "other")},
						PlanModifiers: replace,
					},
					"endpoint": schema.StringAttribute{
						Description: "external: public HTTPS host of the provider, without scheme or path, optionally with :443 " +
							"(for example s3.eu-west-1.amazonaws.com). Only port 443 is allowed.",
						Optional:      true,
						PlanModifiers: replace,
					},
					"region": schema.StringAttribute{
						Description:   "external: region of the destination bucket (lowercase letters, numbers and hyphens).",
						Optional:      true,
						PlanModifiers: replace,
					},
					"bucket": schema.StringAttribute{
						Description:   "external: name of the destination bucket, which must have versioning enabled.",
						Optional:      true,
						PlanModifiers: replace,
					},
					"path_style": schema.StringAttribute{
						Description:   "external: auto, on or off. Defaults to auto.",
						Optional:      true,
						Validators:    []validator.String{StringOneOf("auto", "on", "off")},
						PlanModifiers: replace,
					},
					"access_key_id": schema.StringAttribute{
						Description: "external: access key id at the provider. Changing it rotates the credentials in place.",
						Optional:    true,
					},
					"secret_access_key": schema.StringAttribute{
						Description: "external: secret access key at the provider. Kept in the state, marked sensitive, and never " +
							"read back from the API. Changing it rotates the credentials in place.",
						Optional:  true,
						Sensitive: true,
					},
					"secret_access_key_version": schema.Int64Attribute{
						Description: "external: change this number to send the credentials again without changing them " +
							"(for example after fixing them at the provider).",
						Optional: true,
					},
				},
			},
			"rule": schema.SingleNestedBlock{
				Description: "What to replicate. Without this block every new object version is replicated, plus the objects " +
					"the bucket already holds, but no deletes.",
				Attributes: map[string]schema.Attribute{
					"prefix": schema.StringAttribute{
						Description: "Only objects under this prefix (1 to 1024 characters). Not with tags.",
						Optional:    true,
					},
					"tags": schema.MapAttribute{
						Description: "Only objects carrying all these tags (1 to 10). Not with prefix nor delete_marker_replication.",
						ElementType: types.StringType,
						Optional:    true,
					},
					"delete_marker_replication": schema.BoolAttribute{
						Description: "Replicate delete markers. Defaults to false.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"delete_replication": schema.BoolAttribute{
						Description: "Replicate deletes of a specific version. Defaults to false.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(false),
					},
					"existing_objects": schema.BoolAttribute{
						Description: "Copy the objects the bucket already holds when the replication starts. Defaults to true.",
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(true),
					},
				},
			},
		},
	}
}

func (r *objectStorageReplicationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *objectStorageReplicationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config replicationResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateReplication(ctx, config.Destination, config.Rule)...)
}

// isSet reports whether a value is present in the configuration (unknown counts as present).
func isSet(v interface {
	IsNull() bool
}) bool {
	return !v.IsNull()
}

// validateReplication checks what can be checked without the API (unknown values are skipped).
func validateReplication(ctx context.Context, dest *replicationDestinationModel, rule *replicationRuleModel) diag.Diagnostics {
	var diags diag.Diagnostics
	at := path.Root("destination")
	if dest == nil || dest.Type.IsNull() {
		diags.AddAttributeError(at, "Missing destination", "A destination block with type cubepath or external is required.")
		return diags
	}
	cubepathFields := map[string]bool{"bucket_uuid": isSet(dest.BucketUUID), "grant_token": isSet(dest.GrantToken)}
	externalFields := map[string]bool{
		"provider": isSet(dest.Provider), "endpoint": isSet(dest.Endpoint), "region": isSet(dest.Region), "bucket": isSet(dest.Bucket),
		"path_style": isSet(dest.PathStyle), "access_key_id": isSet(dest.AccessKeyID), "secret_access_key": isSet(dest.SecretAccessKey),
		"secret_access_key_version": isSet(dest.SecretAccessKeyVersion),
	}
	checkFields := func(required []string, present, foreign map[string]bool) {
		for _, f := range required {
			if !present[f] {
				diags.AddAttributeError(at.AtName(f), "Missing destination field",
					fmt.Sprintf("destination.%s is required for a %s destination.", f, dest.Type.ValueString()))
			}
		}
		for f, ok := range foreign {
			if ok {
				diags.AddAttributeError(at.AtName(f), "Invalid destination field",
					fmt.Sprintf("destination.%s does not apply to a %s destination.", f, dest.Type.ValueString()))
			}
		}
	}
	switch dest.Type.ValueString() {
	case "cubepath":
		checkFields([]string{"bucket_uuid"}, cubepathFields, externalFields)
	case "external":
		checkFields([]string{"endpoint", "region", "bucket", "access_key_id", "secret_access_key"}, externalFields, cubepathFields)
		if v := knownString(dest.Endpoint); v != nil {
			if _, err := normalizeReplicationEndpoint(*v); err != nil {
				diags.AddAttributeError(at.AtName("endpoint"), "Invalid endpoint", err.Error())
			}
		}
		if v := knownString(dest.Region); v != nil && !replicationRegionRe.MatchString(*v) {
			diags.AddAttributeError(at.AtName("region"), "Invalid region",
				"The destination region must be 1 to 64 lowercase letters, numbers and hyphens.")
		}
		if v := knownString(dest.Bucket); v != nil && (len(*v) < 3 || len(*v) > 63 || !replicationBucketRe.MatchString(*v)) {
			diags.AddAttributeError(at.AtName("bucket"), "Invalid bucket",
				"The destination bucket name must be 3 to 63 characters long, use only lowercase letters, numbers and "+
					"hyphens, and start and end with a letter or number.")
		}
	}

	if rule == nil {
		return diags
	}
	rat := path.Root("rule")
	if v := knownString(rule.Prefix); v != nil && (len(*v) < 1 || len(*v) > 1024) {
		diags.AddAttributeError(rat.AtName("prefix"), "Invalid prefix", "prefix must be 1 to 1024 characters.")
	}
	if isSet(rule.Tags) {
		if isSet(rule.Prefix) {
			diags.AddAttributeError(rat.AtName("tags"), "Invalid rule", "Filter by prefix or by tags, not both.")
		}
		if rule.DeleteMarkerReplication.ValueBool() {
			diags.AddAttributeError(rat.AtName("delete_marker_replication"), "Invalid rule",
				"Delete markers cannot be replicated with a tag filter.")
		}
		if !rule.Tags.IsUnknown() {
			var tags map[string]types.String
			diags.Append(rule.Tags.ElementsAs(ctx, &tags, false)...)
			if len(tags) < 1 || len(tags) > 10 {
				diags.AddAttributeError(rat.AtName("tags"), "Invalid tags", "tags must have 1 to 10 entries.")
			}
			for k, v := range tags {
				if len(k) < 1 || len(k) > 128 || len(v.ValueString()) > 256 {
					diags.AddAttributeError(rat.AtName("tags"), "Invalid tags",
						"Tag keys must be 1 to 128 characters and values at most 256.")
					break
				}
			}
		}
	}
	return diags
}

// normalizeReplicationEndpoint lowercases a host, checks it has no scheme, path or user and drops
// an explicit :443 (the only port allowed), like the API does.
func normalizeReplicationEndpoint(endpoint string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(endpoint))
	if e == "" || strings.Contains(e, "://") || strings.ContainsAny(e, "/@?# []") {
		return "", fmt.Errorf("endpoint must be a host name, without scheme, path or user (for example s3.eu-west-1.amazonaws.com)")
	}
	if i := strings.LastIndex(e, ":"); i >= 0 {
		if e[i+1:] != "443" {
			return "", fmt.Errorf("only HTTPS on port 443 is allowed: drop the port or use :443")
		}
		e = e[:i]
	}
	if !strings.Contains(e, ".") || len(e) > 253 {
		return "", fmt.Errorf("endpoint must be a DNS name with at least one dot")
	}
	return e, nil
}

// rulesFromModel turns the rule block into comparable rules; a missing block gives the API defaults.
func replicationRulesFromModel(ctx context.Context, rule *replicationRuleModel, diags *diag.Diagnostics) replicationRules {
	out := replicationRules{ExistingObjects: true}
	if rule == nil {
		return out
	}
	out.Prefix = knownString(rule.Prefix)
	if !rule.Tags.IsNull() && !rule.Tags.IsUnknown() {
		var tags map[string]string
		diags.Append(rule.Tags.ElementsAs(ctx, &tags, false)...)
		out.Tags = sortedReplicationTags(tags)
	}
	out.DeleteMarkerReplication = rule.DeleteMarkerReplication.ValueBool()
	out.DeleteReplication = rule.DeleteReplication.ValueBool()
	if !rule.ExistingObjects.IsNull() && !rule.ExistingObjects.IsUnknown() {
		out.ExistingObjects = rule.ExistingObjects.ValueBool()
	}
	return out
}

func sortedReplicationTags(tags map[string]string) []client.ObjectStorageLifecycleTag {
	if len(tags) == 0 {
		return nil
	}
	out := make([]client.ObjectStorageLifecycleTag, 0, len(tags))
	for k, v := range tags {
		out = append(out, client.ObjectStorageLifecycleTag{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func replicationRulesFromAPI(r client.ObjectStorageReplicationRules) replicationRules {
	out := replicationRules{
		DeleteMarkerReplication: r.DeleteMarkerReplication,
		DeleteReplication:       r.DeleteReplication,
		ExistingObjects:         r.ExistingObjects,
	}
	if r.Prefix != nil && *r.Prefix != "" {
		p := *r.Prefix
		out.Prefix = &p
	}
	if len(r.Tags) > 0 {
		m := map[string]string{}
		for _, t := range r.Tags {
			m[t.Key] = t.Value
		}
		out.Tags = sortedReplicationTags(m)
	}
	return out
}

func tagsEqual(a, b []client.ObjectStorageLifecycleTag) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (x replicationRules) equal(y replicationRules) bool {
	samePrefix := (x.Prefix == nil && y.Prefix == nil) || (x.Prefix != nil && y.Prefix != nil && *x.Prefix == *y.Prefix)
	return samePrefix && tagsEqual(x.Tags, y.Tags) && x.DeleteMarkerReplication == y.DeleteMarkerReplication &&
		x.DeleteReplication == y.DeleteReplication && x.ExistingObjects == y.ExistingObjects
}

// ruleModelFromAPI builds the rule block from the API rules.
func replicationRuleModelFromAPI(ctx context.Context, r replicationRules, diags *diag.Diagnostics) *replicationRuleModel {
	m := &replicationRuleModel{
		Prefix:                  stringOrNull(r.Prefix),
		Tags:                    types.MapNull(types.StringType),
		DeleteMarkerReplication: types.BoolValue(r.DeleteMarkerReplication),
		DeleteReplication:       types.BoolValue(r.DeleteReplication),
		ExistingObjects:         types.BoolValue(r.ExistingObjects),
	}
	if len(r.Tags) > 0 {
		tags := map[string]string{}
		for _, t := range r.Tags {
			tags[t.Key] = t.Value
		}
		v, d := types.MapValueFrom(ctx, types.StringType, tags)
		diags.Append(d...)
		m.Tags = v
	}
	return m
}

// buildReplicationCreate builds the POST body from the plan, with only the fields of the destination type.
func buildReplicationCreate(ctx context.Context, plan *replicationResourceModel, diags *diag.Diagnostics) *client.CreateObjectStorageReplicationRequest {
	rules := replicationRulesFromModel(ctx, plan.Rule, diags)
	d := plan.Destination
	dest := client.CreateObjectStorageReplicationDestination{Type: d.Type.ValueString()}
	if dest.Type == "cubepath" {
		dest.BucketUUID = d.BucketUUID.ValueString()
		dest.GrantToken = d.GrantToken.ValueString()
	} else {
		dest.Provider = d.Provider.ValueString()
		dest.Endpoint = d.Endpoint.ValueString()
		dest.Region = d.Region.ValueString()
		dest.Bucket = d.Bucket.ValueString()
		dest.PathStyle = d.PathStyle.ValueString()
		dest.AccessKeyID = d.AccessKeyID.ValueString()
		dest.SecretAccessKey = d.SecretAccessKey.ValueString()
	}
	return &client.CreateObjectStorageReplicationRequest{
		SourceBucketUUID:        plan.SourceBucketUUID.ValueString(),
		Destination:             dest,
		Prefix:                  rules.Prefix,
		Tags:                    rules.Tags,
		DeleteMarkerReplication: rules.DeleteMarkerReplication,
		DeleteReplication:       rules.DeleteReplication,
		ExistingObjects:         rules.ExistingObjects,
	}
}

// buildReplicationPatch returns the PATCH body for the differences between prior and plan, or nil
// when nothing changes. A removed prefix or tag filter is sent as null.
func buildReplicationPatch(ctx context.Context, prior, plan *replicationResourceModel, diags *diag.Diagnostics) map[string]interface{} {
	body := map[string]interface{}{}
	if plan.Enabled.ValueBool() != prior.Enabled.ValueBool() {
		body["enabled"] = plan.Enabled.ValueBool()
	}
	have := replicationRulesFromModel(ctx, prior.Rule, diags)
	want := replicationRulesFromModel(ctx, plan.Rule, diags)
	samePrefix := (have.Prefix == nil && want.Prefix == nil) || (have.Prefix != nil && want.Prefix != nil && *have.Prefix == *want.Prefix)
	if !samePrefix {
		if want.Prefix == nil {
			body["prefix"] = nil
		} else {
			body["prefix"] = *want.Prefix
		}
	}
	if !tagsEqual(have.Tags, want.Tags) {
		if want.Tags == nil {
			body["tags"] = nil
		} else {
			body["tags"] = want.Tags
		}
	}
	if have.DeleteMarkerReplication != want.DeleteMarkerReplication {
		body["delete_marker_replication"] = want.DeleteMarkerReplication
	}
	if have.DeleteReplication != want.DeleteReplication {
		body["delete_replication"] = want.DeleteReplication
	}
	if have.ExistingObjects != want.ExistingObjects {
		body["existing_objects"] = want.ExistingObjects
	}
	if plan.Destination != nil && plan.Destination.Type.ValueString() == "external" && prior.Destination != nil {
		p, n := prior.Destination, plan.Destination
		if !p.AccessKeyID.Equal(n.AccessKeyID) || !p.SecretAccessKey.Equal(n.SecretAccessKey) ||
			!p.SecretAccessKeyVersion.Equal(n.SecretAccessKeyVersion) {
			body["destination"] = map[string]string{
				"access_key_id":     n.AccessKeyID.ValueString(),
				"secret_access_key": n.SecretAccessKey.ValueString(),
			}
		}
	}
	if len(body) == 0 {
		return nil
	}
	return body
}

func (r *objectStorageReplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan replicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := buildReplicationCreate(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	created, err := r.client.ObjectStorage.CreateReplication(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage replication", err.Error())
		return
	}
	// Save the id first: if the wait fails the resource is tainted and can be destroyed.
	plan.ID = types.StringValue(created.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	repl, err := r.waitReady(ctx, created.UUID)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the replication to be configured", err.Error())
		return
	}

	if !plan.Enabled.ValueBool() {
		if err := r.patch(ctx, created.UUID, map[string]interface{}{"enabled": false}); err != nil {
			resp.Diagnostics.AddError("Error pausing the replication", err.Error())
			return
		}
		if repl, err = r.client.ObjectStorage.GetReplication(ctx, created.UUID); err != nil {
			resp.Diagnostics.AddError("Error reading Object Storage replication", err.Error())
			return
		}
	}

	r.mapToState(ctx, &plan, repl, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// waitReady waits until a new replication leaves pending.
func (r *objectStorageReplicationResource) waitReady(ctx context.Context, uuid string) (*client.ObjectStorageReplication, error) {
	conf := &utils.StateChangeConf{
		Pending:      []string{"pending"},
		Target:       []string{"active", "paused", "suspended"},
		Timeout:      replicationCreateTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			repl, err := r.client.ObjectStorage.GetReplication(ctx, uuid)
			if err != nil {
				return nil, "", err
			}
			if repl.Status == "error" {
				msg := "the replication could not be configured"
				if repl.ErrorMessage != nil && *repl.ErrorMessage != "" {
					msg = *repl.ErrorMessage
				}
				return nil, "", fmt.Errorf("%s", msg)
			}
			return repl, repl.Status, nil
		},
	}
	result, err := conf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*client.ObjectStorageReplication), nil
}

// patch sends a PATCH, retrying while the replication is busy with another operation (409).
func (r *objectStorageReplicationResource) patch(ctx context.Context, uuid string, body map[string]interface{}) error {
	return retryWhile(ctx, replicationBusyTimeout, bucketPollInterval, isReplicationBusy, func() error {
		return r.client.ObjectStorage.UpdateReplication(ctx, uuid, body)
	})
}

// isReplicationBusy reports a 409 that goes away by itself (another operation is running).
func isReplicationBusy(err error) bool {
	return isConflict(err) && strings.Contains(err.Error(), "busy")
}

func (r *objectStorageReplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state replicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	repl, err := r.client.ObjectStorage.GetReplication(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage replication", err.Error())
		return
	}
	if repl.Status == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}
	r.mapToState(ctx, &state, repl, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *objectStorageReplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, prior replicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := prior.ID.ValueString()
	plan.ID = prior.ID

	body := buildReplicationPatch(ctx, &prior, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if body != nil {
		if err := r.patch(ctx, id, body); err != nil {
			resp.Diagnostics.AddError("Error updating Object Storage replication", err.Error())
			return
		}
	}
	repl, err := r.client.ObjectStorage.GetReplication(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Object Storage replication", err.Error())
		return
	}
	r.mapToState(ctx, &plan, repl, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageReplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state replicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	err := retryWhile(ctx, replicationBusyTimeout, bucketPollInterval, isReplicationBusy, func() error {
		return r.client.ObjectStorage.DeleteReplication(ctx, id)
	})
	if err != nil {
		if isNotFound(err) {
			return
		}
		// Any other 409: it is already being removed, just wait for it.
		if !isConflict(err) {
			resp.Diagnostics.AddError("Error deleting Object Storage replication", err.Error())
			return
		}
	}

	conf := &utils.StateChangeConf{
		Pending:      []string{"deleting"},
		Target:       []string{"deleted"},
		Timeout:      replicationCreateTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			repl, err := r.client.ObjectStorage.GetReplication(ctx, id)
			if err != nil {
				if isNotFound(err) {
					return struct{}{}, "deleted", nil
				}
				return nil, "", err
			}
			return repl, repl.Status, nil
		},
	}
	if _, err := conf.WaitForState(ctx); err != nil {
		resp.Diagnostics.AddError("Error waiting for the replication to be removed", err.Error())
	}
}

func (r *objectStorageReplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	repl, err := r.client.ObjectStorage.GetReplication(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Object Storage replication", err.Error())
		return
	}
	state := replicationResourceModel{ID: types.StringValue(repl.UUID)}
	if repl.Source.BucketUUID != nil {
		state.SourceBucketUUID = types.StringValue(*repl.Source.BucketUUID)
	}
	dest := &replicationDestinationModel{
		Type:                   types.StringValue(repl.Destination.Type),
		BucketUUID:             types.StringNull(),
		GrantToken:             types.StringNull(),
		Provider:               types.StringNull(),
		Endpoint:               types.StringNull(),
		Region:                 types.StringNull(),
		Bucket:                 types.StringNull(),
		PathStyle:              types.StringNull(),
		AccessKeyID:            types.StringNull(),
		SecretAccessKey:        types.StringNull(),
		SecretAccessKeyVersion: types.Int64Null(),
	}
	// mapToState fills the identity fields; the credentials stay null until they are configured.
	state.Destination = dest
	rules := replicationRulesFromAPI(repl.Rules)
	if !rules.equal(replicationRules{ExistingObjects: true}) {
		state.Rule = replicationRuleModelFromAPI(ctx, rules, &resp.Diagnostics)
	}
	r.mapToState(ctx, &state, repl, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// mapToState copies the API fields. The destination keeps the configured values (the API masks
// the access key id and never returns the secret or the grant token) except the identity fields,
// so a change made outside Terraform shows up as a replacement. The rule block is only written when
// the API rules differ from what is configured (a missing block stands for the defaults).
func (r *objectStorageReplicationResource) mapToState(ctx context.Context, state *replicationResourceModel, repl *client.ObjectStorageReplication, diags *diag.Diagnostics) {
	state.ID = types.StringValue(repl.UUID)
	if repl.Source.BucketUUID != nil {
		state.SourceBucketUUID = types.StringValue(*repl.Source.BucketUUID)
	}
	state.Enabled = types.BoolValue(repl.Rules.Enabled)
	state.Status = types.StringValue(repl.Status)
	state.PauseReason = stringOrNull(repl.PauseReason)
	state.Health = types.StringValue(repl.Health)
	state.HealthReason = stringOrNull(repl.HealthReason)
	state.BackfillStatus = types.StringValue(repl.Backfill.Status)
	state.ErrorMessage = stringOrNull(repl.ErrorMessage)

	if d := state.Destination; d != nil {
		api := repl.Destination
		d.Type = types.StringValue(api.Type)
		if api.Type == "cubepath" {
			if api.BucketUUID != nil {
				d.BucketUUID = types.StringValue(*api.BucketUUID)
			}
		} else {
			// The API stores the endpoint lowercased and without :443: keep the configured spelling.
			if api.Endpoint != nil {
				configured, err := normalizeReplicationEndpoint(d.Endpoint.ValueString())
				if err != nil || configured != *api.Endpoint {
					d.Endpoint = types.StringValue(*api.Endpoint)
				}
			}
			if api.Region != nil {
				d.Region = types.StringValue(*api.Region)
			}
			if api.Bucket != nil {
				d.Bucket = types.StringValue(*api.Bucket)
			}
			// provider and path_style are optional: only overwrite them when they were set or the API
			// holds something else than the default.
			if api.Provider != nil && (!d.Provider.IsNull() || *api.Provider != "other") {
				d.Provider = types.StringValue(*api.Provider)
			}
			if api.PathStyle != nil && (!d.PathStyle.IsNull() || *api.PathStyle != "auto") {
				d.PathStyle = types.StringValue(*api.PathStyle)
			}
		}
	}

	apiRules := replicationRulesFromAPI(repl.Rules)
	if !replicationRulesFromModel(ctx, state.Rule, diags).equal(apiRules) {
		state.Rule = replicationRuleModelFromAPI(ctx, apiRules, diags)
	}
}
