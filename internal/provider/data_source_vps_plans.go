package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// parsePriceToFloat64 converts a price string to float64 for Terraform
func parsePriceToFloat64(priceStr string) types.Float64 {
	if priceStr == "" {
		return types.Float64Value(0.0)
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return types.Float64Value(0.0)
	}
	return types.Float64Value(price)
}

var (
	_ datasource.DataSource              = &vpsPlansDataSource{}
	_ datasource.DataSourceWithConfigure = &vpsPlansDataSource{}
)

func NewVPSPlansDataSource() datasource.DataSource {
	return &vpsPlansDataSource{}
}

type vpsPlansDataSource struct {
	client *client.Client
}

type vpsPlansDataSourceModel struct {
	ID       types.String   `tfsdk:"id"`
	Location types.String   `tfsdk:"location"`
	Plans    []vpsPlanModel `tfsdk:"plans"`
}

type vpsPlanModel struct {
	PlanName     types.String  `tfsdk:"plan_name"`
	Location     types.String  `tfsdk:"location"`
	Cluster      types.String  `tfsdk:"cluster"`
	CPU          types.Int64   `tfsdk:"cpu"`
	RAM          types.Int64   `tfsdk:"ram"`
	Storage      types.Int64   `tfsdk:"storage"`
	Bandwidth    types.Int64   `tfsdk:"bandwidth"`
	PricePerHour types.Float64 `tfsdk:"price_per_hour"`
}

func (d *vpsPlansDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vps_plans"
}

func (d *vpsPlansDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the list of available VPS plans.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"location": schema.StringAttribute{
				Description: "Optional filter by location name.",
				Optional:    true,
			},
			"plans": schema.ListNestedAttribute{
				Description: "List of available VPS plans.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"plan_name": schema.StringAttribute{
							Description: "The plan identifier (e.g., 'gp.pro').",
							Computed:    true,
						},
						"location": schema.StringAttribute{
							Description: "The location where this plan is available.",
							Computed:    true,
						},
						"cluster": schema.StringAttribute{
							Description: "The cluster where this plan is available.",
							Computed:    true,
						},
						"cpu": schema.Int64Attribute{
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
						"price_per_hour": schema.Float64Attribute{
							Description: "Price per hour in USD.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *vpsPlansDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vpsPlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vpsPlansDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pricing, err := d.client.Pricing.Get(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VPS plans",
			"Could not read VPS plans: "+err.Error(),
		)
		return
	}

	// Extract plans
	config.ID = types.StringValue("vps_plans")
	config.Plans = make([]vpsPlanModel, 0)

	locationFilter := ""
	if !config.Location.IsNull() {
		locationFilter = config.Location.ValueString()
	}

	for _, location := range pricing.VPS.Locations {
		// Apply location filter if specified
		if locationFilter != "" && location.LocationName != locationFilter {
			continue
		}

		for _, cluster := range location.Clusters {
			for _, plan := range cluster.Plans {
				config.Plans = append(config.Plans, vpsPlanModel{
					PlanName:     types.StringValue(plan.PlanName),
					Location:     types.StringValue(location.LocationName),
					Cluster:      types.StringValue(cluster.ClusterName),
					CPU:          types.Int64Value(int64(plan.CPU)),
					RAM:          types.Int64Value(int64(plan.RAM)),
					Storage:      types.Int64Value(int64(plan.Storage)),
					Bandwidth:    types.Int64Value(int64(plan.Bandwidth)),
					PricePerHour: parsePriceToFloat64(plan.PricePerHour),
				})
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
