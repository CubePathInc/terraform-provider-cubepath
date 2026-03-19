package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &sshKeyResource{}
	_ resource.ResourceWithConfigure   = &sshKeyResource{}
	_ resource.ResourceWithImportState = &sshKeyResource{}
)

// NewSSHKeyResource is a helper function to simplify the provider implementation.
func NewSSHKeyResource() resource.Resource {
	return &sshKeyResource{}
}

// sshKeyResource is the resource implementation.
type sshKeyResource struct {
	client *client.Client
}

// sshKeyResourceModel maps the resource schema data.
type sshKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	PublicKey   types.String `tfsdk:"public_key"`
	Fingerprint types.String `tfsdk:"fingerprint"`
	KeyType     types.String `tfsdk:"key_type"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

// Metadata returns the resource type name.
func (r *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

// Schema defines the schema for the resource.
func (r *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an SSH key on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the SSH key.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the SSH key.",
				Required:    true,
			},
			"public_key": schema.StringAttribute{
				Description: "The SSH public key.",
				Required:    true,
				Sensitive:   true,
				Validators:  []validator.String{SSHKeyValidator()},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"fingerprint": schema.StringAttribute{
				Description: "The fingerprint of the SSH key.",
				Computed:    true,
			},
			"key_type": schema.StringAttribute{
				Description: "The type of the SSH key (e.g., ssh-rsa, ssh-ed25519).",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the SSH key was created.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *sshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// Create creates the resource and sets the initial Terraform state.
func (r *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan sshKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new SSH key
	createReq := &client.CreateSSHKeyRequest{
		Name:   plan.Name.ValueString(),
		SSHKey: plan.PublicKey.ValueString(),
	}

	sshKey, err := r.client.SSHKeys.Create(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating SSH key",
			"Could not create SSH key, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ID = types.StringValue(strconv.Itoa(sshKey.ID))
	plan.Fingerprint = types.StringValue(sshKey.Fingerprint)
	plan.KeyType = types.StringValue(sshKey.KeyType)
	if sshKey.CreatedAt != "" {
		plan.CreatedAt = types.StringValue(sshKey.CreatedAt)
	} else {
		plan.CreatedAt = types.StringValue("")
	}

	// Set state to fully populated data
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state sshKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get SSH key ID from state
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing SSH key ID",
			"Could not parse SSH key ID: "+err.Error(),
		)
		return
	}

	// Get refreshed SSH key value from CubePath
	sshKey, err := r.client.SSHKeys.Get(ctx, id)
	if err != nil {
		// If the key is not found, remove it from state
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error Reading SSH key",
			"Could not read SSH key ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite items with refreshed state
	state.Name = types.StringValue(sshKey.Name)
	state.Fingerprint = types.StringValue(sshKey.Fingerprint)
	state.KeyType = types.StringValue(sshKey.KeyType)
	if sshKey.CreatedAt != "" {
		state.CreatedAt = types.StringValue(sshKey.CreatedAt)
	}
	// Note: We don't update public_key as it's sensitive and not returned by the API

	// Set refreshed state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Note: SSH keys don't support updates in the CubePath API
	// The public_key has RequiresReplace, so any change will recreate the resource
	// The name field could potentially be updated, but the API doesn't support it
	// So for now, we just copy the plan to state

	var plan sshKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Since CubePath API doesn't support updating SSH keys,
	// we would need to delete and recreate, which is handled by RequiresReplace
	resp.Diagnostics.AddError(
		"Update Not Supported",
		"SSH keys cannot be updated. Any changes require recreating the resource.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state sshKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get SSH key ID from state
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing SSH key ID",
			"Could not parse SSH key ID: "+err.Error(),
		)
		return
	}

	// Delete existing SSH key
	err = r.client.SSHKeys.Delete(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting SSH key",
			"Could not delete SSH key, unexpected error: "+err.Error(),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform.
func (r *sshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
