package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &firewallGroupResource{}
	_ resource.ResourceWithConfigure   = &firewallGroupResource{}
	_ resource.ResourceWithImportState = &firewallGroupResource{}
)

// NewFirewallGroupResource is a helper function to simplify the provider implementation.
func NewFirewallGroupResource() resource.Resource {
	return &firewallGroupResource{}
}

// firewallGroupResource is the resource implementation.
type firewallGroupResource struct {
	client *client.Client
}

// firewallGroupResourceModel maps the resource schema data.
type firewallGroupResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.Int64  `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	Rules     types.List   `tfsdk:"rule"` // List of firewallRuleModel
	VPSCount  types.Int64  `tfsdk:"vps_count"`
}

// firewallRuleModel maps a single firewall rule
type firewallRuleModel struct {
	Direction types.String `tfsdk:"direction"`
	Protocol  types.String `tfsdk:"protocol"`
	Port      types.String `tfsdk:"port"`
	Source    types.String `tfsdk:"source"`
	Comment   types.String `tfsdk:"comment"`
}

// Metadata returns the resource type name.
func (r *firewallGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_group"
}

// Schema defines the schema for the resource.
func (r *firewallGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a firewall group with reusable firewall rules for VPS instances.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "The unique identifier of the firewall group.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.Int64Attribute{
				Description: "The ID of the project this firewall group belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the firewall group.",
				Required:    true,
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the firewall group is enabled.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"vps_count": schema.Int64Attribute{
				Description: "Number of VPS instances using this firewall group.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				Description: "Firewall rules in this group.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"direction": schema.StringAttribute{
							Description: "Direction of traffic: 'in' for incoming, 'out' for outgoing.",
							Required:    true,
						},
						"protocol": schema.StringAttribute{
							Description: "Protocol: 'tcp', 'udp', 'icmp', or 'gre'.",
							Required:    true,
						},
						"port": schema.StringAttribute{
							Description: "Port specification (e.g., '80', '80-443', '80,443,8080'). Optional for icmp/gre.",
							Optional:    true,
						},
						"source": schema.StringAttribute{
							Description: "Source IP or CIDR (e.g., '0.0.0.0/0', '10.0.0.0/24'). Defaults to any if not specified.",
							Optional:    true,
						},
						"comment": schema.StringAttribute{
							Description: "A comment describing this rule.",
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the resource.
func (r *firewallGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *firewallGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert rules from Terraform plan to API format
	var rules []client.FirewallRule
	for _, ruleElem := range plan.Rules.Elements() {
		ruleObj, ok := ruleElem.(types.Object)
		if !ok {
			resp.Diagnostics.AddError("Invalid Rule", "Could not parse firewall rule")
			return
		}

		ruleAttrs := ruleObj.Attributes()
		rule := client.FirewallRule{
			Direction: ruleAttrs["direction"].(types.String).ValueString(),
			Protocol:  ruleAttrs["protocol"].(types.String).ValueString(),
		}

		if port, ok := ruleAttrs["port"].(types.String); ok && !port.IsNull() {
			portStr := port.ValueString()
			rule.Port = &portStr
		}
		if source, ok := ruleAttrs["source"].(types.String); ok && !source.IsNull() {
			sourceStr := source.ValueString()
			rule.Source = &sourceStr
		}
		if comment, ok := ruleAttrs["comment"].(types.String); ok && !comment.IsNull() {
			commentStr := comment.ValueString()
			rule.Comment = &commentStr
		}

		rules = append(rules, rule)
	}

	// Create firewall group via API
	createReq := &client.CreateFirewallGroupRequest{
		Name:    plan.Name.ValueString(),
		Rules:   rules,
		Enabled: plan.Enabled.ValueBool(),
	}

	group, err := r.client.Firewall.Create(ctx, int(plan.ProjectID.ValueInt64()), createReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating firewall group",
			"Could not create firewall group: "+err.Error(),
		)
		return
	}

	// Map response to state
	plan.ID = types.StringValue(fmt.Sprintf("%d", group.ID))
	plan.VPSCount = types.Int64Value(int64(group.VPSCount))

	// Save state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the Terraform state with the latest data.
func (r *firewallGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse ID
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing firewall group ID",
			"Could not parse firewall group ID: "+err.Error(),
		)
		return
	}

	// Get firewall group from API
	group, err := r.client.Firewall.Get(ctx, id)
	if err != nil {
		if apiErr, ok := err.(*client.APIError); ok && apiErr.IsNotFound() {
			// Resource no longer exists
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Error reading firewall group",
			"Could not read firewall group: "+err.Error(),
		)
		return
	}

	// Update state from API response
	state.ID = types.StringValue(fmt.Sprintf("%d", group.ID))
	state.ProjectID = types.Int64Value(int64(group.ProjectID))
	state.Name = types.StringValue(group.Name)
	state.Enabled = types.BoolValue(group.Enabled)
	state.VPSCount = types.Int64Value(int64(group.VPSCount))

	// Convert rules to Terraform types
	ruleElements := []attr.Value{}
	for _, rule := range group.Rules {
		ruleAttrs := map[string]attr.Value{
			"direction": types.StringValue(rule.Direction),
			"protocol":  types.StringValue(rule.Protocol),
			"port":      types.StringNull(),
			"source":    types.StringNull(),
			"comment":   types.StringNull(),
		}
		if rule.Port != nil {
			ruleAttrs["port"] = types.StringValue(*rule.Port)
		}
		if rule.Source != nil {
			ruleAttrs["source"] = types.StringValue(*rule.Source)
		}
		if rule.Comment != nil {
			ruleAttrs["comment"] = types.StringValue(*rule.Comment)
		}

		ruleObj, diags := types.ObjectValue(
			map[string]attr.Type{
				"direction": types.StringType,
				"protocol":  types.StringType,
				"port":      types.StringType,
				"source":    types.StringType,
				"comment":   types.StringType,
			},
			ruleAttrs,
		)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		ruleElements = append(ruleElements, ruleObj)
	}

	rulesList, diags := types.ListValue(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"direction": types.StringType,
				"protocol":  types.StringType,
				"port":      types.StringType,
				"source":    types.StringType,
				"comment":   types.StringType,
			},
		},
		ruleElements,
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Rules = rulesList

	// Save state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *firewallGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan firewallGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state firewallGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse ID
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing firewall group ID",
			"Could not parse firewall group ID: "+err.Error(),
		)
		return
	}

	// Build update request
	updateReq := &client.UpdateFirewallGroupRequest{}
	changed := false

	// Check if name changed
	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
		changed = true
	}

	// Check if enabled changed
	if !plan.Enabled.Equal(state.Enabled) {
		enabled := plan.Enabled.ValueBool()
		updateReq.Enabled = &enabled
		changed = true
	}

	// Check if rules changed
	if !plan.Rules.Equal(state.Rules) {
		var rules []client.FirewallRule
		for _, ruleElem := range plan.Rules.Elements() {
			ruleObj, ok := ruleElem.(types.Object)
			if !ok {
				resp.Diagnostics.AddError("Invalid Rule", "Could not parse firewall rule")
				return
			}

			ruleAttrs := ruleObj.Attributes()
			rule := client.FirewallRule{
				Direction: ruleAttrs["direction"].(types.String).ValueString(),
				Protocol:  ruleAttrs["protocol"].(types.String).ValueString(),
			}

			if port, ok := ruleAttrs["port"].(types.String); ok && !port.IsNull() {
				portStr := port.ValueString()
				rule.Port = &portStr
			}
			if source, ok := ruleAttrs["source"].(types.String); ok && !source.IsNull() {
				sourceStr := source.ValueString()
				rule.Source = &sourceStr
			}
			if comment, ok := ruleAttrs["comment"].(types.String); ok && !comment.IsNull() {
				commentStr := comment.ValueString()
				rule.Comment = &commentStr
			}

			rules = append(rules, rule)
		}
		updateReq.Rules = &rules
		changed = true
	}

	// Only call API if something changed
	if changed {
		group, err := r.client.Firewall.Update(ctx, id, updateReq)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error updating firewall group",
				"Could not update firewall group: "+err.Error(),
			)
			return
		}

		// Update state with response
		plan.VPSCount = types.Int64Value(int64(group.VPSCount))
	}

	// Save updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *firewallGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse ID
	id, err := strconv.Atoi(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error parsing firewall group ID",
			"Could not parse firewall group ID: "+err.Error(),
		)
		return
	}

	// Delete via API
	err = r.client.Firewall.Delete(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting firewall group",
			"Could not delete firewall group: "+err.Error(),
		)
		return
	}
}

// ImportState imports the resource state.
func (r *firewallGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by ID
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
