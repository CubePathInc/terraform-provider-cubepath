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
	_ datasource.DataSource              = &cdnPlansDataSource{}
	_ datasource.DataSourceWithConfigure = &cdnPlansDataSource{}
)

func NewCDNPlansDataSource() datasource.DataSource {
	return &cdnPlansDataSource{}
}

type cdnPlansDataSource struct {
	client *client.Client
}

type cdnPlansDataSourceModel struct {
	ID    types.String   `tfsdk:"id"`
	Plans []cdnPlanModel `tfsdk:"plans"`
}

type cdnPlanModel struct {
	Name              types.String  `tfsdk:"name"`
	Description       types.String  `tfsdk:"description"`
	BasePricePerHour  types.Float64 `tfsdk:"base_price_per_hour"`
	MaxZones          types.Int64   `tfsdk:"max_zones"`
	MaxOriginsPerZone types.Int64   `tfsdk:"max_origins_per_zone"`
	MaxRulesPerZone   types.Int64   `tfsdk:"max_rules_per_zone"`
	CustomSSLAllowed  types.Bool    `tfsdk:"custom_ssl_allowed"`
}

func (d *cdnPlansDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cdn_plans"
}

func (d *cdnPlansDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches available CDN plans.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"plans": schema.ListNestedAttribute{
				Description: "Available CDN plans.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "Plan name.",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Plan description.",
							Computed:    true,
						},
						"base_price_per_hour": schema.Float64Attribute{
							Description: "Base price per hour.",
							Computed:    true,
						},
						"max_zones": schema.Int64Attribute{
							Description: "Maximum number of CDN zones.",
							Computed:    true,
						},
						"max_origins_per_zone": schema.Int64Attribute{
							Description: "Maximum origins per zone.",
							Computed:    true,
						},
						"max_rules_per_zone": schema.Int64Attribute{
							Description: "Maximum rules per zone.",
							Computed:    true,
						},
						"custom_ssl_allowed": schema.BoolAttribute{
							Description: "Whether custom SSL certificates are allowed.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *cdnPlansDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *cdnPlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state cdnPlansDataSourceModel

	plans, err := d.client.CDN.ListPlans(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading CDN plans", err.Error())
		return
	}

	state.ID = types.StringValue("cdn_plans")
	state.Plans = make([]cdnPlanModel, 0, len(plans))
	for _, p := range plans {
		state.Plans = append(state.Plans, cdnPlanModel{
			Name:              types.StringValue(p.Name),
			Description:       types.StringValue(p.Description),
			BasePricePerHour:  types.Float64Value(p.BasePricePerHour),
			MaxZones:          types.Int64Value(int64(p.MaxZones)),
			MaxOriginsPerZone: types.Int64Value(int64(p.MaxOriginsPerZone)),
			MaxRulesPerZone:   types.Int64Value(int64(p.MaxRulesPerZone)),
			CustomSSLAllowed:  types.BoolValue(p.CustomSSLAllowed),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
