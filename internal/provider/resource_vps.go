package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &vpsResource{}
	_ resource.ResourceWithConfigure   = &vpsResource{}
	_ resource.ResourceWithImportState = &vpsResource{}
)

func NewVPSResource() resource.Resource {
	return &vpsResource{}
}

type vpsResource struct {
	client *client.Client
}

type vpsResourceModel struct {
	ID                    types.String   `tfsdk:"id"`
	Name                  types.String   `tfsdk:"name"`
	Label                 types.String   `tfsdk:"label"`
	ProjectID             types.Int64    `tfsdk:"project_id"`
	Location              types.String   `tfsdk:"location"`
	PlanName              types.String   `tfsdk:"plan_name"`
	TemplateName          types.String   `tfsdk:"template_name"`
	NetworkID             types.Int64    `tfsdk:"network_id"`
	SSHKeyIDs             types.Set      `tfsdk:"ssh_key_ids"`
	User                  types.String   `tfsdk:"user"`
	Password              types.String   `tfsdk:"password"`
	IPv4                  types.Bool     `tfsdk:"ipv4"`
	IPv6Enabled           types.Bool     `tfsdk:"ipv6_enabled"`
	EnableBackups         types.Bool     `tfsdk:"enable_backups"`
	CustomCloudInit       types.String   `tfsdk:"custom_cloudinit"`
	FirewallGroupIDs      types.Set      `tfsdk:"firewall_group_ids"`
	AvailabilityGroupUUID types.String   `tfsdk:"availability_group_uuid"`
	Protected             types.Bool     `tfsdk:"protected"`
	ISOID                 types.String   `tfsdk:"iso_id"`
	BackupScheduleHour    types.Int64    `tfsdk:"backup_schedule_hour"`
	BackupRetentionDays   types.Int64    `tfsdk:"backup_retention_days"`
	BackupMaxBackups      types.Int64    `tfsdk:"backup_max_backups"`
	PowerState            types.String   `tfsdk:"power_state"`
	Status                types.String   `tfsdk:"status"`
	MainIP                types.String   `tfsdk:"main_ip"`
	IPv6                  types.String   `tfsdk:"ipv6"`
	PrivateIP             types.String   `tfsdk:"private_ip"`
	VCPUs                 types.Int64    `tfsdk:"vcpus"`
	RAM                   types.Int64    `tfsdk:"ram"`
	Storage               types.Int64    `tfsdk:"storage"`
	Bandwidth             types.Int64    `tfsdk:"bandwidth"`
	CreatedAt             types.String   `tfsdk:"created_at"`
	Timeouts              timeouts.Value `tfsdk:"timeouts"`
}

func (r *vpsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vps"
}

func (r *vpsResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a VPS instance on CubePath Cloud.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the VPS.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The hostname of the VPS. Can be changed in place.",
				Required:    true,
				Validators:  []validator.String{HostnameValidator()},
			},
			"label": schema.StringAttribute{
				Description: "A label for the VPS. Can be changed in place.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID to associate the VPS with. Changing it moves the VPS to the other project in place.",
				Required:    true,
			},
			"location": schema.StringAttribute{
				Description: "The location where the VPS will be created (e.g., 'us-mia-1').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"plan_name": schema.StringAttribute{
				Description: "The plan name for the VPS (e.g., 'gp.pro').",
				Required:    true,
			},
			"template_name": schema.StringAttribute{
				Description: "The operating system template (e.g., 'debian-12').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network_id": schema.Int64Attribute{
				Description: "Optional private network ID to attach to the VPS. Changing it detaches the old network and " +
					"attaches the new one in place; the new interface is active after the VPS restarts. " +
					"Set it to 0 to detach the network.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"ssh_key_ids": schema.SetAttribute{
				Description: "Set of SSH key IDs to add to the VPS. Changing it attaches and detaches keys in place; " +
					"the guest only picks up the new keys on its next reinstall.",
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"user": schema.StringAttribute{
				Description: "SSH user for the VPS. Defaults to 'root' for Linux or 'Administrator' for Windows.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Description: "Root password for the VPS. Required if ssh_key_ids is not provided.",
				Optional:    true,
				Sensitive:   true,
				Validators:  []validator.String{StrongPasswordValidator()},
			},
			"ipv4": schema.BoolAttribute{
				Description: "Enable public IPv4 address (dual-stack). Additional $1.50/month. Defaults to true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_enabled": schema.BoolAttribute{
				Description: "Enable public IPv6 address (free). Defaults to true. Set to false to deploy without any public IP — requires network_id.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_backups": schema.BoolAttribute{
				Description: "Enable automatic backups for this VPS. Defaults to false. Can be changed in place " +
					"(disabling needs every backup deleted first).",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"custom_cloudinit": schema.StringAttribute{
				Description: "Custom cloud-init configuration (YAML). Only for Linux templates. The API does not return it, so it is not read back on import.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					// Not readable from the API: after an import the state is null,
					// so only force replacement when a previously known value changes.
					stringplanmodifier.RequiresReplaceIf(requiresReplaceIfKnownInState,
						"Changing custom_cloudinit requires replacing the VPS.",
						"Changing custom_cloudinit requires replacing the VPS."),
				},
			},
			"firewall_group_ids": schema.SetAttribute{
				Description: "Set of firewall group IDs to assign to this VPS. If omitted, firewall groups are not managed by Terraform.",
				Optional:    true,
				Computed:    true,
				ElementType: types.Int64Type,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"availability_group_uuid": schema.StringAttribute{
				Description: "UUID of the availability group to place this VPS in. VPS in the same availability group are " +
					"distributed across different physical hosts. Changing it moves the VPS between groups in place; " +
					"set it to an empty string to take the VPS out of its group.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"protected": schema.BoolAttribute{
				Description: "Destruction protection. A protected VPS cannot be destroyed or reinstalled until this is " +
					"set to false. If omitted, the current setting is kept.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"iso_id": schema.StringAttribute{
				Description: "ID of an ISO image to mount (see the cubepath_vps_isos data source). Set it to an empty " +
					"string to unmount. If omitted, whatever is mounted is left alone.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"backup_schedule_hour": schema.Int64Attribute{
				Description: "UTC hour (0-23) of the daily automatic backup. Defaults to 3.",
				Optional:    true,
				Computed:    true,
				Validators:  []validator.Int64{Int64Between(0, 23)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"backup_retention_days": schema.Int64Attribute{
				Description: "Days automatic backups are kept (1-7). Defaults to 7.",
				Optional:    true,
				Computed:    true,
				Validators:  []validator.Int64{Int64Between(1, 7)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"backup_max_backups": schema.Int64Attribute{
				Description: "Maximum number of automatic backups kept (1-10). Defaults to 7.",
				Optional:    true,
				Computed:    true,
				Validators:  []validator.Int64{Int64Between(1, 10)},
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"power_state": schema.StringAttribute{
				Description: "Desired power state of the VPS. Valid values: 'running', 'stopped'. Changing this will start or stop the VPS.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "The current status of the VPS (read-only).",
				Computed:    true,
			},
			"main_ip": schema.StringAttribute{
				Description: "The main public IPv4 address of the VPS.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6": schema.StringAttribute{
				Description: "The public IPv6 address of the VPS (read-only). Empty when ipv6_enabled is false.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"private_ip": schema.StringAttribute{
				Description: "The private IP address if attached to a network.",
				Computed:    true,
			},
			"vcpus": schema.Int64Attribute{
				Description: "Number of vCPUs.",
				Computed:    true,
			},
			"ram": schema.Int64Attribute{
				Description: "RAM in MB.",
				Computed:    true,
			},
			"storage": schema.Int64Attribute{
				Description: "Storage in GB.",
				Computed:    true,
			},
			"bandwidth": schema.Int64Attribute{
				Description: "Bandwidth in GB.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the VPS was created.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{
				Create: true,
				Delete: true,
			}),
		},
	}
}

func (r *vpsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *vpsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vpsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that at least SSH keys or password is provided
	hasSSHKeys := !plan.SSHKeyIDs.IsNull() && len(plan.SSHKeyIDs.Elements()) > 0
	hasPassword := !plan.Password.IsNull() && plan.Password.ValueString() != ""

	if !hasSSHKeys && !hasPassword {
		resp.Diagnostics.AddError(
			"Missing Authentication",
			"Either ssh_key_ids or password must be provided",
		)
		return
	}

	// Get timeout
	createTimeout, diags := plan.Timeouts.Create(ctx, 10*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Build create request
	createReq := &client.CreateVPSRequest{
		Name:         plan.Name.ValueString(),
		PlanName:     plan.PlanName.ValueString(),
		TemplateName: plan.TemplateName.ValueString(),
		LocationName: plan.Location.ValueString(),
		Label:        plan.Label.ValueString(),
	}

	if createReq.Label == "" {
		createReq.Label = createReq.Name
	}

	if !plan.NetworkID.IsNull() && !plan.NetworkID.IsUnknown() && plan.NetworkID.ValueInt64() != 0 {
		networkID := int(plan.NetworkID.ValueInt64())
		createReq.NetworkID = &networkID
	}

	if hasSSHKeys {
		var sshKeyIDs []int
		resp.Diagnostics.Append(plan.SSHKeyIDs.ElementsAs(ctx, &sshKeyIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.SSHKeyIDs = sshKeyIDs
	}

	if hasPassword {
		createReq.Password = plan.Password.ValueString()
	}

	// User configuration (defaults to root for Linux, Administrator for Windows)
	if !plan.User.IsNull() && plan.User.ValueString() != "" {
		createReq.User = plan.User.ValueString()
	}

	// IPv4 configuration (defaults to true if not specified).
	//
	// Both attributes are Optional+Computed, so an unset value arrives at Create
	// as UNKNOWN, not null. IsNull() alone is therefore false for the very case
	// this branch exists to handle, and ValueBool() on an unknown Bool returns
	// the zero value — so leaving these unset asked for a server with no public
	// IPv4 AND no public IPv6, which the API rejects with "Disabling public IPv6
	// requires a private network so the server has at least private IPv4
	// connectivity." Unknown must fall through to the documented default.
	if !plan.IPv4.IsNull() && !plan.IPv4.IsUnknown() {
		ipv4 := plan.IPv4.ValueBool()
		createReq.IPv4 = &ipv4
	} else {
		ipv4 := true
		createReq.IPv4 = &ipv4
	}

	// IPv6 configuration (defaults to true if not specified). When false, network_id must be set.
	if !plan.IPv6Enabled.IsNull() && !plan.IPv6Enabled.IsUnknown() {
		ipv6 := plan.IPv6Enabled.ValueBool()
		createReq.IPv6 = &ipv6
	} else {
		ipv6 := true
		createReq.IPv6 = &ipv6
	}

	// Backups configuration
	if !plan.EnableBackups.IsNull() && !plan.EnableBackups.IsUnknown() {
		backups := plan.EnableBackups.ValueBool()
		createReq.EnableBackups = &backups
	}

	// Custom cloud-init
	if !plan.CustomCloudInit.IsNull() && plan.CustomCloudInit.ValueString() != "" {
		cloudinit := plan.CustomCloudInit.ValueString()
		createReq.CustomCloudInit = &cloudinit
	}

	// Firewall groups (passed during creation instead of post-create)
	if !plan.FirewallGroupIDs.IsNull() && len(plan.FirewallGroupIDs.Elements()) > 0 {
		var groupIDs []int
		resp.Diagnostics.Append(plan.FirewallGroupIDs.ElementsAs(ctx, &groupIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.FirewallGroupIDs = groupIDs
	}

	// Availability group for HA placement
	if !plan.AvailabilityGroupUUID.IsNull() && plan.AvailabilityGroupUUID.ValueString() != "" {
		uuid := plan.AvailabilityGroupUUID.ValueString()
		createReq.AvailabilityGroupUUID = &uuid
	}

	// Create VPS
	taskResp, err := r.client.VPS.Create(createCtx, int(plan.ProjectID.ValueInt64()), createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating VPS",
			"Could not create VPS: "+err.Error(),
		)
		return
	}

	// Parse VPS ID from task response message
	// The API typically returns the VPS ID, we need to extract it
	// For now, we'll need to wait and then search for it
	time.Sleep(5 * time.Second)

	// Find the VPS by searching in the project
	projectResp, err := r.client.Projects.Get(createCtx, int(plan.ProjectID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VPS after creation",
			"VPS was created but could not be read: "+err.Error(),
		)
		return
	}

	// Find the VPS we just created by name
	var createdVPS *client.VPS
	for _, vps := range projectResp.VPS {
		if vps.Name == plan.Name.ValueString() {
			createdVPS = &vps
			break
		}
	}

	if createdVPS == nil {
		resp.Diagnostics.AddError(
			"Error finding VPS after creation",
			fmt.Sprintf("VPS with name '%s' not found. Task ID: %s", plan.Name.ValueString(), taskResp.TaskID),
		)
		return
	}

	// Wait for VPS to be active (API uses "active" instead of "running")
	vps, err := r.client.VPS.WaitForVPSStatus(createCtx, createdVPS.ID, []string{"active", "running"}, createTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Timeout waiting for VPS",
			"VPS was created but did not reach running state: "+err.Error(),
		)
		return
	}

	// Save the ID right away so a failed follow-up call does not leave an untracked VPS behind.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strconv.Itoa(vps.ID))...)

	// Settings with their own endpoints: protection, ISO and backup schedule.
	r.applyInPlaceChanges(createCtx, vps.ID, &plan, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if vps, err = r.client.VPS.Get(createCtx, vps.ID); err != nil {
		resp.Diagnostics.AddError("Error reading VPS after creation", err.Error())
		return
	}

	// Update state
	r.updateStateFromVPS(ctx, &plan, vps, false, &resp.Diagnostics)
	r.readBackupSettings(ctx, vps.ID, &plan, false, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vpsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vpsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing VPS ID",
			"Could not parse VPS ID: "+err.Error(),
		)
		return
	}

	vps, err := r.client.VPS.Get(ctx, id)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error reading VPS",
			"Could not read VPS: "+err.Error(),
		)
		return
	}

	r.updateStateFromVPS(ctx, &state, vps, true, &resp.Diagnostics)
	r.readBackupSettings(ctx, vps.ID, &state, true, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vpsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan vpsResourceModel
	var state vpsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing VPS ID",
			"Could not parse VPS ID: "+err.Error(),
		)
		return
	}

	// Hostname and label are changed in place (PATCH /vps/update/{id}).
	updateReq := &client.UpdateVPSRequest{}
	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
	}
	if !plan.Label.IsNull() && !plan.Label.IsUnknown() && !plan.Label.Equal(state.Label) {
		label := plan.Label.ValueString()
		updateReq.Label = &label
	}
	if updateReq.Name != nil || updateReq.Label != nil {
		err := r.client.VPS.Update(ctx, id, updateReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating VPS",
				"Could not update VPS hostname or label: "+err.Error(),
			)
			return
		}
	}

	// Check if plan changed (resize needed)
	if !plan.PlanName.Equal(state.PlanName) {
		_, err := r.client.VPS.Resize(ctx, id, plan.PlanName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error resizing VPS",
				"Could not resize VPS: "+err.Error(),
			)
			return
		}

		// Wait for resize to complete
		time.Sleep(30 * time.Second)
	}

	// Check if password changed
	if !plan.Password.IsNull() && !plan.Password.Equal(state.Password) {
		_, err := r.client.VPS.ChangePassword(ctx, id, plan.Password.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error changing VPS password",
				"Could not change VPS password: "+err.Error(),
			)
			return
		}
	}

	// Check if firewall groups changed
	if !plan.FirewallGroupIDs.IsUnknown() && !plan.FirewallGroupIDs.Equal(state.FirewallGroupIDs) {
		var groupIDs []int
		if !plan.FirewallGroupIDs.IsNull() {
			resp.Diagnostics.Append(plan.FirewallGroupIDs.ElementsAs(ctx, &groupIDs, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}

		_, err := r.client.Firewall.UpdateVPSGroups(ctx, id, groupIDs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating VPS firewall groups",
				"Could not update VPS firewall groups: "+err.Error(),
			)
			return
		}
	}

	// Check if power state changed
	if !plan.PowerState.IsNull() && !plan.PowerState.Equal(state.PowerState) {
		desiredState := plan.PowerState.ValueString()

		switch desiredState {
		case "running":
			_, err := r.client.VPS.Start(ctx, id)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error starting VPS",
					"Could not start VPS: "+err.Error(),
				)
				return
			}
			// Wait for VPS to be running
			_, err = r.client.VPS.WaitForVPSStatus(ctx, id, []string{"running", "active"}, 5*time.Minute)
			if err != nil {
				resp.Diagnostics.AddError(
					"Timeout waiting for VPS to start",
					"VPS did not reach running state: "+err.Error(),
				)
				return
			}
		case "stopped":
			_, err := r.client.VPS.Stop(ctx, id)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error stopping VPS",
					"Could not stop VPS: "+err.Error(),
				)
				return
			}
			// Wait for VPS to be stopped
			_, err = r.client.VPS.WaitForVPSStatus(ctx, id, []string{"stopped"}, 5*time.Minute)
			if err != nil {
				resp.Diagnostics.AddError(
					"Timeout waiting for VPS to stop",
					"VPS did not reach stopped state: "+err.Error(),
				)
				return
			}
		default:
			resp.Diagnostics.AddError(
				"Invalid power_state",
				fmt.Sprintf("power_state must be 'running' or 'stopped', got: %s", desiredState),
			)
			return
		}
	}

	// Project, protection, network, SSH keys, availability group, backups and ISO.
	r.applyInPlaceChanges(ctx, id, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read updated VPS
	vps, err := r.client.VPS.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VPS after update",
			"Could not read VPS: "+err.Error(),
		)
		return
	}

	r.updateStateFromVPS(ctx, &plan, vps, false, &resp.Diagnostics)
	r.readBackupSettings(ctx, id, &plan, false, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *vpsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vpsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteTimeout, diags := state.Timeouts.Delete(ctx, 5*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteCtx, cancel := context.WithTimeout(ctx, deleteTimeout)
	defer cancel()

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing VPS ID",
			"Could not parse VPS ID: "+err.Error(),
		)
		return
	}

	_, err = r.client.VPS.Destroy(deleteCtx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error destroying VPS",
			"Could not destroy VPS: "+err.Error(),
		)
		return
	}

	// Wait for VPS to be destroyed
	err = r.client.VPS.WaitForVPSDestroy(deleteCtx, id, deleteTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Timeout waiting for VPS deletion",
			"VPS destroy was initiated but did not complete: "+err.Error(),
		)
		return
	}
}

func (r *vpsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// updateStateFromVPS copies the API view of a VPS into state. refresh is true
// when called from Read: then firewall_group_ids is always taken from the API
// so out-of-band changes show up as drift. Create-only attributes are only
// filled in when unknown or null (e.g. after an import), never overwritten.
func (r *vpsResource) updateStateFromVPS(ctx context.Context, state *vpsResourceModel, vps *client.VPS, refresh bool, diags *diag.Diagnostics) {
	state.ID = types.StringValue(strconv.Itoa(vps.ID))
	state.Name = types.StringValue(vps.Name)
	state.Label = types.StringValue(vps.Label)
	state.Status = types.StringValue(vps.Status)
	state.IPv6 = types.StringValue(vps.IPv6)
	state.CreatedAt = types.StringValue(vps.CreatedAt.String())

	// Resource configuration fields
	// Only update project_id if it's non-zero (API returns null in nested responses)
	if vps.ProjectID != 0 {
		state.ProjectID = types.Int64Value(int64(vps.ProjectID))
	}
	state.Location = types.StringValue(vps.Location.LocationName)
	state.PlanName = types.StringValue(vps.Plan.PlanName)
	state.TemplateName = types.StringValue(vps.Template.TemplateName)

	// Plan details
	state.VCPUs = types.Int64Value(int64(vps.Plan.CPU))
	state.RAM = types.Int64Value(int64(vps.Plan.RAM))
	state.Storage = types.Int64Value(int64(vps.Plan.Storage))
	state.Bandwidth = types.Int64Value(int64(vps.Plan.Bandwidth))

	// User field from API
	if vps.User != "" {
		state.User = types.StringValue(vps.User)
	} else if state.User.IsNull() {
		state.User = types.StringValue("root")
	}

	// Extract main IP from floating IPs and detect IPv4 status
	// FloatingIPs structure is: {"list": [...], "price_per_hour": ...}
	var floatingIPsResp struct {
		List []client.FloatingIP `json:"list"`
	}
	hasIPv4 := false
	hasIPv6 := false
	if err := json.Unmarshal(vps.FloatingIPs, &floatingIPsResp); err == nil {
		for _, ip := range floatingIPsResp.List {
			switch ip.Type {
			case "IPv4":
				if !hasIPv4 {
					state.MainIP = types.StringValue(ip.Address)
				}
				hasIPv4 = true
			case "IPv6":
				// The VPS payload leaves ipv6 empty: the address is in the floating IP list.
				if !hasIPv6 && vps.IPv6 == "" {
					state.IPv6 = types.StringValue(ip.Address)
				}
				hasIPv6 = true
			}
		}
	}
	if !hasIPv4 {
		state.MainIP = types.StringValue("")
	}

	// Set IPv4 based on whether VPS has an IPv4 address
	state.IPv4 = types.BoolValue(hasIPv4)

	// ipv6_enabled is Optional+Computed, so it MUST be known after apply or
	// Terraform aborts with "Provider returned invalid result object after
	// apply ... always a bug in the provider" — which it did, leaving a created
	// VPS reported as a failed apply. Derive it the same way ipv4 is derived,
	// falling back to the read-only address when the floating-IP list carries
	// no v6 entry.
	state.IPv6Enabled = types.BoolValue(hasIPv6 || vps.IPv6 != "")

	// CustomCloudInit is write-only and kept from state. EnableBackups is only
	// taken from the API when it isn't known yet (unset in config, or import).
	if state.EnableBackups.IsNull() || state.EnableBackups.IsUnknown() {
		state.EnableBackups = types.BoolValue(vps.BackupEnabled)
	}

	// Set PowerState based on VPS status
	// Map API status to power_state: running/active -> "running", stopped -> "stopped"
	switch vps.Status {
	case "running", "active":
		state.PowerState = types.StringValue("running")
	case "stopped":
		state.PowerState = types.StringValue("stopped")
	default:
		// For transitional states (deploying, starting, stopping), preserve current state
		if state.PowerState.IsNull() {
			state.PowerState = types.StringValue("running")
		}
	}

	// Private IP from network
	if vps.Network != nil {
		state.PrivateIP = types.StringValue(vps.Network.AssignedIP)
	} else {
		state.PrivateIP = types.StringNull()
	}

	state.Protected = types.BoolValue(vps.Protected)

	// iso_id: "" means nothing mounted. Attaching is asynchronous, so after an apply the
	// planned value is kept.
	if refresh || state.ISOID.IsNull() || state.ISOID.IsUnknown() {
		if vps.MountedISO != nil {
			state.ISOID = types.StringValue(vps.MountedISO.ID)
		} else {
			state.ISOID = types.StringValue("")
		}
	}

	if refresh || state.NetworkID.IsNull() || state.NetworkID.IsUnknown() {
		if vps.Network != nil && vps.Network.ID != 0 {
			state.NetworkID = types.Int64Value(int64(vps.Network.ID))
		} else if !state.NetworkID.IsNull() && !state.NetworkID.IsUnknown() && state.NetworkID.ValueInt64() == 0 {
			// network_id = 0 in the configuration means "no private network".
		} else {
			state.NetworkID = types.Int64Null()
		}
	}

	if refresh || state.SSHKeyIDs.IsNull() || state.SSHKeyIDs.IsUnknown() {
		ids := make([]int64, 0, len(vps.SSHKeys))
		for _, key := range vps.SSHKeys {
			ids = append(ids, int64(key.ID))
		}
		state.SSHKeyIDs = int64SetOrNull(ctx, ids, state.SSHKeyIDs, diags)
	}

	if refresh || state.FirewallGroupIDs.IsNull() || state.FirewallGroupIDs.IsUnknown() {
		ids := make([]int64, 0, len(vps.FirewallGroups))
		for _, group := range vps.FirewallGroups {
			ids = append(ids, int64(group.ID))
		}
		state.FirewallGroupIDs = int64SetOrNull(ctx, ids, state.FirewallGroupIDs, diags)
	}

	if refresh || state.AvailabilityGroupUUID.IsNull() || state.AvailabilityGroupUUID.IsUnknown() {
		group := r.findAvailabilityGroup(ctx, state.ProjectID, vps.ID, diags)
		// An empty string in the configuration means "no group": keep it rather than null.
		if group.IsNull() && !state.AvailabilityGroupUUID.IsNull() && !state.AvailabilityGroupUUID.IsUnknown() &&
			state.AvailabilityGroupUUID.ValueString() == "" {
			group = types.StringValue("")
		}
		state.AvailabilityGroupUUID = group
	}
}

// findAvailabilityGroup returns the UUID of the availability group containing
// the VPS, or null. The VPS payload does not include it, so the project's
// groups are searched instead.
func (r *vpsResource) findAvailabilityGroup(ctx context.Context, projectID types.Int64, vpsID int, diags *diag.Diagnostics) types.String {
	if projectID.IsNull() || projectID.IsUnknown() {
		return types.StringNull()
	}

	groups, err := r.client.AvailabilityGroups.List(ctx, int(projectID.ValueInt64()))
	if err != nil {
		diags.AddWarning(
			"Could not read availability groups",
			fmt.Sprintf("availability_group_uuid could not be determined for VPS %d: %s", vpsID, err),
		)
		return types.StringNull()
	}

	for _, group := range groups {
		for _, member := range group.VPSList {
			if member.ID == vpsID {
				return types.StringValue(group.UUID)
			}
		}
	}

	return types.StringNull()
}

// int64SetOrNull builds a set from ids. An empty result is stored as null,
// unless the previous value was a known empty set (configured as []).
func int64SetOrNull(ctx context.Context, ids []int64, previous types.Set, diags *diag.Diagnostics) types.Set {
	if len(ids) == 0 && (previous.IsNull() || previous.IsUnknown()) {
		return types.SetNull(types.Int64Type)
	}

	set, d := types.SetValueFrom(ctx, types.Int64Type, ids)
	diags.Append(d...)
	return set
}

// requiresReplaceIfConfiguredString forces replacement only when the attribute is set in
// the configuration. Omitting an Optional+Computed attribute keeps the value read from the
// API instead of replacing the resource.
func requiresReplaceIfConfiguredString(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = !req.ConfigValue.IsNull()
}

// requiresReplaceIfKnownInState forces replacement only when the prior state
// holds a value, so filling in a write-only attribute after import is an
// in-place update.
func requiresReplaceIfKnownInState(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = !req.StateValue.IsNull()
}

// applyInPlaceChanges applies the attributes that have their own endpoints. prior is nil
// on create, where network, SSH keys and availability group were already part of the
// create request.
func (r *vpsResource) applyInPlaceChanges(ctx context.Context, id int, plan, prior *vpsResourceModel, diags *diag.Diagnostics) {
	if prior != nil && !plan.ProjectID.Equal(prior.ProjectID) {
		if err := r.client.VPS.MoveProject(ctx, id, int(plan.ProjectID.ValueInt64())); err != nil {
			diags.AddError("Error moving VPS", err.Error())
			return
		}
	}

	if want := knownBool(plan.Protected); want != nil {
		changed := *want // on create only enabling needs a call
		if prior != nil {
			changed = !plan.Protected.Equal(prior.Protected)
		}
		if changed {
			if err := r.client.VPS.SetProtection(ctx, id, *want); err != nil {
				diags.AddError("Error changing VPS protection", err.Error())
				return
			}
		}
	}

	if prior != nil {
		r.applyNetwork(ctx, id, plan, prior, diags)
		if diags.HasError() {
			return
		}

		if !plan.SSHKeyIDs.IsUnknown() && !plan.SSHKeyIDs.Equal(prior.SSHKeyIDs) {
			add, remove := diffInts(intsFromSet(ctx, prior.SSHKeyIDs, diags), intsFromSet(ctx, plan.SSHKeyIDs, diags))
			for _, key := range remove {
				if err := r.client.VPS.RemoveSSHKey(ctx, id, int(key)); err != nil {
					diags.AddError("Error removing SSH key from VPS", err.Error())
					return
				}
			}
			if len(add) > 0 {
				keys := make([]int, 0, len(add))
				for _, key := range add {
					keys = append(keys, int(key))
				}
				if err := r.client.VPS.AddSSHKeys(ctx, id, keys); err != nil {
					diags.AddError("Error adding SSH keys to VPS", err.Error())
					return
				}
			}
		}

		if !plan.AvailabilityGroupUUID.IsUnknown() && !plan.AvailabilityGroupUUID.Equal(prior.AvailabilityGroupUUID) {
			if old := prior.AvailabilityGroupUUID.ValueString(); old != "" {
				if err := r.client.AvailabilityGroups.RemoveVPS(ctx, old, id); err != nil && !isNotFound(err) {
					diags.AddError("Error removing VPS from availability group", err.Error())
					return
				}
			}
			if group := plan.AvailabilityGroupUUID.ValueString(); group != "" {
				if err := r.client.AvailabilityGroups.AddVPS(ctx, group, id); err != nil {
					diags.AddError("Error adding VPS to availability group", err.Error())
					return
				}
			}
		}
	}

	r.applyBackupSettings(ctx, id, plan, prior, diags)
	if diags.HasError() {
		return
	}

	if want := knownString(plan.ISOID); want != nil {
		have := ""
		if prior != nil {
			have = prior.ISOID.ValueString()
		}
		if *want != have {
			if have != "" {
				if err := r.client.VPS.DetachISO(ctx, id); err != nil {
					diags.AddError("Error unmounting ISO", err.Error())
					return
				}
			}
			if *want != "" {
				// A detach queued just before answers 409 until it is done.
				err := retryWhile(ctx, 5*time.Minute, 10*time.Second, isConflict, func() error {
					return r.client.VPS.AttachISO(ctx, id, *want)
				})
				if err != nil {
					diags.AddError("Error mounting ISO", err.Error())
				}
			}
		}
	}
}

// applyNetwork moves the VPS to the planned private network: detach the old one, then
// attach the new one once the detach task is done.
func (r *vpsResource) applyNetwork(ctx context.Context, id int, plan, prior *vpsResourceModel, diags *diag.Diagnostics) {
	if plan.NetworkID.IsUnknown() || plan.NetworkID.Equal(prior.NetworkID) {
		return
	}
	if !prior.NetworkID.IsNull() && prior.NetworkID.ValueInt64() != 0 {
		if err := r.client.VPS.DetachNetwork(ctx, id); err != nil {
			diags.AddError("Error detaching private network", err.Error())
			return
		}
	}
	if plan.NetworkID.IsNull() || plan.NetworkID.ValueInt64() == 0 {
		return
	}
	networkID := int(plan.NetworkID.ValueInt64())
	err := retryWhile(ctx, 5*time.Minute, 10*time.Second, isConflict, func() error {
		return r.client.VPS.AttachNetwork(ctx, id, networkID)
	})
	if err != nil {
		diags.AddError("Error attaching private network", err.Error())
	}
}

// applyBackupSettings updates the automatic backup settings when one of them changed.
// The API replaces all four values at once, so unset ones keep their current value.
func (r *vpsResource) applyBackupSettings(ctx context.Context, id int, plan, prior *vpsResourceModel, diags *diag.Diagnostics) {
	changed := func(p, s types.Int64) bool {
		if p.IsUnknown() || p.IsNull() {
			return false
		}
		return prior == nil || !p.Equal(s)
	}
	var priorHour, priorRetention, priorMax types.Int64
	enabledChanged := false
	if prior != nil {
		priorHour, priorRetention, priorMax = prior.BackupScheduleHour, prior.BackupRetentionDays, prior.BackupMaxBackups
		enabledChanged = !plan.EnableBackups.IsUnknown() && !plan.EnableBackups.IsNull() && !plan.EnableBackups.Equal(prior.EnableBackups)
	}
	if !enabledChanged && !changed(plan.BackupScheduleHour, priorHour) &&
		!changed(plan.BackupRetentionDays, priorRetention) && !changed(plan.BackupMaxBackups, priorMax) {
		return
	}

	settings, err := r.client.VPS.GetBackupSettings(ctx, id)
	if err != nil {
		diags.AddError("Error reading VPS backup settings", err.Error())
		return
	}
	if v := knownBool(plan.EnableBackups); v != nil {
		settings.Enabled = *v
	}
	if v := knownInt(plan.BackupScheduleHour); v != nil {
		settings.ScheduleHour = *v
	}
	if v := knownInt(plan.BackupRetentionDays); v != nil {
		settings.RetentionDays = *v
	}
	if v := knownInt(plan.BackupMaxBackups); v != nil {
		settings.MaxBackups = *v
	}
	if _, err := r.client.VPS.UpdateBackupSettings(ctx, id, settings); err != nil {
		diags.AddError("Error updating VPS backup settings", err.Error())
	}
}

// readBackupSettings fills the backup schedule attributes. On refresh enable_backups is
// taken from the settings too, so changes made in the dashboard show up as drift.
func (r *vpsResource) readBackupSettings(ctx context.Context, id int, state *vpsResourceModel, refresh bool, diags *diag.Diagnostics) {
	settings, err := r.client.VPS.GetBackupSettings(ctx, id)
	if err != nil {
		diags.AddWarning("Could not read VPS backup settings", err.Error())
		for _, v := range []*types.Int64{&state.BackupScheduleHour, &state.BackupRetentionDays, &state.BackupMaxBackups} {
			if v.IsUnknown() {
				*v = types.Int64Null()
			}
		}
		return
	}
	state.BackupScheduleHour = types.Int64Value(int64(settings.ScheduleHour))
	state.BackupRetentionDays = types.Int64Value(int64(settings.RetentionDays))
	state.BackupMaxBackups = types.Int64Value(int64(settings.MaxBackups))
	if refresh {
		state.EnableBackups = types.BoolValue(settings.Enabled)
	}
}
