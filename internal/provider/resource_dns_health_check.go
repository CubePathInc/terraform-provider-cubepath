package provider

import (
	"context"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &dnsHealthCheckResource{}
	_ resource.ResourceWithConfigure   = &dnsHealthCheckResource{}
	_ resource.ResourceWithImportState = &dnsHealthCheckResource{}
)

func NewDNSRecordHealthCheckResource() resource.Resource {
	return &dnsHealthCheckResource{}
}

type dnsHealthCheckResource struct {
	client *client.Client
}

type dnsHealthCheckModel struct {
	ID                 types.String `tfsdk:"id"`
	ZoneUUID           types.String `tfsdk:"zone_uuid"`
	RecordUUID         types.String `tfsdk:"record_uuid"`
	Name               types.String `tfsdk:"name"`
	CheckType          types.String `tfsdk:"check_type"`
	Target             types.String `tfsdk:"target"`
	Port               types.Int64  `tfsdk:"port"`
	Path               types.String `tfsdk:"path"`
	ExpectedStatus     types.Int64  `tfsdk:"expected_status"`
	IntervalSecs       types.Int64  `tfsdk:"interval_secs"`
	TimeoutSecs        types.Int64  `tfsdk:"timeout_secs"`
	HealthyThreshold   types.Int64  `tfsdk:"healthy_threshold"`
	UnhealthyThreshold types.Int64  `tfsdk:"unhealthy_threshold"`
	Enabled            types.Bool   `tfsdk:"enabled"`
	LastStatus         types.String `tfsdk:"last_status"`
}

func (r *dnsHealthCheckResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dns_record_health_check"
}

func (r *dnsHealthCheckResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the health check of an A or AAAA DNS record (one per record): while the endpoint " +
			"fails, the record is left out of DNS answers. Needs a Pro or Business DNS plan and is billed " +
			"monthly while enabled. Import with zone_uuid/record_uuid.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "UUID of the health check.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"zone_uuid": schema.StringAttribute{
				Description:   "UUID of the DNS zone.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"record_uuid": schema.StringAttribute{
				Description:   "UUID of the A or AAAA record.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "Name of the check.",
				Required:    true,
			},
			"check_type": schema.StringAttribute{
				Description: "Probe: http, https, tcp or ping.",
				Required:    true,
				Validators:  []validator.String{StringOneOf("http", "https", "tcp", "ping")},
			},
			"target": schema.StringAttribute{
				Description: "Hostname or IP to probe. Omit it to probe the record's own address.",
				Optional:    true,
			},
			"port": schema.Int64Attribute{
				Description: "Port to probe. Required for tcp; used by http and https.",
				Optional:    true,
				Validators:  []validator.Int64{Int64Between(1, 65535)},
			},
			"path": schema.StringAttribute{
				Description: "Path requested by http and https checks, starting with /.",
				Optional:    true,
			},
			"expected_status": schema.Int64Attribute{
				Description: "HTTP status that counts as healthy, for http and https. Defaults to 200.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(200),
				Validators:  []validator.Int64{Int64Between(100, 599)},
			},
			"interval_secs": schema.Int64Attribute{
				Description: "Seconds between probes, 10 to 3600. Defaults to 60.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(60),
				Validators:  []validator.Int64{Int64Between(10, 3600)},
			},
			"timeout_secs": schema.Int64Attribute{
				Description: "Probe timeout, 1 to 60 and lower than interval_secs. Defaults to 5.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(5),
				Validators:  []validator.Int64{Int64Between(1, 60)},
			},
			"healthy_threshold": schema.Int64Attribute{
				Description: "Consecutive successes to mark it healthy, 1 to 10. Defaults to 2.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(2),
				Validators:  []validator.Int64{Int64Between(1, 10)},
			},
			"unhealthy_threshold": schema.Int64Attribute{
				Description: "Consecutive failures to mark it unhealthy, 1 to 10. Defaults to 3.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(3),
				Validators:  []validator.Int64{Int64Between(1, 10)},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the check runs (and is billed). Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"last_status": schema.StringAttribute{
				Description: "Last result: healthy, unhealthy or unknown.",
				Computed:    true,
			},
		},
	}
}

func (r *dnsHealthCheckResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *dnsHealthCheckResource) put(ctx context.Context, m *dnsHealthCheckModel) (*client.DNSHealthCheck, error) {
	expected := int(m.ExpectedStatus.ValueInt64())
	return r.client.DNS.PutHealthCheck(ctx, m.ZoneUUID.ValueString(), m.RecordUUID.ValueString(), &client.DNSHealthCheck{
		Name:               m.Name.ValueString(),
		CheckType:          m.CheckType.ValueString(),
		Target:             knownString(m.Target),
		Port:               knownInt(m.Port),
		Path:               knownString(m.Path),
		ExpectedStatus:     &expected,
		IntervalSecs:       int(m.IntervalSecs.ValueInt64()),
		TimeoutSecs:        int(m.TimeoutSecs.ValueInt64()),
		HealthyThreshold:   int(m.HealthyThreshold.ValueInt64()),
		UnhealthyThreshold: int(m.UnhealthyThreshold.ValueInt64()),
		Enabled:            m.Enabled.ValueBool(),
	})
}

func (r *dnsHealthCheckResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan dnsHealthCheckModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hc, err := r.put(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS health check", err.Error())
		return
	}
	plan.ID = types.StringValue(hc.UUID)
	plan.LastStatus = types.StringValue(hc.LastStatus)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnsHealthCheckResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state dnsHealthCheckModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hc, err := r.client.DNS.GetHealthCheck(ctx, state.ZoneUUID.ValueString(), state.RecordUUID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNS health check", err.Error())
		return
	}

	state.ID = types.StringValue(hc.UUID)
	state.Name = types.StringValue(hc.Name)
	state.CheckType = types.StringValue(hc.CheckType)
	state.Target = stringOrNull(hc.Target)
	state.Path = stringOrNull(hc.Path)
	if hc.Port != nil {
		state.Port = types.Int64Value(int64(*hc.Port))
	} else {
		state.Port = types.Int64Null()
	}
	if hc.ExpectedStatus != nil {
		state.ExpectedStatus = types.Int64Value(int64(*hc.ExpectedStatus))
	}
	state.IntervalSecs = types.Int64Value(int64(hc.IntervalSecs))
	state.TimeoutSecs = types.Int64Value(int64(hc.TimeoutSecs))
	state.HealthyThreshold = types.Int64Value(int64(hc.HealthyThreshold))
	state.UnhealthyThreshold = types.Int64Value(int64(hc.UnhealthyThreshold))
	state.Enabled = types.BoolValue(hc.Enabled)
	state.LastStatus = types.StringValue(hc.LastStatus)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *dnsHealthCheckResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan dnsHealthCheckModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hc, err := r.put(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNS health check", err.Error())
		return
	}
	plan.ID = types.StringValue(hc.UUID)
	plan.LastStatus = types.StringValue(hc.LastStatus)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *dnsHealthCheckResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state dnsHealthCheckModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.DNS.DeleteHealthCheck(ctx, state.ZoneUUID.ValueString(), state.RecordUUID.ValueString())
	if err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting DNS health check", err.Error())
	}
}

func (r *dnsHealthCheckResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	zone, record, err := splitImportID(req.ID, "zone_uuid/record_uuid")
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("zone_uuid"), zone)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("record_uuid"), record)...)
}
