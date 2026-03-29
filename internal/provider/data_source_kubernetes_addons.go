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
	_ datasource.DataSource              = &kubernetesAddonsDataSource{}
	_ datasource.DataSourceWithConfigure = &kubernetesAddonsDataSource{}
)

func NewKubernetesAddonsDataSource() datasource.DataSource {
	return &kubernetesAddonsDataSource{}
}

type kubernetesAddonsDataSource struct {
	client *client.Client
}

type kubernetesAddonsDataSourceModel struct {
	ID     types.String            `tfsdk:"id"`
	Addons []kubernetesAddonModel  `tfsdk:"addons"`
}

type kubernetesAddonModel struct {
	Name             types.String `tfsdk:"name"`
	Slug             types.String `tfsdk:"slug"`
	Description      types.String `tfsdk:"description"`
	Category         types.String `tfsdk:"category"`
	DefaultVersion   types.String `tfsdk:"default_version"`
	Namespace        types.String `tfsdk:"namespace"`
	HelmChart        types.String `tfsdk:"helm_chart"`
	MinK8sVersion    types.String `tfsdk:"min_k8s_version"`
	DocumentationURL types.String `tfsdk:"documentation_url"`
}

func (d *kubernetesAddonsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_addons"
}

func (d *kubernetesAddonsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches available Kubernetes addons.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"addons": schema.ListNestedAttribute{
				Description: "Available Kubernetes addons.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "Addon display name.",
							Computed:    true,
						},
						"slug": schema.StringAttribute{
							Description: "Addon slug identifier.",
							Computed:    true,
						},
						"description": schema.StringAttribute{
							Description: "Addon description.",
							Computed:    true,
						},
						"category": schema.StringAttribute{
							Description: "Addon category.",
							Computed:    true,
						},
						"default_version": schema.StringAttribute{
							Description: "Default Helm chart version.",
							Computed:    true,
						},
						"namespace": schema.StringAttribute{
							Description: "Default namespace for installation.",
							Computed:    true,
						},
						"helm_chart": schema.StringAttribute{
							Description: "Helm chart name.",
							Computed:    true,
						},
						"min_k8s_version": schema.StringAttribute{
							Description: "Minimum Kubernetes version required.",
							Computed:    true,
						},
						"documentation_url": schema.StringAttribute{
							Description: "Link to addon documentation.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *kubernetesAddonsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *kubernetesAddonsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state kubernetesAddonsDataSourceModel

	addons, err := d.client.Kubernetes.ListAddons(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading Kubernetes addons", err.Error())
		return
	}

	state.ID = types.StringValue("kubernetes_addons")
	state.Addons = make([]kubernetesAddonModel, 0, len(addons))
	for _, a := range addons {
		state.Addons = append(state.Addons, kubernetesAddonModel{
			Name:             types.StringValue(a.Name),
			Slug:             types.StringValue(a.Slug),
			Description:      types.StringValue(a.Description),
			Category:         types.StringValue(a.Category),
			DefaultVersion:   types.StringValue(a.DefaultVersion),
			Namespace:        types.StringValue(a.Namespace),
			HelmChart:        types.StringValue(a.HelmChart),
			MinK8sVersion:    types.StringValue(a.MinK8sVersion),
			DocumentationURL: types.StringValue(a.DocumentationURL),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
