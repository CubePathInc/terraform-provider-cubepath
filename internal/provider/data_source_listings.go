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
	_ datasource.DataSource              = &natGatewaysDataSource{}
	_ datasource.DataSourceWithConfigure = &natGatewaysDataSource{}
	_ datasource.DataSource              = &kubernetesLoadBalancersDataSource{}
	_ datasource.DataSourceWithConfigure = &kubernetesLoadBalancersDataSource{}
	_ datasource.DataSource              = &vpsISOsDataSource{}
	_ datasource.DataSourceWithConfigure = &vpsISOsDataSource{}
)

// ---- cubepath_nat_gateways ----

func NewNATGatewaysDataSource() datasource.DataSource {
	return &natGatewaysDataSource{}
}

type natGatewaysDataSource struct {
	client *client.Client
}

type natGatewaysModel struct {
	ID          types.String      `tfsdk:"id"`
	ProjectID   types.Int64       `tfsdk:"project_id"`
	NATGateways []natGatewayModel `tfsdk:"nat_gateways"`
}

type natGatewayModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Label        types.String `tfsdk:"label"`
	Status       types.String `tfsdk:"status"`
	PlanName     types.String `tfsdk:"plan_name"`
	NetworkID    types.Int64  `tfsdk:"network_id"`
	ProjectID    types.Int64  `tfsdk:"project_id"`
	LocationName types.String `tfsdk:"location_name"`
	PrivateIP    types.String `tfsdk:"private_ip"`
	Protected    types.Bool   `tfsdk:"protected"`
}

func (d *natGatewaysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nat_gateways"
}

func (d *natGatewaysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the organization's NAT gateways.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Placeholder identifier.", Computed: true},
			"project_id": schema.Int64Attribute{
				Description: "Only list the NAT gateways of this project.",
				Optional:    true,
			},
			"nat_gateways": schema.ListNestedAttribute{
				Description: "NAT gateways.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":            schema.StringAttribute{Description: "UUID.", Computed: true},
						"name":          schema.StringAttribute{Description: "Name.", Computed: true},
						"label":         schema.StringAttribute{Description: "Label.", Computed: true},
						"status":        schema.StringAttribute{Description: "Status.", Computed: true},
						"plan_name":     schema.StringAttribute{Description: "Plan.", Computed: true},
						"network_id":    schema.Int64Attribute{Description: "Private network it serves.", Computed: true},
						"project_id":    schema.Int64Attribute{Description: "Project.", Computed: true},
						"location_name": schema.StringAttribute{Description: "Location.", Computed: true},
						"private_ip":    schema.StringAttribute{Description: "Gateway address in the private network.", Computed: true},
						"protected":     schema.BoolAttribute{Description: "Deletion protection.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *natGatewaysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *natGatewaysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state natGatewaysModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	gateways, err := d.client.NATGateway.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading NAT gateways", err.Error())
		return
	}
	state.ID = types.StringValue("nat_gateways")
	state.NATGateways = []natGatewayModel{}
	for _, g := range gateways {
		if !state.ProjectID.IsNull() && int64(g.ProjectID) != state.ProjectID.ValueInt64() {
			continue
		}
		state.NATGateways = append(state.NATGateways, natGatewayModel{
			ID:           types.StringValue(g.UUID),
			Name:         types.StringValue(g.Name),
			Label:        types.StringValue(g.Label),
			Status:       types.StringValue(g.Status),
			PlanName:     types.StringValue(g.PlanName),
			NetworkID:    types.Int64Value(int64(g.NetworkID)),
			ProjectID:    types.Int64Value(int64(g.ProjectID)),
			LocationName: types.StringValue(g.LocationName),
			PrivateIP:    types.StringValue(g.PrivateIP),
			Protected:    types.BoolValue(g.Protected),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_kubernetes_loadbalancers ----

func NewKubernetesLoadBalancersDataSource() datasource.DataSource {
	return &kubernetesLoadBalancersDataSource{}
}

type kubernetesLoadBalancersDataSource struct {
	client *client.Client
}

type kubernetesLoadBalancersModel struct {
	ID            types.String        `tfsdk:"id"`
	ClusterID     types.String        `tfsdk:"cluster_id"`
	LoadBalancers []kubernetesLBModel `tfsdk:"load_balancers"`
}

type kubernetesLBModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Status         types.String `tfsdk:"status"`
	PlanName       types.String `tfsdk:"plan_name"`
	LocationName   types.String `tfsdk:"location_name"`
	IPAddress      types.String `tfsdk:"ip_address"`
	ListenersCount types.Int64  `tfsdk:"listeners_count"`
	TargetCount    types.Int64  `tfsdk:"target_count"`
}

func (d *kubernetesLoadBalancersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_loadbalancers"
}

func (d *kubernetesLoadBalancersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the load balancers that send traffic to the node pools of a Kubernetes cluster.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "The cluster UUID.", Computed: true},
			"cluster_id": schema.StringAttribute{
				Description: "UUID of the cluster.",
				Required:    true,
			},
			"load_balancers": schema.ListNestedAttribute{
				Description: "Load balancers targeting the cluster.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              schema.StringAttribute{Description: "UUID of the load balancer.", Computed: true},
						"name":            schema.StringAttribute{Description: "Name.", Computed: true},
						"status":          schema.StringAttribute{Description: "Status.", Computed: true},
						"plan_name":       schema.StringAttribute{Description: "Plan.", Computed: true},
						"location_name":   schema.StringAttribute{Description: "Location.", Computed: true},
						"ip_address":      schema.StringAttribute{Description: "Public IP address.", Computed: true},
						"listeners_count": schema.Int64Attribute{Description: "Listeners of the load balancer.", Computed: true},
						"target_count":    schema.Int64Attribute{Description: "Targets pointing at the cluster.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *kubernetesLoadBalancersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *kubernetesLoadBalancersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state kubernetesLoadBalancersModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	lbs, err := d.client.Kubernetes.ListLoadBalancers(ctx, state.ClusterID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading the cluster load balancers", err.Error())
		return
	}
	state.ID = state.ClusterID
	state.LoadBalancers = []kubernetesLBModel{}
	for _, lb := range lbs {
		state.LoadBalancers = append(state.LoadBalancers, kubernetesLBModel{
			ID:             types.StringValue(lb.UUID),
			Name:           types.StringValue(lb.Name),
			Status:         types.StringValue(lb.Status),
			PlanName:       types.StringValue(lb.PlanName),
			LocationName:   types.StringValue(lb.LocationName),
			IPAddress:      stringOrNull(lb.FloatingIP),
			ListenersCount: types.Int64Value(int64(lb.ListenersCount)),
			TargetCount:    types.Int64Value(int64(lb.TargetCount)),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_vps_isos ----

func NewVPSISOsDataSource() datasource.DataSource {
	return &vpsISOsDataSource{}
}

type vpsISOsDataSource struct {
	client *client.Client
}

type vpsISOsModel struct {
	ID    types.String  `tfsdk:"id"`
	VPSID types.Int64   `tfsdk:"vps_id"`
	ISOs  []vpsISOModel `tfsdk:"isos"`
}

type vpsISOModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Filename    types.String `tfsdk:"filename"`
	FileSize    types.Int64  `tfsdk:"file_size"`
	IsMounted   types.Bool   `tfsdk:"is_mounted"`
}

func (d *vpsISOsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vps_isos"
}

func (d *vpsISOsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the ISO images that can be mounted on a VPS (iso_id of cubepath_vps).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "The VPS ID.", Computed: true},
			"vps_id": schema.Int64Attribute{
				Description: "ID of the VPS.",
				Required:    true,
			},
			"isos": schema.ListNestedAttribute{
				Description: "Available ISOs.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Description: "ISO ID.", Computed: true},
						"name":        schema.StringAttribute{Description: "Name.", Computed: true},
						"description": schema.StringAttribute{Description: "Description.", Computed: true},
						"filename":    schema.StringAttribute{Description: "File name.", Computed: true},
						"file_size":   schema.Int64Attribute{Description: "Size in bytes.", Computed: true},
						"is_mounted":  schema.BoolAttribute{Description: "Whether it is mounted on the VPS.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *vpsISOsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *vpsISOsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state vpsISOsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	isos, err := d.client.VPS.ListISOs(ctx, int(state.VPSID.ValueInt64()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading VPS ISOs", err.Error())
		return
	}
	state.ID = types.StringValue(strconv.FormatInt(state.VPSID.ValueInt64(), 10))
	state.ISOs = []vpsISOModel{}
	for _, iso := range isos {
		state.ISOs = append(state.ISOs, vpsISOModel{
			ID:          types.StringValue(iso.ID),
			Name:        types.StringValue(iso.Name),
			Description: types.StringValue(iso.Description),
			Filename:    types.StringValue(iso.Filename),
			FileSize:    types.Int64Value(iso.FileSize),
			IsMounted:   types.BoolValue(iso.IsMounted),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ---- cubepath_baremetal_models ----

var (
	_ datasource.DataSource              = &baremetalModelsDataSource{}
	_ datasource.DataSourceWithConfigure = &baremetalModelsDataSource{}
)

func NewBaremetalModelsDataSource() datasource.DataSource {
	return &baremetalModelsDataSource{}
}

type baremetalModelsDataSource struct {
	client *client.Client
}

type baremetalModelsModel struct {
	ID           types.String          `tfsdk:"id"`
	LocationName types.String          `tfsdk:"location_name"`
	Models       []baremetalOfferModel `tfsdk:"models"`
}

type baremetalOfferModel struct {
	ModelName      types.String  `tfsdk:"model_name"`
	LocationName   types.String  `tfsdk:"location_name"`
	Price          types.Float64 `tfsdk:"price"`
	DiscountValue  types.Float64 `tfsdk:"discount_value"`
	DiscountType   types.String  `tfsdk:"discount_type"`
	Setup          types.Float64 `tfsdk:"setup"`
	CPU            types.String  `tfsdk:"cpu"`
	RAMSize        types.Int64   `tfsdk:"ram_size"`
	RAMType        types.String  `tfsdk:"ram_type"`
	DiskSize       types.String  `tfsdk:"disk_size"`
	DiskType       types.String  `tfsdk:"disk_type"`
	PortGbps       types.Int64   `tfsdk:"port_gbps"`
	StockAvailable types.Int64   `tfsdk:"stock_available"`
}

func (d *baremetalModelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_baremetal_models"
}

func (d *baremetalModelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the baremetal server models on sale (model_name of cubepath_baremetal), with price and stock.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Placeholder identifier.", Computed: true},
			"location_name": schema.StringAttribute{
				Description: "Only list the models of this location.",
				Optional:    true,
			},
			"models": schema.ListNestedAttribute{
				Description: "Models on sale.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"model_name":      schema.StringAttribute{Description: "Model name.", Computed: true},
						"location_name":   schema.StringAttribute{Description: "Location.", Computed: true},
						"price":           schema.Float64Attribute{Description: "Monthly price before discount.", Computed: true},
						"discount_value":  schema.Float64Attribute{Description: "Discount.", Computed: true},
						"discount_type":   schema.StringAttribute{Description: "Discount type: fixed or percentage.", Computed: true},
						"setup":           schema.Float64Attribute{Description: "Setup fee.", Computed: true},
						"cpu":             schema.StringAttribute{Description: "CPU.", Computed: true},
						"ram_size":        schema.Int64Attribute{Description: "RAM, in GB.", Computed: true},
						"ram_type":        schema.StringAttribute{Description: "RAM type.", Computed: true},
						"disk_size":       schema.StringAttribute{Description: "Disks (for example 2x960GB).", Computed: true},
						"disk_type":       schema.StringAttribute{Description: "Disk type.", Computed: true},
						"port_gbps":       schema.Int64Attribute{Description: "Network port speed, in Gbps.", Computed: true},
						"stock_available": schema.Int64Attribute{Description: "Servers in stock.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *baremetalModelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c := configureDataSourceClient(req.ProviderData, resp); c != nil {
		d.client = c
	}
}

func (d *baremetalModelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state baremetalModelsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	locations, err := d.client.Baremetal.ListModels(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error reading baremetal models", err.Error())
		return
	}
	state.ID = types.StringValue("baremetal_models")
	state.Models = []baremetalOfferModel{}
	for _, loc := range locations {
		if !state.LocationName.IsNull() && loc.LocationName != state.LocationName.ValueString() {
			continue
		}
		for _, m := range loc.Models {
			state.Models = append(state.Models, baremetalOfferModel{
				ModelName:      types.StringValue(m.ModelName),
				LocationName:   types.StringValue(loc.LocationName),
				Price:          types.Float64Value(m.Price),
				DiscountValue:  types.Float64Value(m.DiscountValue),
				DiscountType:   types.StringValue(m.DiscountType),
				Setup:          types.Float64Value(m.Setup),
				CPU:            types.StringValue(m.CPU),
				RAMSize:        types.Int64Value(int64(m.RAMSize)),
				RAMType:        types.StringValue(m.RAMType),
				DiskSize:       types.StringValue(m.DiskSize),
				DiskType:       types.StringValue(m.DiskType),
				PortGbps:       types.Int64Value(int64(m.Port)),
				StockAvailable: types.Int64Value(int64(m.StockAvailable)),
			})
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
