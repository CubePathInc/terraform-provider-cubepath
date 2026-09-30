package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &objectStorageAccessKeyResource{}
	_ resource.ResourceWithConfigure      = &objectStorageAccessKeyResource{}
	_ resource.ResourceWithImportState    = &objectStorageAccessKeyResource{}
	_ resource.ResourceWithValidateConfig = &objectStorageAccessKeyResource{}
)

const accessKeyTimeout = 10 * time.Minute

func NewObjectStorageAccessKeyResource() resource.Resource {
	return &objectStorageAccessKeyResource{}
}

type objectStorageAccessKeyResource struct {
	client *client.Client
}

type objectStorageAccessKeyResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Tier            types.String `tfsdk:"tier"`
	ProjectID       types.Int64  `tfsdk:"project_id"`
	Permission      types.String `tfsdk:"permission"`
	Buckets         types.List   `tfsdk:"buckets"`
	ExpiresAt       types.String `tfsdk:"expires_at"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	Status          types.String `tfsdk:"status"`
	Region          types.String `tfsdk:"region"`
	Endpoint        types.String `tfsdk:"endpoint"`
}

func (r *objectStorageAccessKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_access_key"
}

func (r *objectStorageAccessKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CubePath Object Storage access key for S3 clients (AWS CLI, rclone, SDKs). " +
			"The secret is only returned when the key is created and is kept in the Terraform state " +
			"(marked sensitive): protect your state. Import with the key UUID " +
			"(terraform import cubepath_object_storage_access_key.example <uuid>); an imported key has no secret. " +
			"Every attribute forces a new key when changed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the access key.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description:   "Key name (1 to 100 characters).",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"tier": schema.StringAttribute{
				Description:   "Storage tier, by slug (for example infrequent_access) or UUID.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"project_id": schema.Int64Attribute{
				Description: "Project ID. Defaults to the organization's first project.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			"permission": schema.StringAttribute{
				Description:   "read_write or read_only. Defaults to read_write.",
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("read_write"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"buckets": schema.ListAttribute{
				Description: "UUIDs of the buckets the key is limited to (1 to 20, same project and tier). " +
					"Omit it to give the key access to every bucket of the project in the tier, present and future.",
				ElementType:   types.StringType,
				Optional:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"expires_at": schema.StringAttribute{
				Description:   "Optional expiry time in RFC 3339 (for example 2027-01-01T00:00:00Z). The key stops working then.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"access_key_id": schema.StringAttribute{
				Description:   "The access key ID to configure in S3 clients.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"secret_access_key": schema.StringAttribute{
				Description:   "The secret access key. Only known for keys created by Terraform; null after an import.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Description: "Key status: pending, active, suspended, error or deleting.",
				Computed:    true,
			},
			"region": schema.StringAttribute{
				Description:   "S3 region to configure in clients.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint": schema.StringAttribute{
				Description:   "S3 endpoint to configure in clients.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *objectStorageAccessKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *objectStorageAccessKeyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var permission, expiresAt types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("permission"), &permission)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("expires_at"), &expiresAt)...)
	if !permission.IsNull() && !permission.IsUnknown() {
		if v := permission.ValueString(); v != "read_write" && v != "read_only" {
			resp.Diagnostics.AddAttributeError(path.Root("permission"), "Invalid permission",
				"permission must be read_write or read_only.")
		}
	}
	if !expiresAt.IsNull() && !expiresAt.IsUnknown() {
		if _, err := time.Parse(time.RFC3339, expiresAt.ValueString()); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid expires_at",
				"expires_at must be an RFC 3339 time, for example 2027-01-01T00:00:00Z.")
		}
	}
}

func (r *objectStorageAccessKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan objectStorageAccessKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &client.CreateObjectStorageKeyRequest{
		Name:       plan.Name.ValueString(),
		Tier:       plan.Tier.ValueString(),
		Permission: plan.Permission.ValueString(),
	}
	if !plan.ProjectID.IsNull() && !plan.ProjectID.IsUnknown() {
		pid := int(plan.ProjectID.ValueInt64())
		createReq.ProjectID = &pid
	}
	if !plan.Buckets.IsNull() && !plan.Buckets.IsUnknown() {
		var buckets []string
		resp.Diagnostics.Append(plan.Buckets.ElementsAs(ctx, &buckets, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.BucketUUIDs = buckets
	}
	if !plan.ExpiresAt.IsNull() && !plan.ExpiresAt.IsUnknown() {
		v := plan.ExpiresAt.ValueString()
		createReq.ExpiresAt = &v
	}

	created, err := r.client.ObjectStorage.CreateKey(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage access key", err.Error())
		return
	}

	// The secret is never returned again: store it before waiting.
	plan.ID = types.StringValue(created.UUID)
	plan.AccessKeyID = types.StringValue(created.AccessKeyID)
	plan.SecretAccessKey = types.StringValue(created.SecretAccessKey)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("access_key_id"), plan.AccessKeyID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("secret_access_key"), plan.SecretAccessKey)...)

	conf := &utils.StateChangeConf{
		Pending:      []string{"pending"},
		Target:       []string{"active"},
		Timeout:      accessKeyTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			key, err := r.client.ObjectStorage.GetKey(ctx, created.UUID)
			if err != nil {
				return nil, "", err
			}
			if key.Status == "error" {
				return nil, "", fmt.Errorf("the access key ended in error")
			}
			return key, key.Status, nil
		},
	}
	result, err := conf.WaitForState(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the access key to become active", err.Error())
		return
	}

	r.mapToState(&plan, result.(*client.ObjectStorageAccessKey))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageAccessKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state objectStorageAccessKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, err := r.client.ObjectStorage.GetKey(ctx, state.ID.ValueString())
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage access key", err.Error())
		return
	}
	if key.Status == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}

	r.mapToState(&state, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called with a real change: every configurable attribute forces a new key.
func (r *objectStorageAccessKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan objectStorageAccessKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageAccessKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state objectStorageAccessKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	if err := r.client.ObjectStorage.DeleteKey(ctx, id); err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.IsNotFound() {
			return
		}
		// 409: already being deleted, just wait for it.
		if !(errors.As(err, &apiErr) && apiErr.IsConflict()) {
			resp.Diagnostics.AddError("Error deleting Object Storage access key", err.Error())
			return
		}
	}

	conf := &utils.StateChangeConf{
		Pending:      []string{"deleting"},
		Target:       []string{"deleted"},
		Timeout:      accessKeyTimeout,
		PollInterval: bucketPollInterval,
		Refresh: func() (interface{}, string, error) {
			key, err := r.client.ObjectStorage.GetKey(ctx, id)
			if err != nil {
				var apiErr *client.APIError
				if errors.As(err, &apiErr) && apiErr.IsNotFound() {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			return key, key.Status, nil
		},
	}
	if _, err := conf.WaitForState(ctx); err != nil {
		resp.Diagnostics.AddError("Error waiting for the access key to be deleted", err.Error())
	}
}

func (r *objectStorageAccessKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	key, err := r.client.ObjectStorage.GetKey(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Object Storage access key", err.Error())
		return
	}
	state := objectStorageAccessKeyResourceModel{
		Tier:            types.StringValue(key.Tier.Slug),
		Buckets:         types.ListNull(types.StringType),
		ExpiresAt:       types.StringNull(),
		SecretAccessKey: types.StringNull(),
	}
	if len(key.BucketScope) > 0 {
		uuids := make([]string, 0, len(key.BucketScope))
		for _, b := range key.BucketScope {
			uuids = append(uuids, b.UUID)
		}
		list, diags := types.ListValueFrom(ctx, types.StringType, uuids)
		resp.Diagnostics.Append(diags...)
		state.Buckets = list
	}
	if key.ExpiresAt != nil && *key.ExpiresAt != "" {
		state.ExpiresAt = types.StringValue(*key.ExpiresAt)
	}
	r.mapToState(&state, key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// mapToState copies the API fields. The tier, buckets and expires_at keep the configured values
// (the API echoes them in another shape); the secret is never returned after the create.
func (r *objectStorageAccessKeyResource) mapToState(state *objectStorageAccessKeyResourceModel, key *client.ObjectStorageAccessKey) {
	state.ID = types.StringValue(key.UUID)
	state.Name = types.StringValue(key.Name)
	state.Permission = types.StringValue(key.Permission)
	state.AccessKeyID = types.StringValue(key.AccessKeyID)
	if key.ProjectID != nil {
		state.ProjectID = types.Int64Value(int64(*key.ProjectID))
	} else {
		state.ProjectID = types.Int64Null()
	}
	if state.Tier.IsNull() || state.Tier.IsUnknown() || state.Tier.ValueString() == "" {
		state.Tier = types.StringValue(key.Tier.Slug)
	}
	state.Status = types.StringValue(key.Status)
	state.Region = types.StringValue(key.Region)
	state.Endpoint = types.StringValue(key.Endpoint)
	if state.SecretAccessKey.IsUnknown() {
		state.SecretAccessKey = types.StringNull()
	}
}
