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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
	SSHKeyNames           types.List     `tfsdk:"ssh_key_names"`
	User                  types.String   `tfsdk:"user"`
	Password              types.String   `tfsdk:"password"`
	IPv4                  types.Bool     `tfsdk:"ipv4"`
	EnableBackups         types.Bool     `tfsdk:"enable_backups"`
	CustomCloudInit       types.String   `tfsdk:"custom_cloudinit"`
	FirewallGroupIDs      types.List     `tfsdk:"firewall_group_ids"`
	AvailabilityGroupUUID types.String   `tfsdk:"availability_group_uuid"`
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
				Description: "The hostname of the VPS.",
				Required:    true,
				Validators:  []validator.String{HostnameValidator()},
			},
			"label": schema.StringAttribute{
				Description: "A label for the VPS.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID to associate the VPS with.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
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
				Description: "Optional private network ID to attach to the VPS.",
				Optional:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"ssh_key_names": schema.ListAttribute{
				Description: "List of SSH key names to add to the VPS.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
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
				Description: "Root password for the VPS. Required if ssh_key_names is not provided.",
				Optional:    true,
				Sensitive:   true,
				Validators:  []validator.String{StrongPasswordValidator()},
			},
			"ipv4": schema.BoolAttribute{
				Description: "Enable IPv4 address (dual-stack). Additional $1.50/month. IPv6 is always included for free. Defaults to true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"enable_backups": schema.BoolAttribute{
				Description: "Enable automatic backups for this VPS. Defaults to false.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"custom_cloudinit": schema.StringAttribute{
				Description: "Custom cloud-init configuration (YAML). Only for Linux templates.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"firewall_group_ids": schema.ListAttribute{
				Description: "List of firewall group IDs to assign to this VPS.",
				Optional:    true,
				ElementType: types.Int64Type,
			},
			"availability_group_uuid": schema.StringAttribute{
				Description: "UUID of the availability group to place this VPS in. VPS in the same availability group are distributed across different physical hosts.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
			},
			"ipv6": schema.StringAttribute{
				Description: "The IPv6 address of the VPS.",
				Computed:    true,
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
	hasSSHKeys := !plan.SSHKeyNames.IsNull() && len(plan.SSHKeyNames.Elements()) > 0
	hasPassword := !plan.Password.IsNull() && plan.Password.ValueString() != ""

	if !hasSSHKeys && !hasPassword {
		resp.Diagnostics.AddError(
			"Missing Authentication",
			"Either ssh_key_names or password must be provided",
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

	if !plan.NetworkID.IsNull() {
		networkID := int(plan.NetworkID.ValueInt64())
		createReq.NetworkID = &networkID
	}

	if hasSSHKeys {
		var sshKeyNames []string
		resp.Diagnostics.Append(plan.SSHKeyNames.ElementsAs(ctx, &sshKeyNames, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.SSHKeyNames = sshKeyNames
	}

	if hasPassword {
		createReq.Password = plan.Password.ValueString()
	}

	// User configuration (defaults to root for Linux, Administrator for Windows)
	if !plan.User.IsNull() && plan.User.ValueString() != "" {
		createReq.User = plan.User.ValueString()
	}

	// IPv4 configuration (defaults to true if not specified)
	if !plan.IPv4.IsNull() {
		ipv4 := plan.IPv4.ValueBool()
		createReq.IPv4 = &ipv4
	} else {
		ipv4 := true
		createReq.IPv4 = &ipv4
	}

	// Backups configuration
	if !plan.EnableBackups.IsNull() {
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

	// Update state
	r.updateStateFromVPS(ctx, &plan, vps, &resp.Diagnostics)

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

	r.updateStateFromVPS(ctx, &state, vps, &resp.Diagnostics)

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

	// Check if name changed
	if !plan.Name.Equal(state.Name) {
		err := r.client.VPS.Rename(ctx, id, plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error renaming VPS",
				"Could not rename VPS: "+err.Error(),
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
	if !plan.FirewallGroupIDs.Equal(state.FirewallGroupIDs) {
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

	// Read updated VPS
	vps, err := r.client.VPS.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VPS after update",
			"Could not read VPS: "+err.Error(),
		)
		return
	}

	r.updateStateFromVPS(ctx, &plan, vps, &resp.Diagnostics)

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

func (r *vpsResource) updateStateFromVPS(ctx context.Context, state *vpsResourceModel, vps *client.VPS, diags *diag.Diagnostics) {
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
	if err := json.Unmarshal(vps.FloatingIPs, &floatingIPsResp); err == nil {
		for _, ip := range floatingIPsResp.List {
			if ip.Type == "IPv4" {
				state.MainIP = types.StringValue(ip.Address)
				hasIPv4 = true
				break
			}
		}
	}
	if !hasIPv4 {
		state.MainIP = types.StringValue("")
	}

	// Set IPv4 based on whether VPS has an IPv4 address
	state.IPv4 = types.BoolValue(hasIPv4)

	// EnableBackups and CustomCloudInit are write-only, preserve from state if not null/unknown
	if state.EnableBackups.IsNull() || state.EnableBackups.IsUnknown() {
		state.EnableBackups = types.BoolValue(false)
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
}
