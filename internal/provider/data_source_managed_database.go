package provider

import (
	"context"
	"fmt"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = &managedDatabasePlansDataSource{}
	_ datasource.DataSourceWithConfigure = &managedDatabasePlansDataSource{}
	_ datasource.DataSource              = &managedDatabaseCredentialsDataSource{}
	_ datasource.DataSourceWithConfigure = &managedDatabaseCredentialsDataSource{}
)

func configureDataSourceClient(providerData any, resp *datasource.ConfigureResponse) *client.Client {
	if providerData == nil {
		return nil
	}
	c, ok := providerData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.Client, got: %T.", providerData))
		return nil
	}
	return c
}

// ---- cubepath_managed_database_plans ----

func NewManagedDatabasePlansDataSource() datasource.DataSource {
	return &managedDatabasePlansDataSource{}
}

type managedDatabasePlansDataSource struct {
	client *client.Client
}

type managedDatabasePlansModel struct {
	ID           types.String               `tfsdk:"id"`
	Engine       types.String               `tfsdk:"engine"`
	LocationName types.String               `tfsdk:"location_name"`
	Plans        []managedDatabasePlanModel `tfsdk:"plans"`
}

type managedDatabasePlanModel struct {
	UUID         types.String  `tfsdk:"uuid"`
	Name         types.String  `tfsdk:"name"`
	Description  types.String  `tfsdk:"description"`
	Engine       types.String  `tfsdk:"engine"`
	LocationName types.String  `tfsdk:"location_name"`
	CPU          types.Int64   `tfsdk:"cpu"`
	MemoryGB     types.Int64   `tfsdk:"memory_gb"`
	StorageGB    types.Int64   `tfsdk:"storage_gb"`
	MaxReplicas  types.Int64   `tfsdk:"max_replicas"`
	PricePerHour types.Float64 `tfsdk:"price_per_hour"`
}

func (d *managedDatabasePlansDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_database_plans"
}

func (d *managedDatabasePlansDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the Managed Database plans, cheapest first. Sizes and prices are per node: " +
			"a database costs price_per_hour x replicas.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Placeholder identifier.",
				Computed:    true,
			},
			"engine": schema.StringAttribute{
				Description: "Only list plans of this engine: mysql, postgresql or valkey.",
				Optional:    true,
				Validators:  []validator.String{StringOneOf("mysql", "postgresql", "valkey")},
			},
			"location_name": schema.StringAttribute{
				Description: "Only list plans of this location (for example eu-bcn-1).",
				Optional:    true,
			},
			"plans": schema.ListNestedAttribute{
				Description: "Matching plans.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"uuid":           schema.StringAttribute{Description: "Plan UUID, used as plan_uuid.", Computed: true},
						"name":           schema.StringAttribute{Description: "Plan name.", Computed: true},
						"description":    schema.StringAttribute{Description: "Plan description.", Computed: true},
						"engine":         schema.StringAttribute{Description: "Engine of the plan.", Computed: true},
						"location_name":  schema.StringAttribute{Description: "Location of the plan.", Computed: true},
						"cpu":            schema.Int64Attribute{Description: "vCPUs per node.", Computed: true},
						"memory_gb":      schema.Int64Attribute{Description: "Memory per node, in GB.", Computed: true},
						"storage_gb":     schema.Int64Attribute{Description: "Storage per node, in GB.", Computed: true},
						"max_replicas":   schema.Int64Attribute{Description: "Maximum number of nodes.", Computed: true},
						"price_per_hour": schema.Float64Attribute{Description: "Price per node per hour.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *managedDatabasePlansDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *managedDatabasePlansDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state managedDatabasePlansModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	locations, err := d.client.ManagedDatabases.ListPlans(ctx, state.Engine.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading managed database plans", err.Error())
		return
	}

	state.ID = types.StringValue("managed_database_plans")
	state.Plans = []managedDatabasePlanModel{}
	for _, loc := range locations {
		if !state.LocationName.IsNull() && loc.LocationName != state.LocationName.ValueString() {
			continue
		}
		for _, p := range loc.Plans {
			state.Plans = append(state.Plans, managedDatabasePlanModel{
				UUID:         types.StringValue(p.UUID),
				Name:         types.StringValue(p.Name),
				Description:  types.StringValue(p.Description),
				Engine:       types.StringValue(p.Engine),
				LocationName: types.StringValue(loc.LocationName),
				CPU:          types.Int64Value(int64(p.CPU)),
				MemoryGB:     types.Int64Value(int64(p.MemoryGB)),
				StorageGB:    types.Int64Value(int64(p.StorageGB)),
				MaxReplicas:  types.Int64Value(int64(p.MaxReplicas)),
				PricePerHour: types.Float64Value(p.PricePerHour),
			})
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_managed_database_credentials ----

func NewManagedDatabaseCredentialsDataSource() datasource.DataSource {
	return &managedDatabaseCredentialsDataSource{}
}

type managedDatabaseCredentialsDataSource struct {
	client *client.Client
}

type managedDatabaseCredentialsModel struct {
	ID                types.String `tfsdk:"id"`
	ManagedDatabaseID types.String `tfsdk:"managed_database_id"`
	Host              types.String `tfsdk:"host"`
	Port              types.Int64  `tfsdk:"port"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	URI               types.String `tfsdk:"uri"`
}

func (d *managedDatabaseCredentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_managed_database_credentials"
}

func (d *managedDatabaseCredentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads the admin connection credentials of an active managed database. Needs an API token " +
			"with managed_database:write; every read is recorded in the activity log and the API allows " +
			"10 reads every 5 minutes.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "The managed database UUID.", Computed: true},
			"managed_database_id": schema.StringAttribute{
				Description: "UUID of the managed database.",
				Required:    true,
			},
			"host":     schema.StringAttribute{Description: "Host to connect to.", Computed: true},
			"port":     schema.Int64Attribute{Description: "Port to connect to.", Computed: true},
			"username": schema.StringAttribute{Description: "Admin username.", Computed: true},
			"password": schema.StringAttribute{Description: "Admin password.", Computed: true, Sensitive: true},
			"uri": schema.StringAttribute{
				Description: "Connection URI (mysql://, postgresql:// or redis://) with the credentials.",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (d *managedDatabaseCredentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *managedDatabaseCredentialsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state managedDatabaseCredentialsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	creds, err := d.client.ManagedDatabases.GetCredentials(ctx, state.ManagedDatabaseID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading managed database credentials", err.Error())
		return
	}

	state.ID = state.ManagedDatabaseID
	state.Host = types.StringValue(creds.Host)
	state.Port = types.Int64Value(int64(creds.Port))
	state.Username = types.StringValue(creds.Username)
	state.Password = types.StringValue(creds.Password)
	state.URI = types.StringValue(creds.URI)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
