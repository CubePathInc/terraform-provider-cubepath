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
	_ datasource.DataSource              = &kubernetesPlansDataSource{}
	_ datasource.DataSourceWithConfigure = &kubernetesPlansDataSource{}
)

func NewKubernetesPlansDataSource() datasource.DataSource {
	return &kubernetesPlansDataSource{}
}

type kubernetesPlansDataSource struct {
	client *client.Client
}

type kubernetesPlansDataSourceModel struct {
	ID      types.String           `tfsdk:"id"`
	Version types.String           `tfsdk:"version"`
	Plans   []kubernetesPlanModel  `tfsdk:"plans"`
}

type kubernetesPlanModel struct {
	ID           types.Int64   `tfsdk:"id"`
	Name         types.String  `tfsdk:"name"`
	CPU          types.Int64   `tfsdk:"cpu"`
	RAM          types.Int64   `tfsdk:"ram"`
	Storage      types.Int64   `tfsdk:"storage"`
	PricePerHour types.Float64 `tfsdk:"price_per_hour"`
}

func (d *kubernetesPlansDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_plans"
}

func (d *kubernetesPlansDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches server plans compatible with Kubernetes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"version": schema.StringAttribute{
				Description: "Filter plans by Kubernetes version.",
				Optional:    true,
			},
			"plans": schema.ListNestedAttribute{
				Description: "Available Kubernetes-compatible server plans.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Description: "Plan ID.",
							Computed:    true,
						},
						"name": schema.StringAttribute{
							Description: "Plan name.",
							Computed:    true,
						},
						"cpu": schema.Int64Attribute{
							Description: "Number of CPU cores.",
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
						"price_per_hour": schema.Float64Attribute{
							Description: "Price per hour.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *kubernetesPlansDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *kubernetesPlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubernetesPlansDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	version := ""
	if !config.Version.IsNull() && !config.Version.IsUnknown() {
		version = config.Version.ValueString()
	}

	plans, err := d.client.Kubernetes.ListPlans(ctx, version)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kubernetes plans", err.Error())
		return
	}

	config.ID = types.StringValue("kubernetes_plans")
	config.Plans = make([]kubernetesPlanModel, 0, len(plans))
	for _, p := range plans {
		config.Plans = append(config.Plans, kubernetesPlanModel{
			ID:           types.Int64Value(int64(p.ID)),
			Name:         types.StringValue(p.Name),
			CPU:          types.Int64Value(int64(p.CPU)),
			RAM:          types.Int64Value(int64(p.RAM)),
			Storage:      types.Int64Value(int64(p.Storage)),
			PricePerHour: types.Float64Value(p.PricePerHour),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
