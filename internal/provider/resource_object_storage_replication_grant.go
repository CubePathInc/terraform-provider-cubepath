package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = &objectStorageReplicationGrantResource{}
	_ resource.ResourceWithConfigure = &objectStorageReplicationGrantResource{}
)

func NewObjectStorageReplicationGrantResource() resource.Resource {
	return &objectStorageReplicationGrantResource{}
}

type objectStorageReplicationGrantResource struct {
	client *client.Client
}

type replicationGrantResourceModel struct {
	ID            types.String `tfsdk:"id"`
	BucketUUID    types.String `tfsdk:"bucket_uuid"`
	Note          types.String `tfsdk:"note"`
	ExpiresInDays types.Int64  `tfsdk:"expires_in_days"`
	Token         types.String `tfsdk:"token"`
	TokenPrefix   types.String `tfsdk:"token_prefix"`
	Status        types.String `tfsdk:"status"`
	ExpiresAt     types.String `tfsdk:"expires_at"`
	UsedAt        types.String `tfsdk:"used_at"`
}

func (r *objectStorageReplicationGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_storage_replication_grant"
}

func (r *objectStorageReplicationGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Authorizes another organization to replicate into one of your buckets. Give the token to the owner " +
			"of the source bucket, who sets it as destination.grant_token of cubepath_object_storage_replication. " +
			"Not needed between buckets of the same organization. A grant is used once, expires (1 to 30 days) and " +
			"can be revoked until it is used. The token is only returned when the grant is created and is kept in " +
			"the Terraform state, marked sensitive: protect your state. Every attribute forces a new grant when " +
			"changed. Destroying it revokes the grant if it was not used; a used grant stays (stop an incoming " +
			"replication from the panel, the API or cubecli). Grants cannot be imported (the token is not readable).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the grant.",
				Computed:      true,
				PlanModifiers: keep,
			},
			"bucket_uuid": schema.StringAttribute{
				Description:   "UUID of the destination bucket, with versioning enabled.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"note": schema.StringAttribute{
				Description:   "A note for yourself, for example who the grant is for (up to 255 characters).",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expires_in_days": schema.Int64Attribute{
				Description:   "Days until the grant expires if it is not used (1 to 30). Defaults to 7.",
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(7),
				Validators:    []validator.Int64{Int64Between(1, 30)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"token": schema.StringAttribute{
				Description:   "The grant token, to give to the owner of the source bucket. Only known when the grant is created.",
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: keep,
			},
			"token_prefix": schema.StringAttribute{
				Description:   "The first characters of the token, to recognize it.",
				Computed:      true,
				PlanModifiers: keep,
			},
			"status": schema.StringAttribute{
				Description: "open, used, expired or revoked.",
				Computed:    true,
			},
			"expires_at": schema.StringAttribute{
				Description:   "When the grant expires if it is not used.",
				Computed:      true,
				PlanModifiers: keep,
			},
			"used_at": schema.StringAttribute{
				Description: "When the grant was used, if it was.",
				Computed:    true,
			},
		},
	}
}

func (r *objectStorageReplicationGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *objectStorageReplicationGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan replicationGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	grant, err := r.client.ObjectStorage.CreateReplicationGrant(ctx, plan.BucketUUID.ValueString(),
		&client.CreateObjectStorageReplicationGrantRequest{
			Note:          knownString(plan.Note),
			ExpiresInDays: int(plan.ExpiresInDays.ValueInt64()),
		})
	if err != nil {
		resp.Diagnostics.AddError("Error creating Object Storage replication grant", err.Error())
		return
	}
	plan.ID = types.StringValue(grant.UUID)
	plan.Token = types.StringValue(grant.Token)
	plan.TokenPrefix = types.StringValue(grant.TokenPrefix)
	plan.Status = types.StringValue("open")
	plan.ExpiresAt = stringOrNull(grant.ExpiresAt)
	plan.UsedAt = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageReplicationGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state replicationGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	grant, err := r.client.ObjectStorage.GetReplicationGrant(ctx, state.BucketUUID.ValueString(), state.ID.ValueString())
	if err != nil {
		// A 404 here is the grant or its bucket gone.
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Object Storage replication grant", err.Error())
		return
	}
	// Revoked outside Terraform: plan a new one.
	if grant.Status == "revoked" {
		resp.State.RemoveResource(ctx)
		return
	}
	mapGrantToState(&state, grant)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func mapGrantToState(state *replicationGrantResourceModel, grant *client.ObjectStorageReplicationGrant) {
	state.TokenPrefix = types.StringValue(grant.TokenPrefix)
	state.Status = types.StringValue(grant.Status)
	state.ExpiresAt = stringOrNull(grant.ExpiresAt)
	state.UsedAt = stringOrNull(grant.UsedAt)
	state.Note = stringOrNull(grant.Note)
}

// Update is never called with a real change: every configurable attribute forces a new grant.
func (r *objectStorageReplicationGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan replicationGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *objectStorageReplicationGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state replicationGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.ObjectStorage.DeleteReplicationGrant(ctx, state.ID.ValueString())
	switch {
	case err == nil, isNotFound(err):
	case isConflict(err):
		// Used grants cannot be revoked: the replication it created is not affected by this.
		resp.Diagnostics.AddWarning("Replication grant already used",
			"The grant was used by a replication and cannot be revoked; it was only removed from the Terraform state. "+
				"To stop the incoming replication, revoke it from the panel, the API or cubecli.")
	default:
		resp.Diagnostics.AddError("Error revoking Object Storage replication grant", err.Error())
	}
}
