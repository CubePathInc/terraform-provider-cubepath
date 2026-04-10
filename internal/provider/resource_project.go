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
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &projectResource{}
	_ resource.ResourceWithConfigure   = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

// NewProjectResource is a helper function to simplify the provider implementation.
func NewProjectResource() resource.Resource {
	return &projectResource{}
}

// projectResource is the resource implementation.
type projectResource struct {
	client *client.Client
}

// projectResourceModel maps the resource schema data.
type projectResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

// Metadata returns the resource type name.
func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

// Schema defines the schema for the resource.
func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a project on CubePath Cloud. Projects are used to organize and group resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the project.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the project.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "The description of the project.",
				Optional:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the project was created.",
				Computed:    true,
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new project
	createReq := &client.CreateProjectRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	project, err := r.client.Projects.Create(ctx, createReq)
	if err != nil {
		// If the project already exists, adopt it instead of failing
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsBadRequest() {
			existing, lookupErr := r.client.Projects.GetByName(ctx, plan.Name.ValueString())
			if lookupErr != nil {
				resp.Diagnostics.AddError(
					"Error creating project",
					"Project already exists but could not be found: "+err.Error(),
				)
				return
			}
			project = &existing.Project
		} else {
			resp.Diagnostics.AddError(
				"Error creating project",
				"Could not create project, unexpected error: "+err.Error(),
			)
			return
		}
	}

	// Map response to schema
	plan.ID = types.StringValue(strconv.Itoa(project.ID))
	plan.CreatedAt = types.StringValue(project.CreatedAt.String())

	// Set state
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project ID from state
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing project ID",
			"Could not parse project ID: "+err.Error(),
		)
		return
	}

	// Get refreshed project value
	projectResp, err := r.client.Projects.Get(ctx, id)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error reading project",
			"Could not read project ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update state
	state.Name = types.StringValue(projectResp.Project.Name)
	state.Description = types.StringValue(projectResp.Project.Description)
	state.CreatedAt = types.StringValue(projectResp.Project.CreatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project ID from state
	id, err := strconv.Atoi(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing project ID",
			"Could not parse project ID: "+err.Error(),
		)
		return
	}

	// Update project
	updateReq := &client.CreateProjectRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
	}

	project, err := r.client.Projects.Update(ctx, id, updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating project",
			"Could not update project, unexpected error: "+err.Error(),
		)
		return
	}

	// Update state
	plan.CreatedAt = types.StringValue(project.CreatedAt.String())

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project ID from state
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing project ID",
			"Could not parse project ID: "+err.Error(),
		)
		return
	}

	// Delete project
	err = r.client.Projects.Delete(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting project",
			"Could not delete project, unexpected error: "+err.Error(),
		)
		return
	}
}

// ImportState imports an existing resource into Terraform.
func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
