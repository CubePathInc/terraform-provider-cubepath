package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &firewallGroupsDataSource{}
	_ datasource.DataSourceWithConfigure = &firewallGroupsDataSource{}
)

func NewFirewallGroupsDataSource() datasource.DataSource {
	return &firewallGroupsDataSource{}
}

type firewallGroupsDataSource struct {
	client *client.Client
}

type firewallGroupsDataSourceModel struct {
	Groups types.List `tfsdk:"groups"`
}

type firewallGroupDataModel struct {
	ID        types.Int64  `tfsdk:"id"`
	ProjectID types.Int64  `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	VPSCount  types.Int64  `tfsdk:"vps_count"`
	Rules     types.List   `tfsdk:"rules"`
}

func (d *firewallGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_groups"
}

func (d *firewallGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches all firewall groups for the user's organization",
		Attributes: map[string]schema.Attribute{
			"groups": schema.ListNestedAttribute{
				Description: "List of firewall groups",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Description: "Firewall group ID",
							Computed:    true,
						},
						"project_id": schema.Int64Attribute{
							Description: "Project ID this firewall group belongs to",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Firewall group name",
							Computed:    true,
						},
						"enabled": schema.BoolAttribute{
							Description: "Whether the firewall group is enabled",
							Computed:    true,
						},
						"vps_count": schema.Int64Attribute{
							Description: "Number of VPS instances using this firewall group",
							Computed:    true,
						},
						"rules": schema.ListNestedAttribute{
							Description: "Firewall rules in this group",
							Computed:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"direction": schema.StringAttribute{
										Description: "Direction of traffic",
										Computed:    true,
									},
									"protocol": schema.StringAttribute{
										Description: "Protocol",
										Computed:    true,
									},
									"port": schema.StringAttribute{
										Description: "Port specification",
										Computed:    true,
									},
									"source": schema.StringAttribute{
										Description: "Source IP or CIDR",
										Computed:    true,
									},
									"comment": schema.StringAttribute{
										Description: "Rule comment",
										Computed:    true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *firewallGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *firewallGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state firewallGroupsDataSourceModel

	// Get firewall groups from API
	groups, err := d.client.Firewall.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Firewall Groups",
			err.Error(),
		)
		return
	}

	// Convert to Terraform types
	groupElements := []attr.Value{}
	for _, group := range groups {
		// Convert rules
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

		// Build group object
		groupAttrs := map[string]attr.Value{
			"id":         types.Int64Value(int64(group.ID)),
			"project_id": types.Int64Value(int64(group.ProjectID)),
			"name":       types.StringValue(group.Name),
			"enabled":    types.BoolValue(group.Enabled),
			"vps_count":  types.Int64Value(int64(group.VPSCount)),
			"rules":      rulesList,
		}

		groupObj, diags := types.ObjectValue(
			map[string]attr.Type{
				"id":         types.Int64Type,
				"project_id": types.Int64Type,
				"name":       types.StringType,
				"enabled":    types.BoolType,
				"vps_count":  types.Int64Type,
				"rules": types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"direction": types.StringType,
							"protocol":  types.StringType,
							"port":      types.StringType,
							"source":    types.StringType,
							"comment":   types.StringType,
						},
					},
				},
			},
			groupAttrs,
		)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		groupElements = append(groupElements, groupObj)
	}

	groupsList, diags := types.ListValue(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"id":         types.Int64Type,
				"project_id": types.Int64Type,
				"name":       types.StringType,
				"enabled":    types.BoolType,
				"vps_count":  types.Int64Type,
				"rules": types.ListType{
					ElemType: types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"direction": types.StringType,
							"protocol":  types.StringType,
							"port":      types.StringType,
							"source":    types.StringType,
							"comment":   types.StringType,
						},
					},
				},
			},
		},
		groupElements,
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Groups = groupsList

	// Set state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
