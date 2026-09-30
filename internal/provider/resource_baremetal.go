package provider

import (
	"context"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &baremetalResource{}
	_ resource.ResourceWithConfigure   = &baremetalResource{}
	_ resource.ResourceWithImportState = &baremetalResource{}
)

func NewBaremetalResource() resource.Resource {
	return &baremetalResource{}
}

type baremetalResource struct {
	client *client.Client
}

type baremetalResourceModel struct {
	ID               types.String   `tfsdk:"id"`
	Hostname         types.String   `tfsdk:"hostname"`
	Label            types.String   `tfsdk:"label"`
	ProjectID        types.Int64    `tfsdk:"project_id"`
	Location         types.String   `tfsdk:"location"`
	ModelName        types.String   `tfsdk:"model_name"`
	OSName           types.String   `tfsdk:"os_name"`
	DiskLayoutName   types.String   `tfsdk:"disk_layout_name"`
	User             types.String   `tfsdk:"user"`
	Password         types.String   `tfsdk:"password"`
	SSHKeyIDs        types.List     `tfsdk:"ssh_key_ids"`
	MonitoringEnable types.Bool     `tfsdk:"monitoring_enable"`
	Protected        types.Bool     `tfsdk:"protected"`
	NetworkID        types.Int64    `tfsdk:"network_id"`
	PrivateIP        types.String   `tfsdk:"private_ip"`
	PowerState       types.String   `tfsdk:"power_state"`
	Status           types.String   `tfsdk:"status"`
	MainIP           types.String   `tfsdk:"main_ip"`
	IPv6             types.String   `tfsdk:"ipv6"`
	CPU              types.String   `tfsdk:"cpu"`
	RAM              types.Int64    `tfsdk:"ram"`
	StorageType      types.String   `tfsdk:"storage_type"`
	DiskSize         types.String   `tfsdk:"disk_size"`
	CreatedAt        types.String   `tfsdk:"created_at"`
	Timeouts         timeouts.Value `tfsdk:"timeouts"`
}

func (r *baremetalResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_baremetal"
}

func (r *baremetalResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Baremetal server on CubePath Cloud. Destroying this resource (terraform destroy, or removing it from the configuration) only removes it from the Terraform state: the server is still running and billed, and has to be cancelled from the CubePath dashboard.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the Baremetal server.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hostname": schema.StringAttribute{
				Description: "The hostname of the Baremetal server.",
				Required:    true,
				Validators:  []validator.String{HostnameValidator()},
			},
			"label": schema.StringAttribute{
				Description: "A label for the Baremetal server.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The project ID to associate the Baremetal server with. Changing it moves the server to the other project in place.",
				Required:    true,
			},
			"location": schema.StringAttribute{
				Description: "The location where the Baremetal server will be deployed (e.g., 'us-mia-1').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"model_name": schema.StringAttribute{
				Description: "The model name for the Baremetal server (e.g., 'c1.metal.plus').",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"os_name": schema.StringAttribute{
				Description: "The operating system to install (e.g., 'debian-12').",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"disk_layout_name": schema.StringAttribute{
				Description: "The disk layout configuration (e.g., 'single', 'raid1').",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Description: "SSH user for the server. Defaults to 'root'.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Description: "Root password for the Baremetal server.",
				Required:    true,
				Sensitive:   true,
				Validators:  []validator.String{StrongPasswordValidator()},
			},
			"ssh_key_ids": schema.ListAttribute{
				Description: "List of SSH key IDs to add to the server. Changing it attaches and detaches keys in place; " +
					"the server only picks up the new keys on its next reinstall.",
				Optional:    true,
				ElementType: types.Int64Type,
			},
			"protected": schema.BoolAttribute{
				Description: "Protection against reinstallation. If omitted, the current setting is kept.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"network_id": schema.Int64Attribute{
				Description: "Private network to attach the server to. Changing it detaches the old network and attaches " +
					"the new one in place; the new interface is active after the server restarts. Set it to 0 to detach it.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"private_ip": schema.StringAttribute{
				Description: "Address of the server in the private network.",
				Computed:    true,
			},
			"monitoring_enable": schema.BoolAttribute{
				Description: "Enable monitoring for this Baremetal server.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"power_state": schema.StringAttribute{
				Description: "Desired power state of the server. Valid values: 'running', 'stopped'.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "The current status of the Baremetal server (read-only).",
				Computed:    true,
			},
			"main_ip": schema.StringAttribute{
				Description: "The main public IPv4 address of the server.",
				Computed:    true,
			},
			"ipv6": schema.StringAttribute{
				Description: "The IPv6 address of the server.",
				Computed:    true,
			},
			"cpu": schema.StringAttribute{
				Description: "CPU model of the server.",
				Computed:    true,
			},
			"ram": schema.Int64Attribute{
				Description: "RAM in GB.",
				Computed:    true,
			},
			"storage_type": schema.StringAttribute{
				Description: "Storage type (SSD, HDD, NVMe).",
				Computed:    true,
			},
			"disk_size": schema.StringAttribute{
				Description: "Total disk size.",
				Computed:    true,
			},
			"created_at": schema.StringAttribute{
				Description: "The timestamp when the server was deployed.",
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

func (r *baremetalResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *baremetalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan baremetalResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get timeout
	createTimeout, diags := plan.Timeouts.Create(ctx, 30*time.Minute)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createCtx, cancel := context.WithTimeout(ctx, createTimeout)
	defer cancel()

	// Build create request
	createReq := &client.CreateBaremetalRequest{
		ModelName:    plan.ModelName.ValueString(),
		LocationName: plan.Location.ValueString(),
		Hostname:     plan.Hostname.ValueString(),
		Password:     plan.Password.ValueString(),
	}

	if !plan.Label.IsNull() && plan.Label.ValueString() != "" {
		createReq.Label = plan.Label.ValueString()
	} else {
		createReq.Label = plan.Hostname.ValueString()
	}

	if !plan.User.IsNull() && plan.User.ValueString() != "" {
		createReq.User = plan.User.ValueString()
	}

	if !plan.OSName.IsNull() && plan.OSName.ValueString() != "" {
		createReq.OSName = plan.OSName.ValueString()
	}

	if !plan.DiskLayoutName.IsNull() && plan.DiskLayoutName.ValueString() != "" {
		createReq.DiskLayoutName = plan.DiskLayoutName.ValueString()
	}

	if !plan.SSHKeyIDs.IsNull() && len(plan.SSHKeyIDs.Elements()) > 0 {
		var sshKeyIDs []int
		resp.Diagnostics.Append(plan.SSHKeyIDs.ElementsAs(ctx, &sshKeyIDs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.SSHKeyIDs = sshKeyIDs
	}

	// Deploy baremetal
	_, err := r.client.Baremetal.Deploy(createCtx, int(plan.ProjectID.ValueInt64()), createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deploying Baremetal",
			"Could not deploy Baremetal server: "+err.Error(),
		)
		return
	}

	// Wait a bit then search for the baremetal
	time.Sleep(10 * time.Second)

	// Find the baremetal we just created by hostname
	projectResp, err := r.client.Projects.Get(createCtx, int(plan.ProjectID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Baremetal after deployment",
			"Baremetal was deployed but could not be read: "+err.Error(),
		)
		return
	}

	var createdBaremetal *client.Baremetal
	for _, bm := range projectResp.Baremetals {
		if bm.Hostname == plan.Hostname.ValueString() {
			createdBaremetal = &bm
			break
		}
	}

	if createdBaremetal == nil {
		resp.Diagnostics.AddError(
			"Error finding Baremetal after deployment",
			fmt.Sprintf("Baremetal with hostname '%s' not found", plan.Hostname.ValueString()),
		)
		return
	}

	// Wait for baremetal to be active
	baremetal, err := r.client.Baremetal.WaitForBaremetalStatus(createCtx, createdBaremetal.ID, []string{"active", "running"}, createTimeout)
	if err != nil {
		resp.Diagnostics.AddError(
			"Timeout waiting for Baremetal",
			"Baremetal was deployed but did not reach running state: "+err.Error(),
		)
		return
	}

	// Update monitoring if specified
	if !plan.MonitoringEnable.IsNull() {
		err := r.client.Baremetal.UpdateMonitoring(ctx, baremetal.ID, plan.MonitoringEnable.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating Baremetal monitoring",
				"Baremetal was deployed but monitoring could not be configured: "+err.Error(),
			)
			return
		}
	}

	// Save the ID right away so a failed follow-up call does not lose track of the server.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strconv.Itoa(baremetal.ID))...)

	if plan.Protected.ValueBool() {
		if err := r.client.Baremetal.SetProtection(createCtx, baremetal.ID, true); err != nil {
			resp.Diagnostics.AddError("Error enabling Baremetal protection", err.Error())
			return
		}
	}
	if networkID := knownInt(plan.NetworkID); networkID != nil && *networkID != 0 {
		if err := r.client.Baremetal.AttachNetwork(createCtx, baremetal.ID, *networkID); err != nil {
			resp.Diagnostics.AddError("Error attaching private network", err.Error())
			return
		}
	}
	if baremetal, err = r.client.Baremetal.Get(createCtx, baremetal.ID); err != nil {
		resp.Diagnostics.AddError("Error reading Baremetal after deployment", err.Error())
		return
	}

	// Update state
	r.updateStateFromBaremetal(ctx, &plan, baremetal, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *baremetalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state baremetalResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Baremetal ID",
			"Could not parse Baremetal ID: "+err.Error(),
		)
		return
	}

	baremetal, err := r.client.Baremetal.Get(ctx, id)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Error reading Baremetal",
			"Could not read Baremetal: "+err.Error(),
		)
		return
	}

	r.updateStateFromBaremetal(ctx, &state, baremetal, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *baremetalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan baremetalResourceModel
	var state baremetalResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing Baremetal ID",
			"Could not parse Baremetal ID: "+err.Error(),
		)
		return
	}

	// Check if hostname or label changed
	if !plan.Hostname.Equal(state.Hostname) || !plan.Label.Equal(state.Label) {
		updateReq := &client.UpdateBaremetalRequest{}

		if !plan.Hostname.Equal(state.Hostname) {
			hostname := plan.Hostname.ValueString()
			updateReq.Hostname = &hostname
		}

		if !plan.Label.Equal(state.Label) {
			label := plan.Label.ValueString()
			updateReq.Label = &label
		}

		err := r.client.Baremetal.Update(ctx, id, updateReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating Baremetal",
				"Could not update Baremetal: "+err.Error(),
			)
			return
		}
	}

	r.applyInPlaceChanges(ctx, id, &plan, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if monitoring changed
	if !plan.MonitoringEnable.Equal(state.MonitoringEnable) {
		err := r.client.Baremetal.UpdateMonitoring(ctx, id, plan.MonitoringEnable.ValueBool())
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating Baremetal monitoring",
				"Could not update monitoring: "+err.Error(),
			)
			return
		}
	}

	// Check if power state changed
	if !plan.PowerState.IsNull() && !plan.PowerState.Equal(state.PowerState) {
		desiredState := plan.PowerState.ValueString()

		switch desiredState {
		case "running":
			_, err := r.client.Baremetal.Start(ctx, id)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error starting Baremetal",
					"Could not start Baremetal: "+err.Error(),
				)
				return
			}
			_, err = r.client.Baremetal.WaitForBaremetalStatus(ctx, id, []string{"running", "active"}, 10*time.Minute)
			if err != nil {
				resp.Diagnostics.AddError(
					"Timeout waiting for Baremetal to start",
					"Baremetal did not reach running state: "+err.Error(),
				)
				return
			}
		case "stopped":
			_, err := r.client.Baremetal.Stop(ctx, id)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error stopping Baremetal",
					"Could not stop Baremetal: "+err.Error(),
				)
				return
			}
			_, err = r.client.Baremetal.WaitForBaremetalStatus(ctx, id, []string{"stopped"}, 10*time.Minute)
			if err != nil {
				resp.Diagnostics.AddError(
					"Timeout waiting for Baremetal to stop",
					"Baremetal did not reach stopped state: "+err.Error(),
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

	// Read updated baremetal
	baremetal, err := r.client.Baremetal.Get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading Baremetal after update",
			"Could not read Baremetal: "+err.Error(),
		)
		return
	}

	r.updateStateFromBaremetal(ctx, &plan, baremetal, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// BaremetalDestroyWarning is shown when a baremetal leaves the Terraform state. Destroying a
// dedicated server is never done from Terraform: a mistaken destroy must not cancel hardware.
const BaremetalDestroyWarning = "The server is still running and billed; cancel it from the CubePath dashboard."

// Delete only removes the baremetal from the Terraform state. The server itself is not touched.
func (r *baremetalResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state baremetalResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.AddWarning(
		"Baremetal "+state.ID.ValueString()+" removed from state only",
		BaremetalDestroyWarning,
	)
}

func (r *baremetalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *baremetalResource) updateStateFromBaremetal(ctx context.Context, state *baremetalResourceModel, bm *client.Baremetal, diags *diag.Diagnostics) {
	state.ID = types.StringValue(strconv.Itoa(bm.ID))
	state.Hostname = types.StringValue(bm.Hostname)
	state.Label = types.StringValue(bm.Label)
	state.Status = types.StringValue(bm.Status)
	state.CreatedAt = types.StringValue(bm.CreatedAt.String())

	// Project ID
	if bm.ProjectID != 0 {
		state.ProjectID = types.Int64Value(int64(bm.ProjectID))
	}

	// Location
	state.Location = types.StringValue(bm.Location.LocationName)

	// Model details
	state.ModelName = types.StringValue(bm.BaremetalModel.ModelName)
	state.CPU = types.StringValue(bm.BaremetalModel.CPU)
	state.RAM = types.Int64Value(int64(bm.BaremetalModel.RAM))
	state.StorageType = types.StringValue(bm.BaremetalModel.StorageType)
	state.DiskSize = types.StringValue(bm.BaremetalModel.DiskSize)

	// User
	if bm.User != "" {
		state.User = types.StringValue(bm.User)
	} else if state.User.IsNull() {
		state.User = types.StringValue("root")
	}

	// OS
	if bm.OS != nil {
		state.OSName = types.StringValue(bm.OS.Name)
	}

	// Monitoring
	state.MonitoringEnable = types.BoolValue(bm.MonitoringEnable)

	// Protection and private network
	state.Protected = types.BoolValue(bm.Protected)
	if bm.Network != nil && bm.Network.ID != 0 {
		state.NetworkID = types.Int64Value(int64(bm.Network.ID))
		state.PrivateIP = types.StringValue(bm.Network.AssignedIP)
	} else {
		// network_id = 0 in the configuration means "no private network".
		if state.NetworkID.IsNull() || state.NetworkID.IsUnknown() || state.NetworkID.ValueInt64() != 0 {
			state.NetworkID = types.Int64Null()
		}
		state.PrivateIP = types.StringNull()
	}

	// IPs
	for _, ip := range bm.FloatingIPs {
		if ip.Type == "IPv4" {
			state.MainIP = types.StringValue(ip.Address)
		} else if ip.Type == "IPv6" {
			state.IPv6 = types.StringValue(ip.Address)
		}
	}

	// Power state based on status
	switch bm.Status {
	case "running", "active":
		state.PowerState = types.StringValue("running")
	case "stopped":
		state.PowerState = types.StringValue("stopped")
	default:
		if state.PowerState.IsNull() {
			state.PowerState = types.StringValue("running")
		}
	}
}

// applyInPlaceChanges applies project, protection, SSH key and private network changes.
func (r *baremetalResource) applyInPlaceChanges(ctx context.Context, id int, plan, prior *baremetalResourceModel, diags *diag.Diagnostics) {
	if !plan.ProjectID.Equal(prior.ProjectID) {
		if err := r.client.Baremetal.MoveProject(ctx, id, int(plan.ProjectID.ValueInt64())); err != nil {
			diags.AddError("Error moving Baremetal", err.Error())
			return
		}
	}

	if want := knownBool(plan.Protected); want != nil && !plan.Protected.Equal(prior.Protected) {
		if err := r.client.Baremetal.SetProtection(ctx, id, *want); err != nil {
			diags.AddError("Error changing Baremetal protection", err.Error())
			return
		}
	}

	if !plan.SSHKeyIDs.IsUnknown() && !plan.SSHKeyIDs.Equal(prior.SSHKeyIDs) {
		var have, want []int64
		if !prior.SSHKeyIDs.IsNull() && !prior.SSHKeyIDs.IsUnknown() {
			diags.Append(prior.SSHKeyIDs.ElementsAs(ctx, &have, false)...)
		}
		if !plan.SSHKeyIDs.IsNull() {
			diags.Append(plan.SSHKeyIDs.ElementsAs(ctx, &want, false)...)
		}
		if diags.HasError() {
			return
		}
		add, remove := diffInts(have, want)
		for _, key := range remove {
			if err := r.client.Baremetal.RemoveSSHKey(ctx, id, int(key)); err != nil && !isNotFound(err) {
				diags.AddError("Error removing SSH key from Baremetal", err.Error())
				return
			}
		}
		if len(add) > 0 {
			keys := make([]int, 0, len(add))
			for _, key := range add {
				keys = append(keys, int(key))
			}
			if err := r.client.Baremetal.AddSSHKeys(ctx, id, keys); err != nil {
				diags.AddError("Error adding SSH keys to Baremetal", err.Error())
				return
			}
		}
	}

	if !plan.NetworkID.IsUnknown() && !plan.NetworkID.Equal(prior.NetworkID) {
		if !prior.NetworkID.IsNull() && prior.NetworkID.ValueInt64() != 0 {
			if err := r.client.Baremetal.DetachNetwork(ctx, id); err != nil {
				diags.AddError("Error detaching private network", err.Error())
				return
			}
		}
		if networkID := knownInt(plan.NetworkID); networkID != nil && *networkID != 0 {
			err := retryWhile(ctx, 5*time.Minute, 10*time.Second, isConflict, func() error {
				return r.client.Baremetal.AttachNetwork(ctx, id, *networkID)
			})
			if err != nil {
				diags.AddError("Error attaching private network", err.Error())
			}
		}
	}
}
