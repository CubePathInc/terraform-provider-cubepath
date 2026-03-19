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
	_ datasource.DataSource              = &dnsZonesDataSource{}
	_ datasource.DataSourceWithConfigure = &dnsZonesDataSource{}
)

func NewDNSZonesDataSource() datasource.DataSource {
	return &dnsZonesDataSource{}
}

type dnsZonesDataSource struct {
	client *client.Client
}

type dnsZonesDataSourceModel struct {
	ID    types.String     `tfsdk:"id"`
	Zones []dnsZoneModel   `tfsdk:"zones"`
}

type dnsZoneModel struct {
	UUID         types.String `tfsdk:"uuid"`
	Domain       types.String `tfsdk:"domain"`
	Status       types.String `tfsdk:"status"`
	RecordsCount types.Int64  `tfsdk:"records_count"`
}

func (d *dnsZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_zones"
}

func (d *dnsZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the list of DNS zones.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"zones": schema.ListNestedAttribute{
				Description: "List of DNS zones.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid": schema.StringAttribute{
							Description: "The UUID of the DNS zone.",
							Computed:    true,
						},
						"domain": schema.StringAttribute{
							Description: "The domain name.",
							Computed:    true,
						},
						"status": schema.StringAttribute{
							Description: "The zone status.",
							Computed:    true,
						},
						"records_count": schema.Int64Attribute{
							Description: "Number of records in the zone.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *dnsZonesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *dnsZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state dnsZonesDataSourceModel

	zones, err := d.client.DNS.ListZones(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zones", err.Error())
		return
	}

	state.ID = types.StringValue("dns_zones")
	state.Zones = make([]dnsZoneModel, 0, len(zones))
	for _, z := range zones {
		state.Zones = append(state.Zones, dnsZoneModel{
			UUID:         types.StringValue(z.UUID),
			Domain:       types.StringValue(z.Domain),
			Status:       types.StringValue(z.Status),
			RecordsCount: types.Int64Value(int64(z.RecordsCount)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
