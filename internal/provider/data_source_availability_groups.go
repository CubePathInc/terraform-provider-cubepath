package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &availabilityGroupsDataSource{}
	_ datasource.DataSourceWithConfigure = &availabilityGroupsDataSource{}
)

func NewAvailabilityGroupsDataSource() datasource.DataSource {
	return &availabilityGroupsDataSource{}
}

type availabilityGroupsDataSource struct {
	client *client.Client
}

type availabilityGroupsDataSourceModel struct {
	ID        types.String                    `tfsdk:"id"`
	ProjectID types.Int64                     `tfsdk:"project_id"`
	Groups    []availabilityGroupItemModel    `tfsdk:"groups"`
}

type availabilityGroupItemModel struct {
	UUID         types.String `tfsdk:"uuid"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Strategy     types.String `tfsdk:"strategy"`
	LocationName types.String `tfsdk:"location_name"`
	MaxServers   types.Int64  `tfsdk:"max_servers"`
	VPSCount     types.Int64  `tfsdk:"vps_count"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (d *availabilityGroupsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_availability_groups"
}

func (d *availabilityGroupsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the list of availability groups for a CubePath Cloud project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"project_id": schema.Int64Attribute{
				Description: "The ID of the project to list availability groups for.",
				Required:    true,
			},
			"groups": schema.ListNestedAttribute{
				Description: "List of availability groups.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Description: "The UUID of the availability group.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "The name of the availability group.",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "The description of the availability group.",
							Computed:    true,
						},
						"strategy": schema.StringAttribute{
							Description: "The placement strategy (spread or cluster).",
							Computed:    true,
						},
						"location_name": schema.StringAttribute{
							Description: "The location where the availability group operates.",
							Computed:    true,
						},
						"max_servers": schema.Int64Attribute{
							Description: "Maximum number of VPS in this group.",
							Computed:    true,
						},
						"vps_count": schema.Int64Attribute{
							Description: "Current number of VPS in this group.",
							Computed:    true,
						},
						"created_at": schema.StringAttribute{
							Description: "The timestamp when the availability group was created.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *availabilityGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *availabilityGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state availabilityGroupsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groups, err := d.client.AvailabilityGroups.List(ctx, int(state.ProjectID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading availability groups",
			"Could not read availability groups: "+err.Error(),
		)
		return
	}

	state.ID = types.StringValue(fmt.Sprintf("availability_groups_%d", state.ProjectID.ValueInt64()))
	state.Groups = make([]availabilityGroupItemModel, 0, len(groups))

	for _, group := range groups {
		state.Groups = append(state.Groups, availabilityGroupItemModel{
			UUID:         types.StringValue(group.UUID),
			Name:         types.StringValue(group.Name),
			Description:  types.StringValue(group.Description),
			Strategy:     types.StringValue(group.Strategy),
			LocationName: types.StringValue(group.LocationName),
			MaxServers:   types.Int64Value(int64(group.MaxServers)),
			VPSCount:     types.Int64Value(int64(group.VPSCount)),
			CreatedAt:    types.StringValue(group.CreatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
