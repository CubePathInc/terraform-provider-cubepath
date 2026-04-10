package provider

import (
	"context"
	"os"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &cubePathProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &cubePathProvider{
			version: version,
		}
	}
}

// cubePathProvider is the provider implementation.
type cubePathProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// cubePathProviderModel maps provider schema data to a Go type.
type cubePathProviderModel struct {
	APIToken  types.String `tfsdk:"api_token"`
	BaseURL   types.String `tfsdk:"base_url"`

	// Timeouts
	DefaultTimeout          types.Int64 `tfsdk:"default_timeout"`
	VPSCreateTimeout        types.Int64 `tfsdk:"vps_create_timeout"`
	VPSDestroyTimeout       types.Int64 `tfsdk:"vps_destroy_timeout"`
	BaremetalDeployTimeout  types.Int64 `tfsdk:"baremetal_deploy_timeout"`

	// Retry configuration
	MaxRetries   types.Int64 `tfsdk:"max_retries"`
	RetryWaitMin types.Int64 `tfsdk:"retry_wait_min"`
	RetryWaitMax types.Int64 `tfsdk:"retry_wait_max"`
}

// Metadata returns the provider type name.
func (p *cubePathProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "cubepath"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *cubePathProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with CubePath Cloud API to manage infrastructure resources.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Description: "API token for CubePath Cloud. Can also be set via CUBE_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Base URL for the CubePath API. Defaults to https://api.cubepath.com. Can also be set via CUBE_API_URL environment variable.",
				Optional:    true,
			},
			"default_timeout": schema.Int64Attribute{
				Description: "Default timeout in seconds for API operations. Defaults to 300 (5 minutes).",
				Optional:    true,
			},
			"vps_create_timeout": schema.Int64Attribute{
				Description: "Timeout in seconds for VPS creation operations. Defaults to 600 (10 minutes).",
				Optional:    true,
			},
			"vps_destroy_timeout": schema.Int64Attribute{
				Description: "Timeout in seconds for VPS destruction operations. Defaults to 300 (5 minutes).",
				Optional:    true,
			},
			"baremetal_deploy_timeout": schema.Int64Attribute{
				Description: "Timeout in seconds for baremetal deployment operations. Defaults to 1800 (30 minutes).",
				Optional:    true,
			},
			"max_retries": schema.Int64Attribute{
				Description: "Maximum number of retries for failed API requests. Defaults to 3.",
				Optional:    true,
			},
			"retry_wait_min": schema.Int64Attribute{
				Description: "Minimum wait time in seconds between retries. Defaults to 1.",
				Optional:    true,
			},
			"retry_wait_max": schema.Int64Attribute{
				Description: "Maximum wait time in seconds between retries. Defaults to 30.",
				Optional:    true,
			},
		},
	}
}

// Configure prepares a CubePath API client for data sources and resources.
func (p *cubePathProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config cubePathProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	// Get API token from config or environment
	apiToken := os.Getenv("CUBE_API_TOKEN")
	if !config.APIToken.IsNull() {
		apiToken = config.APIToken.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddError(
			"Missing API Token",
			"The provider cannot create the CubePath API client as there is a missing or empty value for the API token. "+
				"Set the api_token value in the configuration or use the CUBE_API_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
		return
	}

	// Get base URL from config or environment (always has default)
	baseURL := os.Getenv("CUBE_API_URL")
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}

	if baseURL == "" {
		baseURL = "https://api.cubepath.com"
	}

	// Create a new CubePath client using the configuration values
	apiClient, err := client.NewClient(apiToken, baseURL)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create CubePath API Client",
			"An unexpected error occurred when creating the CubePath API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"CubePath Client Error: "+err.Error(),
		)
		return
	}

	// Make the CubePath client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

// DataSources defines the data sources implemented in the provider.
func (p *cubePathProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewLocationsDataSource,
		NewVPSPlansDataSource,
		NewVPSTemplatesDataSource,
		NewSSHKeyDataSource,
		NewFirewallGroupsDataSource,
		NewDNSZonesDataSource,
		NewLBPlansDataSource,
		NewCDNPlansDataSource,
		NewKubernetesVersionsDataSource,
		NewKubernetesPlansDataSource,
		NewKubernetesAddonsDataSource,
		NewKubeconfigDataSource,
		NewAvailabilityGroupsDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *cubePathProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSSHKeyResource,
		NewProjectResource,
		NewNetworkResource,
		NewVPSResource,
		NewBaremetalResource,
		NewFirewallGroupResource,
		NewFloatingIPResource,
		NewDNSZoneResource,
		NewDNSRecordResource,
		NewLoadBalancerResource,
		NewLBListenerResource,
		NewCDNZoneResource,
		NewCDNOriginResource,
		NewCDNRuleResource,
		NewCDNWAFRuleResource,
		NewKubernetesClusterResource,
		NewKubernetesNodePoolResource,
		NewKubernetesAddonResource,
		NewAvailabilityGroupResource,
	}
}
