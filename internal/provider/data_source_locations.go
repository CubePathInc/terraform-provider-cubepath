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
	_ datasource.DataSource              = &locationsDataSource{}
	_ datasource.DataSourceWithConfigure = &locationsDataSource{}
)

func NewLocationsDataSource() datasource.DataSource {
	return &locationsDataSource{}
}

type locationsDataSource struct {
	client *client.Client
}

type locationsDataSourceModel struct {
	ID        types.String    `tfsdk:"id"`
	Locations []locationModel `tfsdk:"locations"`
}

type locationModel struct {
	LocationName types.String `tfsdk:"location_name"`
	Description  types.String `tfsdk:"description"`
}

func (d *locationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_locations"
}

func (d *locationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the list of available locations for CubePath Cloud resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"locations": schema.ListNestedAttribute{
				Description: "List of available locations.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"location_name": schema.StringAttribute{
							Description: "The location identifier (e.g., 'us-mia-1').",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Human-readable description of the location.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *locationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *locationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state locationsDataSourceModel

	pricing, err := d.client.Pricing.Get(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading locations",
			"Could not read locations: "+err.Error(),
		)
		return
	}

	// Extract locations from pricing response
	state.ID = types.StringValue("locations")
	state.Locations = make([]locationModel, 0)

	for _, loc := range pricing.VPS.Locations {
		state.Locations = append(state.Locations, locationModel{
			LocationName: types.StringValue(loc.LocationName),
			Description:  types.StringValue(loc.Description),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
