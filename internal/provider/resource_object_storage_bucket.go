package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &objectStorageBucketResource{}
	_ resource.ResourceWithConfigure      = &objectStorageBucketResource{}
	_ resource.ResourceWithImportState    = &objectStorageBucketResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageBucketResource{}
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
			"force_destroy is not stored by the API and imports as false.",
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
			"force_destroy": schema.BoolAttribute{
				Description: "Delete every object, version and unfinished upload when the bucket is destroyed. " +
					"Without it, destroying a bucket that is not empty fails.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
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
	var versioning types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("versioning"), &versioning)...)
	if versioning.IsNull() || versioning.IsUnknown() {
		return
	}
	switch versioning.ValueString() {
	case "off", "enabled", "suspended":
	default:
		resp.Diagnostics.AddAttributeError(path.Root("versioning"), "Invalid versioning",
			"versioning must be off, enabled or suspended.")
	}
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
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
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

	if plan.Protected.ValueBool() {
		protected := true
		if err := r.client.ObjectStorage.UpdateBucket(ctx, created.UUID, &client.UpdateObjectStorageBucketRequest{Protected: &protected}); err != nil {
			resp.Diagnostics.AddError("Error enabling deletion protection", err.Error())
			return
		}
		bucket.Protected = true
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

	if changed {
		if err := r.client.ObjectStorage.UpdateBucket(ctx, state.ID.ValueString(), updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating Object Storage bucket", err.Error())
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

	// A CDN origin removed in the same run releases the bucket a few seconds later, and a
	// bucket still finishing another operation answers 409: retry both for a while.
	deadline := time.Now().Add(3 * time.Minute)
	for {
		err := r.client.ObjectStorage.DeleteBucket(ctx, id, state.ForceDestroy.ValueBool())
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
}
