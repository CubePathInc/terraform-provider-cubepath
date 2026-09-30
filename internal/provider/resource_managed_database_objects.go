package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Logical databases and users are applied inside the engine by an asynchronous task.
const (
	managedDatabaseObjectTimeout      = 15 * time.Minute
	managedDatabaseObjectPollInterval = 5 * time.Second
)

var (
	_ resource.Resource                = &managedDatabaseDatabaseResource{}
	_ resource.ResourceWithConfigure   = &managedDatabaseDatabaseResource{}
	_ resource.ResourceWithImportState = &managedDatabaseDatabaseResource{}
	_ resource.Resource                = &managedDatabaseUserResource{}
	_ resource.ResourceWithConfigure   = &managedDatabaseUserResource{}
	_ resource.ResourceWithImportState = &managedDatabaseUserResource{}
)

func configureClient(providerData any, diags interface {
	AddError(string, string)
}) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		diags.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", providerData))
		return nil
	}
	return c
}

// waitForObject polls a logical database or user until it is active. refresh returns the
// object status, or "deleted" when it no longer exists.
func waitForObject(ctx context.Context, pending, target []string, refresh func() (string, error)) error {
	conf := &utils.StateChangeConf{
		Pending:      pending,
		Target:       target,
		Timeout:      managedDatabaseObjectTimeout,
		PollInterval: managedDatabaseObjectPollInterval,
		Refresh: func() (interface{}, string, error) {
			status, err := refresh()
			if err != nil {
				return nil, "", err
			}
			if status == "error" {
				return nil, "", fmt.Errorf("the operation failed inside the database engine")
			}
			return status, status, nil
		},
	}
	_, err := conf.WaitForState(ctx)
	return err
}

func splitImportID(id, format string) (string, string, error) {
	parts := strings.Split(id, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected format: %s", format)
	}
	return parts[0], parts[1], nil
}

// ---- cubepath_managed_database_database ----

func NewManagedDatabaseDatabaseResource() resource.Resource {
	return &managedDatabaseDatabaseResource{}
}

type managedDatabaseDatabaseResource struct {
	client *client.Client
}

type managedDatabaseDatabaseModel struct {
	ID                types.String `tfsdk:"id"`
	ManagedDatabaseID types.String `tfsdk:"managed_database_id"`
	Name              types.String `tfsdk:"name"`
	Status            types.String `tfsdk:"status"`
}

func (r *managedDatabaseDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_database_database"
}

func (r *managedDatabaseDatabaseResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a database (schema) inside a MySQL or PostgreSQL managed database. Not available on " +
			"Valkey, which uses numbered databases. Destroying it drops the database and its data. " +
			"Import with managed_database_id/database_id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the database.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"managed_database_id": schema.StringAttribute{
				Description:   "UUID of the managed database. Changing it forces a new database.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Database name: up to 60 lowercase letters, numbers and underscores, starting with a letter. " +
					"Changing it forces a new database.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				Description: "Status: pending, active, error or deleting.",
				Computed:    true,
			},
		},
	}
}

func (r *managedDatabaseDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *managedDatabaseDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan managedDatabaseDatabaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mdID := plan.ManagedDatabaseID.ValueString()

	created, err := r.client.ManagedDatabases.CreateDatabase(ctx, mdID, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating database", err.Error())
		return
	}
	plan.ID = types.StringValue(created.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("managed_database_id"), plan.ManagedDatabaseID)...)

	err = waitForObject(ctx, []string{"pending"}, []string{"active"}, func() (string, error) {
		db, err := r.client.ManagedDatabases.GetDatabase(ctx, mdID, created.UUID)
		if err != nil {
			return "", err
		}
		return db.Status, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the database to be created", err.Error())
		return
	}

	plan.Status = types.StringValue("active")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *managedDatabaseDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state managedDatabaseDatabaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	db, err := r.client.ManagedDatabases.GetDatabase(ctx, state.ManagedDatabaseID.ValueString(), state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database", err.Error())
		return
	}

	state.Name = types.StringValue(db.Name)
	state.Status = types.StringValue(db.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *managedDatabaseDatabaseResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Every attribute of a managed database database forces a new one.")
}

func (r *managedDatabaseDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state managedDatabaseDatabaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mdID, id := state.ManagedDatabaseID.ValueString(), state.ID.ValueString()

	err := r.client.ManagedDatabases.DeleteDatabase(ctx, mdID, id)
	if err != nil && !isConflict(err) { // 409: already being deleted
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting database", err.Error())
		return
	}

	err = waitForObject(ctx, []string{"deleting", "active", "pending"}, []string{"deleted"}, func() (string, error) {
		db, err := r.client.ManagedDatabases.GetDatabase(ctx, mdID, id)
		if err != nil {
			if isNotFound(err) {
				return "deleted", nil
			}
			return "", err
		}
		return db.Status, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the database to be deleted", err.Error())
	}
}

func (r *managedDatabaseDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mdID, id, err := splitImportID(req.ID, "managed_database_id/database_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("managed_database_id"), mdID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}

// ---- cubepath_managed_database_user ----

func NewManagedDatabaseUserResource() resource.Resource {
	return &managedDatabaseUserResource{}
}

type managedDatabaseUserResource struct {
	client *client.Client
}

type managedDatabaseUserModel struct {
	ID                types.String `tfsdk:"id"`
	ManagedDatabaseID types.String `tfsdk:"managed_database_id"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	Status            types.String `tfsdk:"status"`
}

func (r *managedDatabaseUserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_database_user"
}

func (r *managedDatabaseUserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a user inside a managed database. The platform grants its standard privileges; they " +
			"cannot be customized. The password can never be read back from the API: it is kept in the " +
			"Terraform state only. Import with managed_database_id/user_id (the password imports empty).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the user.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"managed_database_id": schema.StringAttribute{
				Description:   "UUID of the managed database. Changing it forces a new user.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"username": schema.StringAttribute{
				Description: "Username: up to 32 lowercase letters, numbers and underscores, starting with a letter. " +
					"Changing it forces a new user.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"password": schema.StringAttribute{
				Description: "Password, 12 to 64 characters. Omit it to let the platform generate one. " +
					"Changing it recreates the user.",
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIf(requiresReplaceIfConfiguredString,
						"Changing the password recreates the user.",
						"Changing the password recreates the user."),
				},
			},
			"status": schema.StringAttribute{
				Description: "Status: pending, active, error or deleting.",
				Computed:    true,
			},
		},
	}
}

func (r *managedDatabaseUserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *managedDatabaseUserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan managedDatabaseUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mdID := plan.ManagedDatabaseID.ValueString()

	password := ""
	if p := knownString(plan.Password); p != nil {
		password = *p
	}
	created, err := r.client.ManagedDatabases.CreateUser(ctx, mdID, plan.Username.ValueString(), password)
	if err != nil {
		resp.Diagnostics.AddError("Error creating database user", err.Error())
		return
	}
	plan.ID = types.StringValue(created.UUID)
	plan.Password = types.StringValue(created.Password)
	// The password is only returned now: store it before waiting.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("managed_database_id"), plan.ManagedDatabaseID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("username"), plan.Username)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("password"), plan.Password)...)

	err = waitForObject(ctx, []string{"pending"}, []string{"active"}, func() (string, error) {
		user, err := r.client.ManagedDatabases.GetUser(ctx, mdID, created.UUID)
		if err != nil {
			return "", err
		}
		return user.Status, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the database user to be created", err.Error())
		return
	}

	plan.Status = types.StringValue("active")
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *managedDatabaseUserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state managedDatabaseUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.ManagedDatabases.GetUser(ctx, state.ManagedDatabaseID.ValueString(), state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading database user", err.Error())
		return
	}

	state.Username = types.StringValue(user.Username)
	state.Status = types.StringValue(user.Status)
	if state.Password.IsUnknown() {
		state.Password = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *managedDatabaseUserResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Every attribute of a managed database user forces a new one.")
}

func (r *managedDatabaseUserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state managedDatabaseUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	mdID, id := state.ManagedDatabaseID.ValueString(), state.ID.ValueString()

	err := r.client.ManagedDatabases.DeleteUser(ctx, mdID, id)
	if err != nil && !isConflict(err) {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting database user", err.Error())
		return
	}

	err = waitForObject(ctx, []string{"deleting", "active", "pending"}, []string{"deleted"}, func() (string, error) {
		user, err := r.client.ManagedDatabases.GetUser(ctx, mdID, id)
		if err != nil {
			if isNotFound(err) {
				return "deleted", nil
			}
			return "", err
		}
		return user.Status, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the database user to be deleted", err.Error())
	}
}

func (r *managedDatabaseUserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mdID, id, err := splitImportID(req.ID, "managed_database_id/user_id")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("managed_database_id"), mdID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
}
