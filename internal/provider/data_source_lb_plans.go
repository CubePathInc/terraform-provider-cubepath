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
	_ datasource.DataSource              = &lbPlansDataSource{}
	_ datasource.DataSourceWithConfigure = &lbPlansDataSource{}
)

func NewLBPlansDataSource() datasource.DataSource {
	return &lbPlansDataSource{}
}

type lbPlansDataSource struct {
	client *client.Client
}

type lbPlansDataSourceModel struct {
	ID        types.String          `tfsdk:"id"`
	Locations []lbLocationPlanModel `tfsdk:"locations"`
}

type lbLocationPlanModel struct {
	LocationName types.String  `tfsdk:"location_name"`
	Description  types.String  `tfsdk:"description"`
	Plans        []lbPlanModel `tfsdk:"plans"`
}

type lbPlanModel struct {
	Name                 types.String `tfsdk:"name"`
	PricePerHour         types.String `tfsdk:"price_per_hour"`
	MaxListeners         types.Int64  `tfsdk:"max_listeners"`
	MaxTargets           types.Int64  `tfsdk:"max_targets"`
	ConnectionsPerSecond types.Int64  `tfsdk:"connections_per_second"`
}

func (d *lbPlansDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lb_plans"
}

func (d *lbPlansDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches available load balancer plans.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"locations": schema.ListNestedAttribute{
				Description: "Plans grouped by location.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"location_name": schema.StringAttribute{
							Description: "Location identifier.",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Location description.",
							Computed:    true,
						},
						"plans": schema.ListNestedAttribute{
							Description: "Available plans at this location.",
							Computed:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Description: "Plan name.",
										Computed:    true,
									},
									"price_per_hour": schema.StringAttribute{
										Description: "Price per hour.",
										Computed:    true,
									},
									"max_listeners": schema.Int64Attribute{
										Description: "Maximum number of listeners.",
										Computed:    true,
									},
									"max_targets": schema.Int64Attribute{
										Description: "Maximum number of targets.",
										Computed:    true,
									},
									"connections_per_second": schema.Int64Attribute{
										Description: "Maximum connections per second.",
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

func (d *lbPlansDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *lbPlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state lbPlansDataSourceModel

	locations, err := d.client.LoadBalancer.ListPlans(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading load balancer plans", err.Error())
		return
	}

	state.ID = types.StringValue("lb_plans")
	state.Locations = make([]lbLocationPlanModel, 0, len(locations))
	for _, loc := range locations {
		plans := make([]lbPlanModel, 0, len(loc.Plans))
		for _, p := range loc.Plans {
			plans = append(plans, lbPlanModel{
				Name:                 types.StringValue(p.Name),
				PricePerHour:         types.StringValue(p.PricePerHour.String()),
				MaxListeners:         types.Int64Value(int64(p.MaxListeners)),
				MaxTargets:           types.Int64Value(int64(p.MaxTargets)),
				ConnectionsPerSecond: types.Int64Value(int64(p.ConnectionsPerSecond)),
			})
		}
		state.Locations = append(state.Locations, lbLocationPlanModel{
			LocationName: types.StringValue(loc.LocationName),
			Description:  types.StringValue(loc.Description),
			Plans:        plans,
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
