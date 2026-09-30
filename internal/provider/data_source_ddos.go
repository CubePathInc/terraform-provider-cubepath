package provider

import (
	"context"
	"strconv"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &ddosProtectedIPsDataSource{}
	_ datasource.DataSourceWithConfigure = &ddosProtectedIPsDataSource{}
	_ datasource.DataSource              = &ddosCountriesDataSource{}
	_ datasource.DataSourceWithConfigure = &ddosCountriesDataSource{}
	_ datasource.DataSource              = &ddosASNsDataSource{}
	_ datasource.DataSourceWithConfigure = &ddosASNsDataSource{}
)

// ---- cubepath_ddos_protected_ips ----

func NewDDoSProtectedIPsDataSource() datasource.DataSource {
	return &ddosProtectedIPsDataSource{}
}

type ddosProtectedIPsDataSource struct {
	client *client.Client
}

type ddosProtectedIPsModel struct {
	ID  types.String           `tfsdk:"id"`
	IPs []ddosProtectedIPModel `tfsdk:"ips"`
}

type ddosProtectedIPModel struct {
	IPAddress          types.String `tfsdk:"ip_address"`
	IPType             types.String `tfsdk:"ip_type"`
	ProtectionType     types.String `tfsdk:"protection_type"`
	LocationName       types.String `tfsdk:"location_name"`
	Subnet             types.String `tfsdk:"subnet"`
	HasProfile         types.Bool   `tfsdk:"has_profile"`
	FirewallRulesCount types.Int64  `tfsdk:"firewall_rules_count"`
}

func (d *ddosProtectedIPsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddos_protected_ips"
}

func (d *ddosProtectedIPsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the organization's IP addresses with Premium DDoS protection, the ones a " +
			"cubepath_ddos_protection_profile can be set on. IPv4 subnets are expanded to their addresses.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Placeholder identifier.", Computed: true},
			"ips": schema.ListNestedAttribute{
				Description: "Protected IP addresses.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip_address":           schema.StringAttribute{Description: "IP address.", Computed: true},
						"ip_type":              schema.StringAttribute{Description: "IPv4 or IPv6.", Computed: true},
						"protection_type":      schema.StringAttribute{Description: "Premium or Premium Always-On.", Computed: true},
						"location_name":        schema.StringAttribute{Description: "Location of the IP.", Computed: true},
						"subnet":               schema.StringAttribute{Description: "Subnet the address belongs to, in CIDR notation, when it comes from a protected subnet.", Computed: true},
						"has_profile":          schema.BoolAttribute{Description: "Whether a protection profile is set.", Computed: true},
						"firewall_rules_count": schema.Int64Attribute{Description: "Number of DDoS firewall rules on the address.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *ddosProtectedIPsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *ddosProtectedIPsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	ips, err := d.client.DDoS.ListProtectedIPs(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DDoS protected IPs", err.Error())
		return
	}

	state := ddosProtectedIPsModel{ID: types.StringValue("ddos_protected_ips"), IPs: []ddosProtectedIPModel{}}
	for _, ip := range ips.SingleIPs {
		state.IPs = append(state.IPs, ddosProtectedIPModel{
			IPAddress:          types.StringValue(ip.Network),
			IPType:             types.StringValue(ip.IPType),
			ProtectionType:     types.StringValue(ip.ProtectionType),
			LocationName:       types.StringValue(ip.LocationName),
			Subnet:             types.StringNull(),
			HasProfile:         types.BoolValue(ip.HasProfile),
			FirewallRulesCount: types.Int64Value(int64(ip.FirewallRulesCount)),
		})
	}
	for _, sub := range ips.Subnets {
		cidr := types.StringValue(sub.Network + "/" + strconv.Itoa(sub.Prefix))
		for _, host := range sub.IPAddresses {
			state.IPs = append(state.IPs, ddosProtectedIPModel{
				IPAddress:          types.StringValue(host.Address),
				IPType:             types.StringValue(sub.IPType),
				ProtectionType:     types.StringValue(sub.ProtectionType),
				LocationName:       types.StringValue(sub.LocationName),
				Subnet:             cidr,
				HasProfile:         types.BoolValue(host.HasProfile),
				FirewallRulesCount: types.Int64Value(int64(host.FirewallRulesCount)),
			})
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_ddos_countries ----

func NewDDoSCountriesDataSource() datasource.DataSource {
	return &ddosCountriesDataSource{}
}

type ddosCountriesDataSource struct {
	client *client.Client
}

type ddosCountriesModel struct {
	ID        types.String       `tfsdk:"id"`
	Countries []ddosCountryModel `tfsdk:"countries"`
}

type ddosCountryModel struct {
	ISOCode types.String `tfsdk:"iso_code"`
	Name    types.String `tfsdk:"name"`
}

func (d *ddosCountriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddos_countries"
}

func (d *ddosCountriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the countries available for DDoS geo-blocking (countries of cubepath_ddos_protection_profile).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Placeholder identifier.", Computed: true},
			"countries": schema.ListNestedAttribute{
				Description: "Countries.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"iso_code": schema.StringAttribute{Description: "ISO 3166 alpha-2 code.", Computed: true},
						"name":     schema.StringAttribute{Description: "Country name.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *ddosCountriesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *ddosCountriesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	countries, err := d.client.DDoS.ListCountries(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DDoS countries", err.Error())
		return
	}
	state := ddosCountriesModel{ID: types.StringValue("ddos_countries"), Countries: []ddosCountryModel{}}
	for _, c := range countries {
		state.Countries = append(state.Countries, ddosCountryModel{
			ISOCode: types.StringValue(c.ISOCode),
			Name:    types.StringValue(c.Name),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_ddos_asns ----

func NewDDoSASNsDataSource() datasource.DataSource {
	return &ddosASNsDataSource{}
}

type ddosASNsDataSource struct {
	client *client.Client
}

type ddosASNsModel struct {
	ID     types.String   `tfsdk:"id"`
	Search types.String   `tfsdk:"search"`
	ASNs   []ddosASNModel `tfsdk:"asns"`
}

type ddosASNModel struct {
	ASN  types.Int64  `tfsdk:"asn"`
	Name types.String `tfsdk:"name"`
}

func (d *ddosASNsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddos_asns"
}

func (d *ddosASNsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Searches the ASNs available for DDoS ASN filtering (asns of cubepath_ddos_protection_profile).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Placeholder identifier.", Computed: true},
			"search": schema.StringAttribute{
				Description: "An ASN number (exact match) or part of a network name. Omit it to list the whole catalog.",
				Optional:    true,
			},
			"asns": schema.ListNestedAttribute{
				Description: "Matching ASNs.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"asn":  schema.Int64Attribute{Description: "AS number.", Computed: true},
						"name": schema.StringAttribute{Description: "Network name.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *ddosASNsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *ddosASNsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ddosASNsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	asns, err := d.client.DDoS.ListASNs(ctx, state.Search.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DDoS ASNs", err.Error())
		return
	}
	state.ID = types.StringValue("ddos_asns")
	state.ASNs = []ddosASNModel{}
	for _, a := range asns {
		state.ASNs = append(state.ASNs, ddosASNModel{ASN: types.Int64Value(a.ASN), Name: types.StringValue(a.Name)})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
