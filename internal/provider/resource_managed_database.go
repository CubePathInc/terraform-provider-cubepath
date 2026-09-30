package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/cubepath/terraform-provider-cubepath/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &managedDatabaseResource{}
	_ resource.ResourceWithConfigure   = &managedDatabaseResource{}
	_ resource.ResourceWithImportState = &managedDatabaseResource{}
)

const managedDatabasePollInterval = 15 * time.Second

func NewManagedDatabaseResource() resource.Resource {
	return &managedDatabaseResource{}
}

type managedDatabaseResource struct {
	client *client.Client
}

type managedDatabaseResourceModel struct {
	ID                  types.String   `tfsdk:"id"`
	ProjectID           types.Int64    `tfsdk:"project_id"`
	Name                types.String   `tfsdk:"name"`
	Label               types.String   `tfsdk:"label"`
	Engine              types.String   `tfsdk:"engine"`
	Version             types.String   `tfsdk:"version"`
	PlanUUID            types.String   `tfsdk:"plan_uuid"`
	Replicas            types.Int64    `tfsdk:"replicas"`
	Topology            types.String   `tfsdk:"topology"`
	BackupEnabled       types.Bool     `tfsdk:"backup_enabled"`
	BackupScheduleCron  types.String   `tfsdk:"backup_schedule_cron"`
	BackupRetentionDays types.Int64    `tfsdk:"backup_retention_days"`
	Config              types.Map      `tfsdk:"config"`
	Protected           types.Bool     `tfsdk:"protected"`
	Status              types.String   `tfsdk:"status"`
	EndpointHost        types.String   `tfsdk:"endpoint_host"`
	EndpointPort        types.Int64    `tfsdk:"endpoint_port"`
	PlanName            types.String   `tfsdk:"plan_name"`
	LocationName        types.String   `tfsdk:"location_name"`
	BillingType         types.String   `tfsdk:"billing_type"`
	Timeouts            timeouts.Value `tfsdk:"timeouts"`
}

func (r *managedDatabaseResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_database"
}

func (r *managedDatabaseResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a CubePath Managed Database: a highly available MySQL, PostgreSQL or Valkey " +
			"cluster with one node per replica. The plan sets the location and the per-node size; " +
			"see the cubepath_managed_database_plans data source. Connection credentials are read with the " +
			"cubepath_managed_database_credentials data source. Destroying it deletes all its data. " +
			"Import with the database UUID; config is not readable from the API and imports empty.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the managed database.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.Int64Attribute{
				Description:   "Project ID. Changing it forces a new database.",
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Name, unique in the organization: 2 to 60 lowercase letters, numbers and hyphens, " +
					"starting and ending with a letter or number. Can be changed in place.",
				Required: true,
			},
			"label": schema.StringAttribute{
				Description: "Free-form label. Can be changed in place.",
				Optional:    true,
			},
			"engine": schema.StringAttribute{
				Description:   "Database engine: mysql, postgresql or valkey. Changing it forces a new database.",
				Required:      true,
				Validators:    []validator.String{StringOneOf("mysql", "postgresql", "valkey")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"version": schema.StringAttribute{
				Description: "Engine version. Versions are pinned (mysql 8.0.39, postgresql 17.5.0, valkey 7.2.11 at " +
					"the time of writing). Changing it forces a new database.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"plan_uuid": schema.StringAttribute{
				Description: "UUID of the plan (per-node size and location). Changing it to another plan of the same " +
					"engine and location resizes the database in place.",
				Required: true,
			},
			"replicas": schema.Int64Attribute{
				Description: "Number of nodes (one dedicated server each). At least 3 for mysql and 2 for postgresql " +
					"and valkey, at most the plan's max_replicas. Defaults to 3. Can be changed in place.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"topology": schema.StringAttribute{
				Description: "Cluster topology. Defaults to mgr for mysql and replication for postgresql and valkey. " +
					"Changing it forces a new database.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"backup_enabled": schema.BoolAttribute{
				Description:   "Whether the backup policy is enabled. Setting backup_schedule_cron enables it.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"backup_schedule_cron": schema.StringAttribute{
				Description: "Backup schedule as a 5-field cron expression (minute hour day month weekday), in UTC. " +
					"The API cannot clear a schedule once set; disable backups with backup_enabled = false instead.",
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"backup_retention_days": schema.Int64Attribute{
				Description:   "Days to keep each backup, 1 to 365. Defaults to 7.",
				Optional:      true,
				Computed:      true,
				Validators:    []validator.Int64{Int64Between(1, 365)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"config": schema.MapAttribute{
				Description: "Engine parameters to tune, as strings (for example max_connections = \"500\"). Only the " +
					"parameters the platform allows can be set; some need a rolling restart. Removing a " +
					"parameter sets it back to the engine default. The API does not report the effective " +
					"values, so changes made outside Terraform are not detected.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"protected": schema.BoolAttribute{
				Description: "Deletion protection. A protected database cannot be destroyed until this is set to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"status": schema.StringAttribute{
				Description: "Status: provisioning, active, updating, scaling, degraded, error, deleting...",
				Computed:    true,
			},
			"endpoint_host": schema.StringAttribute{
				Description: "Public address to connect to.",
				Computed:    true,
			},
			"endpoint_port": schema.Int64Attribute{
				Description: "Port to connect to (3306 mysql, 5432 postgresql, 6379 valkey).",
				Computed:    true,
			},
			"plan_name": schema.StringAttribute{
				Description: "Name of the plan.",
				Computed:    true,
			},
			"location_name": schema.StringAttribute{
				Description:   "Location of the database, set by the plan.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"billing_type": schema.StringAttribute{
				Description:   "Billing type, inherited from the project.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Update: true,
				Delete: true,
			}),
		},
	}
}

func (r *managedDatabaseResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *managedDatabaseResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan managedDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createTimeout, diags := plan.Timeouts.Create(ctx, 60*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	createReq := &client.CreateManagedDatabaseRequest{
		ProjectID: int(plan.ProjectID.ValueInt64()),
		Name:      plan.Name.ValueString(),
		Engine:    plan.Engine.ValueString(),
		Version:   plan.Version.ValueString(),
		PlanUUID:  plan.PlanUUID.ValueString(),
		Replicas:  knownInt(plan.Replicas),
		Topology:  knownString(plan.Topology),
	}
	if cron := knownString(plan.BackupScheduleCron); cron != nil {
		createReq.Backup = &client.ManagedDatabaseBackup{ScheduleCron: *cron}
		if days := knownInt(plan.BackupRetentionDays); days != nil {
			createReq.Backup.RetentionDays = *days
		}
	}

	// The platform builds a limited number of databases at a time and answers 409 meanwhile.
	var created *client.ManagedDatabase
	err := retryWhile(ctx, createTimeout, 30*time.Second, isConflict, func() error {
		var err error
		created, err = r.client.ManagedDatabases.Create(ctx, createReq)
		return err
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating managed database", err.Error())
		return
	}

	// Save the ID right away so a failed wait does not leave an untracked database behind.
	plan.ID = types.StringValue(created.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	md, err := r.waitReady(ctx, created.UUID, createTimeout)
	if err != nil {
		resp.Diagnostics.AddError("Error waiting for the managed database to become active", err.Error())
		return
	}

	// Backup flag and label are not part of the create request.
	updateReq := &client.UpdateManagedDatabaseRequest{}
	if label := knownString(plan.Label); label != nil && *label != "" {
		updateReq.Label = label
	}
	backup := &client.ManagedDatabaseBackupUpdate{}
	if enabled := knownBool(plan.BackupEnabled); enabled != nil && *enabled != md.BackupEnabled {
		backup.Enabled = enabled
	}
	if days := knownInt(plan.BackupRetentionDays); days != nil && *days != md.BackupRetentionDays {
		backup.RetentionDays = days
	}
	if backup.Enabled != nil || backup.RetentionDays != nil {
		updateReq.Backup = backup
	}
	if updateReq.Label != nil || updateReq.Backup != nil {
		if err := r.client.ManagedDatabases.Update(ctx, created.UUID, updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating managed database", err.Error())
			return
		}
	}

	params, d := r.configChanges(ctx, created.UUID, types.MapNull(types.StringType), plan.Config)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(params) > 0 {
		if err := r.runOperation(ctx, created.UUID, createTimeout, func() error {
			return r.client.ManagedDatabases.UpdateConfig(ctx, created.UUID, params)
		}); err != nil {
			resp.Diagnostics.AddError("Error applying the managed database configuration", err.Error())
			return
		}
	}

	if plan.Protected.ValueBool() {
		if err := r.client.ManagedDatabases.SetProtection(ctx, created.UUID, true); err != nil {
			resp.Diagnostics.AddError("Error enabling deletion protection", err.Error())
			return
		}
	}

	md, err = r.client.ManagedDatabases.Get(ctx, created.UUID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading managed database", err.Error())
		return
	}
	mapManagedDatabaseToState(&plan, md)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *managedDatabaseResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state managedDatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	md, err := r.client.ManagedDatabases.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading managed database", err.Error())
		return
	}

	mapManagedDatabaseToState(&state, md)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *managedDatabaseResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state managedDatabaseResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateTimeout, diags := plan.Timeouts.Update(ctx, 60*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	id := state.ID.ValueString()

	// Protection off first, so the rest of the plan (or a later destroy) is not blocked by it.
	if !plan.Protected.ValueBool() && state.Protected.ValueBool() {
		if err := r.client.ManagedDatabases.SetProtection(ctx, id, false); err != nil {
			resp.Diagnostics.AddError("Error disabling deletion protection", err.Error())
			return
		}
	}

	updateReq := &client.UpdateManagedDatabaseRequest{}
	changed := false
	if !plan.Name.Equal(state.Name) {
		updateReq.Name = knownString(plan.Name)
		changed = true
	}
	if !plan.Label.Equal(state.Label) {
		label := plan.Label.ValueString() // null clears it
		updateReq.Label = &label
		changed = true
	}
	backup := &client.ManagedDatabaseBackupUpdate{}
	if !plan.BackupScheduleCron.IsNull() && !plan.BackupScheduleCron.Equal(state.BackupScheduleCron) {
		backup.ScheduleCron = knownString(plan.BackupScheduleCron)
	}
	if !plan.BackupRetentionDays.IsUnknown() && !plan.BackupRetentionDays.Equal(state.BackupRetentionDays) {
		backup.RetentionDays = knownInt(plan.BackupRetentionDays)
	}
	if !plan.BackupEnabled.IsUnknown() && !plan.BackupEnabled.Equal(state.BackupEnabled) {
		backup.Enabled = knownBool(plan.BackupEnabled)
	}
	if backup.ScheduleCron != nil || backup.RetentionDays != nil || backup.Enabled != nil {
		updateReq.Backup = backup
		changed = true
	}
	if changed {
		if err := r.client.ManagedDatabases.Update(ctx, id, updateReq); err != nil {
			resp.Diagnostics.AddError("Error updating managed database", err.Error())
			return
		}
	}

	// Scaling runs one operation at a time: plan first, then replicas.
	if !plan.PlanUUID.Equal(state.PlanUUID) {
		planUUID := plan.PlanUUID.ValueString()
		if err := r.runOperation(ctx, id, updateTimeout, func() error {
			return r.client.ManagedDatabases.Scale(ctx, id, &client.ScaleManagedDatabaseRequest{PlanUUID: &planUUID})
		}); err != nil {
			resp.Diagnostics.AddError("Error changing the managed database plan", err.Error())
			return
		}
	}
	if replicas := knownInt(plan.Replicas); replicas != nil && !plan.Replicas.Equal(state.Replicas) {
		if err := r.runOperation(ctx, id, updateTimeout, func() error {
			return r.client.ManagedDatabases.Scale(ctx, id, &client.ScaleManagedDatabaseRequest{Replicas: replicas})
		}); err != nil {
			resp.Diagnostics.AddError("Error changing the managed database replicas", err.Error())
			return
		}
	}

	params, d := r.configChanges(ctx, id, state.Config, plan.Config)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(params) > 0 {
		if err := r.runOperation(ctx, id, updateTimeout, func() error {
			return r.client.ManagedDatabases.UpdateConfig(ctx, id, params)
		}); err != nil {
			resp.Diagnostics.AddError("Error updating the managed database configuration", err.Error())
			return
		}
	}

	if plan.Protected.ValueBool() && !state.Protected.ValueBool() {
		if err := r.client.ManagedDatabases.SetProtection(ctx, id, true); err != nil {
			resp.Diagnostics.AddError("Error enabling deletion protection", err.Error())
			return
		}
	}

	md, err := r.client.ManagedDatabases.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading managed database", err.Error())
		return
	}
	mapManagedDatabaseToState(&plan, md)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *managedDatabaseResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state managedDatabaseResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()
	id := state.ID.ValueString()

	// 409 while provisioning or while another operation runs: wait for it to settle.
	err := retryWhile(ctx, deleteTimeout, 30*time.Second, isConflict, func() error {
		return r.client.ManagedDatabases.Delete(ctx, id)
	})
	if err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting managed database", err.Error())
		return
	}

	conf := &utils.StateChangeConf{
		Pending:      []string{"deleting"},
		Target:       []string{"deleted"},
		Timeout:      deleteTimeout,
		PollInterval: managedDatabasePollInterval,
		Refresh: func() (interface{}, string, error) {
			md, err := r.client.ManagedDatabases.Get(ctx, id)
			if err != nil {
				if isNotFound(err) {
					return nil, "deleted", nil
				}
				return nil, "", err
			}
			return md, md.Status, nil
		},
	}
	if _, err := conf.WaitForState(ctx); err != nil {
		resp.Diagnostics.AddError("Error waiting for the managed database to be deleted", err.Error())
	}
}

func (r *managedDatabaseResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// waitReady waits for a managed database to be active (or degraded, which still serves).
func (r *managedDatabaseResource) waitReady(ctx context.Context, id string, timeout time.Duration) (*client.ManagedDatabase, error) {
	conf := &utils.StateChangeConf{
		Pending:      []string{"provisioning", "updating", "scaling", "backing_up", "restoring"},
		Target:       []string{"active", "degraded"},
		Timeout:      timeout,
		PollInterval: managedDatabasePollInterval,
		Refresh: func() (interface{}, string, error) {
			md, err := r.client.ManagedDatabases.Get(ctx, id)
			if err != nil {
				return nil, "", err
			}
			if md.Status == "error" {
				return nil, "", fmt.Errorf("the managed database ended in error; contact support or destroy it")
			}
			return md, md.Status, nil
		},
	}
	result, err := conf.WaitForState(ctx)
	if err != nil {
		return nil, err
	}
	return result.(*client.ManagedDatabase), nil
}

// runOperation starts an asynchronous operation (retrying while another one runs) and waits
// for the database to be ready again.
func (r *managedDatabaseResource) runOperation(ctx context.Context, id string, timeout time.Duration, start func() error) error {
	if err := retryWhile(ctx, timeout, 20*time.Second, isConflict, start); err != nil {
		return err
	}
	// The status flips right away, but give the task a moment to be picked up.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
	}
	_, err := r.waitReady(ctx, id, timeout)
	return err
}

// configChanges returns the parameters to send so the database matches the planned config:
// new and changed values, and the engine default for removed ones. Values are converted to
// the type the API describes for each parameter.
func (r *managedDatabaseResource) configChanges(ctx context.Context, id string, oldConfig, newConfig types.Map) (map[string]interface{}, diag.Diagnostics) {
	var diags diag.Diagnostics
	oldValues := map[string]string{}
	newValues := map[string]string{}
	if !oldConfig.IsNull() && !oldConfig.IsUnknown() {
		diags.Append(oldConfig.ElementsAs(ctx, &oldValues, false)...)
	}
	if !newConfig.IsNull() && !newConfig.IsUnknown() {
		diags.Append(newConfig.ElementsAs(ctx, &newValues, false)...)
	}
	if diags.HasError() {
		return nil, diags
	}

	changed := map[string]string{}
	var removed []string
	for k, v := range newValues {
		if old, ok := oldValues[k]; !ok || old != v {
			changed[k] = v
		}
	}
	for k := range oldValues {
		if _, ok := newValues[k]; !ok {
			removed = append(removed, k)
		}
	}
	if len(changed) == 0 && len(removed) == 0 {
		return nil, diags
	}

	cfg, err := r.client.ManagedDatabases.GetConfig(ctx, id)
	if err != nil {
		diags.AddError("Error reading the managed database configuration", err.Error())
		return nil, diags
	}
	params, err := buildConfigParams(cfg, changed, removed)
	if err != nil {
		diags.AddAttributeError(path.Root("config"), "Invalid managed database configuration", err.Error())
		return nil, diags
	}
	return params, diags
}

// buildConfigParams converts configured string values to the types of the engine
// parameters, and resets removed parameters to their default.
func buildConfigParams(cfg *client.ManagedDatabaseConfig, changed map[string]string, removed []string) (map[string]interface{}, error) {
	params := map[string]interface{}{}
	var unknown []string
	for k, v := range changed {
		p, ok := cfg.Params[k]
		if !ok {
			unknown = append(unknown, k)
			continue
		}
		switch p.Type {
		case "int":
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s must be an integer, got %q", k, v)
			}
			params[k] = n
		case "float":
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return nil, fmt.Errorf("%s must be a number, got %q", k, v)
			}
			params[k] = f
		default:
			params[k] = v
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		allowed := make([]string, 0, len(cfg.Params))
		for k := range cfg.Params {
			allowed = append(allowed, k)
		}
		sort.Strings(allowed)
		return nil, fmt.Errorf("unknown or non-tunable parameters for engine %s: %s. Allowed: %s",
			cfg.Engine, strings.Join(unknown, ", "), strings.Join(allowed, ", "))
	}
	for _, k := range removed {
		p, ok := cfg.Params[k]
		if !ok || len(p.Default) == 0 || string(p.Default) == "null" {
			continue
		}
		params[k] = p.Default
	}
	return params, nil
}

func mapManagedDatabaseToState(state *managedDatabaseResourceModel, md *client.ManagedDatabase) {
	state.ID = types.StringValue(md.UUID)
	state.ProjectID = types.Int64Value(int64(md.ProjectID))
	state.Name = types.StringValue(md.Name)
	state.Label = stringOrNull(md.Label)
	state.Engine = types.StringValue(md.Engine)
	state.Version = types.StringValue(md.Version)
	state.Topology = types.StringValue(md.Topology)
	state.Replicas = types.Int64Value(int64(md.Replicas))
	state.Status = types.StringValue(md.Status)
	state.BackupEnabled = types.BoolValue(md.BackupEnabled)
	state.BackupScheduleCron = stringOrNull(md.BackupScheduleCron)
	state.BackupRetentionDays = types.Int64Value(int64(md.BackupRetentionDays))
	state.Protected = types.BoolValue(md.Protected)
	state.BillingType = types.StringValue(md.BillingType)
	if md.Plan != nil {
		state.PlanUUID = types.StringValue(md.Plan.UUID)
		state.PlanName = types.StringValue(md.Plan.Name)
	} else {
		state.PlanName = types.StringNull()
	}
	if md.Location != nil {
		state.LocationName = types.StringValue(md.Location.LocationName)
	} else {
		state.LocationName = types.StringNull()
	}
	state.EndpointHost = stringOrNull(md.EndpointHost)
	if md.EndpointPort != nil {
		state.EndpointPort = types.Int64Value(int64(*md.EndpointPort))
	} else {
		state.EndpointPort = types.Int64Null()
	}
	if state.Config.IsUnknown() {
		state.Config = types.MapNull(types.StringType)
	}
}
