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
	_ datasource.DataSource              = &natGatewayPlansDataSource{}
	_ datasource.DataSourceWithConfigure = &natGatewayPlansDataSource{}
)

func NewNATGatewayPlansDataSource() datasource.DataSource {
	return &natGatewayPlansDataSource{}
}

type natGatewayPlansDataSource struct {
	client *client.Client
}

type natGatewayPlansDataSourceModel struct {
	ID    types.String          `tfsdk:"id"`
	Plans []natGatewayPlanModel `tfsdk:"plans"`
}

type natGatewayPlanModel struct {
	Name         types.String `tfsdk:"name"`
	PricePerHour types.String `tfsdk:"price_per_hour"`
}

func (d *natGatewayPlansDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nat_gateway_plans"
}

func (d *natGatewayPlansDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches available NAT gateway plans.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"plans": schema.ListNestedAttribute{
				Description: "Available NAT gateway plans.",
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
					},
				},
			},
		},
	}
}

func (d *natGatewayPlansDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = c
}

func (d *natGatewayPlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state natGatewayPlansDataSourceModel

	plans, err := d.client.NATGateway.ListPlans(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading NAT gateway plans", err.Error())
		return
	}

	state.ID = types.StringValue("nat_gateway_plans")
	state.Plans = make([]natGatewayPlanModel, 0, len(plans))
	for _, p := range plans {
		state.Plans = append(state.Plans, natGatewayPlanModel{
			Name:         types.StringValue(p.Name),
			PricePerHour: types.StringValue(p.PricePerHour),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
