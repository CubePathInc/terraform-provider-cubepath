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
	_ datasource.DataSource              = &kubeconfigDataSource{}
	_ datasource.DataSourceWithConfigure = &kubeconfigDataSource{}
)

func NewKubeconfigDataSource() datasource.DataSource {
	return &kubeconfigDataSource{}
}

type kubeconfigDataSource struct {
	client *client.Client
}

type kubeconfigDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	ClusterID  types.String `tfsdk:"cluster_id"`
	Kubeconfig types.String `tfsdk:"kubeconfig"`
}

func (d *kubeconfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_kubeconfig"
}

func (d *kubeconfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the kubeconfig for a Kubernetes cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"cluster_id": schema.StringAttribute{
				Description: "UUID of the Kubernetes cluster.",
				Required:    true,
			},
			"kubeconfig": schema.StringAttribute{
				Description: "The kubeconfig YAML content for connecting to the cluster.",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (d *kubeconfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *kubeconfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubeconfigDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	clusterID := config.ClusterID.ValueString()

	kubeconfig, err := d.client.Kubernetes.GetKubeconfig(ctx, clusterID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading kubeconfig", err.Error())
		return
	}

	config.ID = types.StringValue(clusterID)
	config.Kubeconfig = types.StringValue(kubeconfig)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
