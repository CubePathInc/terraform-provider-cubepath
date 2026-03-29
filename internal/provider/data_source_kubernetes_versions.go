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
	_ datasource.DataSource              = &kubernetesVersionsDataSource{}
	_ datasource.DataSourceWithConfigure = &kubernetesVersionsDataSource{}
)

func NewKubernetesVersionsDataSource() datasource.DataSource {
	return &kubernetesVersionsDataSource{}
}

type kubernetesVersionsDataSource struct {
	client *client.Client
}

type kubernetesVersionsDataSourceModel struct {
	ID       types.String              `tfsdk:"id"`
	Versions []kubernetesVersionModel  `tfsdk:"versions"`
}

type kubernetesVersionModel struct {
	Version   types.String `tfsdk:"version"`
	IsDefault types.Bool   `tfsdk:"is_default"`
	MinCPU    types.Int64  `tfsdk:"min_cpu"`
	MinRAMMB  types.Int64  `tfsdk:"min_ram_mb"`
}

func (d *kubernetesVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_versions"
}

func (d *kubernetesVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches available Kubernetes versions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"versions": schema.ListNestedAttribute{
				Description: "Available Kubernetes versions.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"version": schema.StringAttribute{
							Description: "Kubernetes version string.",
							Computed:    true,
						},
						"is_default": schema.BoolAttribute{
							Description: "Whether this is the default version.",
							Computed:    true,
						},
						"min_cpu": schema.Int64Attribute{
							Description: "Minimum CPU cores required.",
							Computed:    true,
						},
						"min_ram_mb": schema.Int64Attribute{
							Description: "Minimum RAM in MB required.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *kubernetesVersionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *kubernetesVersionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state kubernetesVersionsDataSourceModel

	versions, err := d.client.Kubernetes.ListVersions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kubernetes versions", err.Error())
		return
	}

	state.ID = types.StringValue("kubernetes_versions")
	state.Versions = make([]kubernetesVersionModel, 0, len(versions))
	for _, v := range versions {
		state.Versions = append(state.Versions, kubernetesVersionModel{
			Version:   types.StringValue(v.Version),
			IsDefault: types.BoolValue(v.IsDefault),
			MinCPU:    types.Int64Value(int64(v.MinCPU)),
			MinRAMMB:  types.Int64Value(int64(v.MinRAMMB)),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
