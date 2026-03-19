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
	_ datasource.DataSource              = &vpsTemplatesDataSource{}
	_ datasource.DataSourceWithConfigure = &vpsTemplatesDataSource{}
)

func NewVPSTemplatesDataSource() datasource.DataSource {
	return &vpsTemplatesDataSource{}
}

type vpsTemplatesDataSource struct {
	client *client.Client
}

type vpsTemplatesDataSourceModel struct {
	ID        types.String       `tfsdk:"id"`
	Templates []vpsTemplateModel `tfsdk:"templates"`
}

type vpsTemplateModel struct {
	TemplateName types.String `tfsdk:"template_name"`
	OSName       types.String `tfsdk:"os_name"`
	Version      types.String `tfsdk:"version"`
}

func (d *vpsTemplatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vps_templates"
}

func (d *vpsTemplatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches the list of available VPS operating system templates.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"templates": schema.ListNestedAttribute{
				Description: "List of available VPS templates.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"template_name": schema.StringAttribute{
							Description: "The template identifier (e.g., 'debian-12').",
							Computed:    true,
						},
						"os_name": schema.StringAttribute{
							Description: "The operating system name.",
							Computed:    true,
						},
						"version": schema.StringAttribute{
							Description: "The OS version.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

func (d *vpsTemplatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *vpsTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state vpsTemplatesDataSourceModel

	pricing, err := d.client.Pricing.Get(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading VPS templates",
			"Could not read VPS templates: "+err.Error(),
		)
		return
	}

	// Extract templates
	state.ID = types.StringValue("vps_templates")
	state.Templates = make([]vpsTemplateModel, 0)

	for _, template := range pricing.VPS.Templates {
		state.Templates = append(state.Templates, vpsTemplateModel{
			TemplateName: types.StringValue(template.TemplateName),
			OSName:       types.StringValue(template.OSName),
			Version:      types.StringValue(template.Version),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
