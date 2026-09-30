package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/cubepath/terraform-provider-cubepath/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ddosProfileResource{}
	_ resource.ResourceWithConfigure   = &ddosProfileResource{}
	_ resource.ResourceWithImportState = &ddosProfileResource{}
	_ resource.Resource                = &ddosFirewallRuleResource{}
	_ resource.ResourceWithConfigure   = &ddosFirewallRuleResource{}
	_ resource.ResourceWithImportState = &ddosFirewallRuleResource{}
	_ resource.Resource                = &ddosPrefixListResource{}
	_ resource.ResourceWithConfigure   = &ddosPrefixListResource{}
	_ resource.ResourceWithImportState = &ddosPrefixListResource{}
)

// ipValidator accepts a single IPv4 or IPv6 address.
type ipValidator struct{}

func (v ipValidator) Description(_ context.Context) string {
	return "value must be a single IPv4 or IPv6 address"
}

func (v ipValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v ipValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if net.ParseIP(req.ConfigValue.ValueString()) == nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid IP address",
			fmt.Sprintf("%q is not a single IP address.", req.ConfigValue.ValueString()))
	}
}

// ---- cubepath_ddos_protection_profile ----

// ddosProfileField is one numeric setting of a protection profile. Defaults are the ones the
// API applies on writes (PUT replaces the whole profile).
type ddosProfileField struct {
	name        string
	def         int64
	min, max    int64
	description string
}

var ddosProfileFields = []ddosProfileField{
	{"tcp_validation_level", 1, 0, 10, "TCP validation level. Mutually exclusive with tcp_validation_sym_level."},
	{"tcp_validation_sym_level", 0, 0, 10, "Symmetric TCP validation level. Needs symmetric_routing."},
	{"udp_validation_level", 0, 0, 10, "UDP validation level."},
	{"invalid_filter_level", 1, 0, 10, "Invalid packet filter level."},
	{"fragmented_filter_level", 1, 0, 10, "Fragmented packet filter level."},
	{"amplification_udp_level", 1, 0, 10, "UDP amplification filter level."},
	{"amplification_tcp_level", 1, 0, 10, "TCP amplification filter level."},
	{"icmp_rate_limit_level", 0, 0, 10, "ICMP rate limit level."},
	{"same_packet_size_level", 1, 0, 10, "Same packet size filter level."},
	{"stateful_firewall_level", 0, 0, 10, "Stateful firewall level. Needs symmetric_routing."},
	{"default_action", 0, 0, 2, "Default action: 0 filter, 1 accept, 2 drop."},
	{"country_mode", 0, 0, 2, "How countries is used: 0 off, 1 blacklist, 2 whitelist."},
	{"asn_mode", 0, 0, 2, "How asns is used: 0 off, 1 blacklist, 2 whitelist."},
	{"prefix_list_mode", 0, 0, 2, "How prefix_list_ids is used: 0 off, 1 blacklist, 2 whitelist."},
	{"udp_threshold_pps", 1000, 1, 100000000, "UDP rate limit, packets per second."},
	{"tcp_threshold_pps", 1000, 1, 100000000, "TCP rate limit, packets per second."},
	{"tcp_syn_threshold_pps", 10, 1, 100000000, "TCP SYN rate limit, packets per second."},
	{"tcp_ack_threshold_pps", 200, 1, 100000000, "TCP ACK rate limit, packets per second."},
	{"icmp_threshold_pps", 100, 1, 100000000, "ICMP rate limit, packets per second."},
	{"udp_threshold_mbps", 100, 1, 100000, "UDP rate limit, Mbps."},
	{"tcp_threshold_mbps", 100, 1, 100000, "TCP rate limit, Mbps."},
	{"tcp_syn_threshold_mbps", 100, 1, 100000, "TCP SYN rate limit, Mbps."},
	{"tcp_ack_threshold_mbps", 100, 1, 100000, "TCP ACK rate limit, Mbps."},
	{"icmp_threshold_mbps", 100, 1, 100000, "ICMP rate limit, Mbps."},
	{"syn_flood_threshold", 0, 0, 10000, "SYN flood threshold (0 disables it)."},
	{"syn_flood_block_secs", 60, 0, 86400, "Seconds a SYN flood source stays blocked."},
}

func NewDDoSProtectionProfileResource() resource.Resource {
	return &ddosProfileResource{}
}

type ddosProfileResource struct {
	client *client.Client
}

func (r *ddosProfileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddos_protection_profile"
}

func (r *ddosProfileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description:   "The IP address.",
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"ip_address": schema.StringAttribute{
			Description: "IP address with Premium DDoS protection (see the cubepath_ddos_protected_ips data " +
				"source). Changing it forces a new profile.",
			Required:      true,
			Validators:    []validator.String{ipValidator{}},
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
		},
		"always_on_mitigation": schema.BoolAttribute{
			Description: "Send the IP's traffic through the scrubbing platform all the time, not only during attacks.",
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(false),
		},
		"symmetric_routing": schema.BoolAttribute{
			Description: "Route the return traffic through the scrubbing platform too. Needs always_on_mitigation; " +
				"required by tcp_validation_sym_level and stateful_firewall_level.",
			Optional: true,
			Computed: true,
			Default:  booldefault.StaticBool(false),
		},
		"countries": schema.SetAttribute{
			Description: "ISO country codes for geo-blocking (see the cubepath_ddos_countries data source); " +
				"country_mode says whether they are blocked or the only ones allowed.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"asns": schema.SetAttribute{
			Description: "ASNs to filter (see the cubepath_ddos_asns data source); asn_mode says how.",
			Optional:    true,
			ElementType: types.Int64Type,
		},
		"prefix_list_ids": schema.SetAttribute{
			Description: "Prefix lists to filter (cubepath_ddos_prefix_list or global lists); prefix_list_mode says how.",
			Optional:    true,
			ElementType: types.StringType,
		},
	}
	for _, f := range ddosProfileFields {
		attrs[f.name] = schema.Int64Attribute{
			Description: fmt.Sprintf("%s %d to %d, defaults to %d.", f.description, f.min, f.max, f.def),
			Optional:    true,
			Computed:    true,
			Default:     int64default.StaticInt64(f.def),
			Validators:  []validator.Int64{Int64Between(f.min, f.max)},
		}
	}
	resp.Schema = schema.Schema{
		Description: "Manages the DDoS protection profile of an IP with Premium protection: filter levels, rate " +
			"limits, geo, ASN and prefix list filtering, and always-on mitigation. Destroying it resets the IP to " +
			"the platform defaults. Import with the IP address.",
		Attributes: attrs,
	}
}

func (r *ddosProfileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

// profileFromPlan reads the profile settings from a plan or state.
func profileFromPlan(ctx context.Context, get func(context.Context, path.Path, interface{}) diag.Diagnostics) (*client.DDoSProtectionProfile, diag.Diagnostics) {
	var diags diag.Diagnostics
	values := map[string]int64{}
	for _, f := range ddosProfileFields {
		var v types.Int64
		diags.Append(get(ctx, path.Root(f.name), &v)...)
		values[f.name] = v.ValueInt64()
	}
	var alwaysOn, symmetric types.Bool
	diags.Append(get(ctx, path.Root("always_on_mitigation"), &alwaysOn)...)
	diags.Append(get(ctx, path.Root("symmetric_routing"), &symmetric)...)
	if diags.HasError() {
		return nil, diags
	}
	var profile client.DDoSProtectionProfile
	raw, _ := json.Marshal(values)
	_ = json.Unmarshal(raw, &profile)
	a, s := boolToInt(alwaysOn.ValueBool()), boolToInt(symmetric.ValueBool())
	profile.AlwaysOnMitigation, profile.SymmetricRouting = &a, &s
	return &profile, diags
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// setProfileState writes the profile settings into state.
func setProfileState(ctx context.Context, state *tfsdk.State, profile *client.DDoSProtectionProfile) diag.Diagnostics {
	var diags diag.Diagnostics
	raw, _ := json.Marshal(profile)
	values := map[string]json.Number{}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	_ = dec.Decode(&values)
	for _, f := range ddosProfileFields {
		n, _ := values[f.name].Int64()
		diags.Append(state.SetAttribute(ctx, path.Root(f.name), n)...)
	}
	alwaysOn := profile.AlwaysOnMitigation != nil && *profile.AlwaysOnMitigation == 1
	symmetric := profile.SymmetricRouting != nil && *profile.SymmetricRouting == 1
	diags.Append(state.SetAttribute(ctx, path.Root("always_on_mitigation"), alwaysOn)...)
	diags.Append(state.SetAttribute(ctx, path.Root("symmetric_routing"), symmetric)...)
	return diags
}

type ddosProfileLists struct {
	Countries   types.Set `tfsdk:"countries"`
	ASNs        types.Set `tfsdk:"asns"`
	PrefixLists types.Set `tfsdk:"prefix_list_ids"`
}

func getProfileLists(ctx context.Context, get func(context.Context, path.Path, interface{}) diag.Diagnostics) (ddosProfileLists, diag.Diagnostics) {
	var l ddosProfileLists
	var diags diag.Diagnostics
	diags.Append(get(ctx, path.Root("countries"), &l.Countries)...)
	diags.Append(get(ctx, path.Root("asns"), &l.ASNs)...)
	diags.Append(get(ctx, path.Root("prefix_list_ids"), &l.PrefixLists)...)
	return l, diags
}

// applyLists replaces the assignments whose value differs from the previous one (all of
// them when previous is nil).
func (r *ddosProfileResource) applyLists(ctx context.Context, ip string, want ddosProfileLists, previous *ddosProfileLists, diags *diag.Diagnostics) {
	if previous == nil || !want.Countries.Equal(previous.Countries) {
		codes := stringsFromSet(ctx, want.Countries, diags)
		for i := range codes {
			codes[i] = strings.ToUpper(codes[i])
		}
		if err := r.client.DDoS.SetProfileCountries(ctx, ip, codes); err != nil {
			diags.AddError("Error setting the profile countries", err.Error())
			return
		}
	}
	if previous == nil || !want.ASNs.Equal(previous.ASNs) {
		if err := r.client.DDoS.SetProfileASNs(ctx, ip, intsFromSet(ctx, want.ASNs, diags)); err != nil {
			diags.AddError("Error setting the profile ASNs", err.Error())
			return
		}
	}
	if previous == nil || !want.PrefixLists.Equal(previous.PrefixLists) {
		if err := r.client.DDoS.SetProfilePrefixLists(ctx, ip, stringsFromSet(ctx, want.PrefixLists, diags)); err != nil {
			diags.AddError("Error setting the profile prefix lists", err.Error())
		}
	}
}

func (r *ddosProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var ip types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("ip_address"), &ip)...)
	profile, d := profileFromPlan(ctx, req.Plan.GetAttribute)
	resp.Diagnostics.Append(d...)
	lists, d := getProfileLists(ctx, req.Plan.GetAttribute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DDoS.PutProfile(ctx, ip.ValueString(), profile); err != nil {
		resp.Diagnostics.AddError("Error creating DDoS protection profile", err.Error())
		return
	}
	resp.State.Raw = req.Plan.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ip.ValueString())...)
	r.applyLists(ctx, ip.ValueString(), lists, nil, &resp.Diagnostics)
}

func (r *ddosProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var ip types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &ip)...)
	lists, d := getProfileLists(ctx, req.State.GetAttribute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	addr := ip.ValueString()

	// The profile endpoint answers with defaults when nothing is stored: check first.
	exists, err := r.profileExists(ctx, addr)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DDoS protected IPs", err.Error())
		return
	}
	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	profile, err := r.client.DDoS.GetProfile(ctx, addr)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DDoS protection profile", err.Error())
		return
	}
	resp.Diagnostics.Append(setProfileState(ctx, &resp.State, profile)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_address"), addr)...)

	countries, err := r.client.DDoS.GetProfileCountries(ctx, addr)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the profile countries", err.Error())
		return
	}
	codes := make([]string, 0, len(countries))
	for _, c := range countries {
		codes = append(codes, c.ISOCode)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("countries"), stringSetOrNull(ctx, codes, lists.Countries, &resp.Diagnostics))...)

	asns, err := r.client.DDoS.GetProfileASNs(ctx, addr)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the profile ASNs", err.Error())
		return
	}
	numbers := make([]int64, 0, len(asns))
	for _, a := range asns {
		numbers = append(numbers, a.ASN)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("asns"), int64SetOrNull(ctx, numbers, lists.ASNs, &resp.Diagnostics))...)

	prefixLists, err := r.client.DDoS.GetProfilePrefixLists(ctx, addr)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the profile prefix lists", err.Error())
		return
	}
	ids := make([]string, 0, len(prefixLists))
	for _, p := range prefixLists {
		ids = append(ids, p.UUID)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("prefix_list_ids"), stringSetOrNull(ctx, ids, lists.PrefixLists, &resp.Diagnostics))...)
}

func (r *ddosProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var ip types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &ip)...)
	profile, d := profileFromPlan(ctx, req.Plan.GetAttribute)
	resp.Diagnostics.Append(d...)
	want, d := getProfileLists(ctx, req.Plan.GetAttribute)
	resp.Diagnostics.Append(d...)
	have, d := getProfileLists(ctx, req.State.GetAttribute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DDoS.PutProfile(ctx, ip.ValueString(), profile); err != nil {
		resp.Diagnostics.AddError("Error updating DDoS protection profile", err.Error())
		return
	}
	resp.State.Raw = req.Plan.Raw
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), ip.ValueString())...)
	r.applyLists(ctx, ip.ValueString(), want, &have, &resp.Diagnostics)
}

func (r *ddosProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var ip types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &ip)...)
	profile, d := profileFromPlan(ctx, req.State.GetAttribute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	addr := ip.ValueString()

	// Deleting the profile leaves the assignments and the always-on redirects in place:
	// clear them first.
	empty := ddosProfileLists{
		Countries:   types.SetNull(types.StringType),
		ASNs:        types.SetNull(types.Int64Type),
		PrefixLists: types.SetNull(types.StringType),
	}
	r.applyLists(ctx, addr, empty, nil, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if *profile.AlwaysOnMitigation == 1 || *profile.SymmetricRouting == 1 {
		off := 0
		profile.AlwaysOnMitigation, profile.SymmetricRouting = &off, &off
		profile.TCPValidationSymLevel, profile.StatefulFirewallLevel = 0, 0
		if err := r.client.DDoS.PutProfile(ctx, addr, profile); err != nil {
			resp.Diagnostics.AddError("Error disabling always-on mitigation", err.Error())
			return
		}
	}
	if err := r.client.DDoS.DeleteProfile(ctx, addr); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting DDoS protection profile", err.Error())
	}
}

func (r *ddosProfileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_address"), req.ID)...)
}

// profileExists reports whether a profile is stored for ip.
func (r *ddosProfileResource) profileExists(ctx context.Context, ip string) (bool, error) {
	ips, err := r.client.DDoS.ListProtectedIPs(ctx)
	if err != nil {
		return false, err
	}
	for _, s := range ips.SingleIPs {
		if s.Network == ip {
			return s.HasProfile, nil
		}
	}
	for _, sub := range ips.Subnets {
		for _, host := range sub.IPAddresses {
			if host.Address == ip {
				return host.HasProfile, nil
			}
		}
	}
	return false, nil
}

// stringSetOrNull builds a set from values. An empty result is stored as null, unless the
// previous value was a known empty set.
func stringSetOrNull(ctx context.Context, values []string, previous types.Set, diags *diag.Diagnostics) types.Set {
	if len(values) == 0 && (previous.IsNull() || previous.IsUnknown()) {
		return types.SetNull(types.StringType)
	}
	set, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)
	return set
}

// ---- cubepath_ddos_firewall_rule ----

func NewDDoSFirewallRuleResource() resource.Resource {
	return &ddosFirewallRuleResource{}
}

type ddosFirewallRuleResource struct {
	client *client.Client
}

type ddosFirewallRuleModel struct {
	ID          types.String `tfsdk:"id"`
	RuleID      types.Int64  `tfsdk:"rule_id"`
	IPAddress   types.String `tfsdk:"ip_address"`
	Protocol    types.Int64  `tfsdk:"protocol"`
	DstPort     types.Int64  `tfsdk:"dst_port"`
	Action      types.Int64  `tfsdk:"action"`
	ActionLabel types.String `tfsdk:"action_label"`
	TCPSyn      types.Int64  `tfsdk:"tcp_syn"`
	TCPAck      types.Int64  `tfsdk:"tcp_ack"`
	TCPSynAck   types.Int64  `tfsdk:"tcp_synack"`
	TCPRst      types.Int64  `tfsdk:"tcp_rst"`
	TCPFin      types.Int64  `tfsdk:"tcp_fin"`
	TCPAll      types.Int64  `tfsdk:"tcp_all"`
	UDP         types.Int64  `tfsdk:"udp"`
	ICMP        types.Int64  `tfsdk:"icmp"`
}

func (r *ddosFirewallRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddos_firewall_rule"
}

func (r *ddosFirewallRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	rate := func(desc string) schema.Int64Attribute {
		return schema.Int64Attribute{
			Description:   desc + " Only used by actions 60 (packets per second) and 61 (Mbps). Defaults to 0.",
			Optional:      true,
			Computed:      true,
			Default:       int64default.StaticInt64(0),
			Validators:    []validator.Int64{Int64Between(0, 100000000)},
			PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
		}
	}
	resp.Schema = schema.Schema{
		Description: "Manages a firewall rule of the DDoS mitigation platform on one of your IPs: match a protocol " +
			"and destination port and drop, accept, validate (FiveM, RDP, DNS, Minecraft, TLS) or rate limit " +
			"per source. Up to 20 rules per IP, one per protocol and port. Rules cannot be edited, so every " +
			"change replaces the rule. Import with ip_address/rule_id.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "ip_address/rule_id.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"rule_id": schema.Int64Attribute{
				Description:   "ID of the rule.",
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"ip_address": schema.StringAttribute{
				Description:   "One of your IP addresses.",
				Required:      true,
				Validators:    []validator.String{ipValidator{}},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"protocol": schema.Int64Attribute{
				Description:   "IP protocol number: 0 any, 1 ICMP, 6 TCP, 17 UDP.",
				Required:      true,
				Validators:    []validator.Int64{Int64Between(0, 255)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"dst_port": schema.Int64Attribute{
				Description:   "Destination port, 0 for any.",
				Required:      true,
				Validators:    []validator.Int64{Int64Between(0, 65535)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"action": schema.Int64Attribute{
				Description: "Action: 0 drop, 1 accept, 2 filter; 10-12 FiveM TCP, 15-17 FiveM UDP, 20 RDP TCP, " +
					"21 RDP UDP, 30 DNS UDP, 31 DNS TCP, 40 Minecraft Java, 50 TLS validation (TCP actions need " +
					"protocol 6, UDP ones 17); 60 source rate limit in pps, 61 source rate limit in Mbps.",
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"action_label": schema.StringAttribute{
				Description:   "Name of the action.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tcp_syn":    rate("TCP SYN limit per source."),
			"tcp_ack":    rate("TCP ACK limit per source."),
			"tcp_synack": rate("TCP SYN-ACK limit per source."),
			"tcp_rst":    rate("TCP RST limit per source."),
			"tcp_fin":    rate("TCP FIN limit per source."),
			"tcp_all":    rate("TCP limit per source."),
			"udp":        rate("UDP limit per source."),
			"icmp":       rate("ICMP limit per source."),
		},
	}
}

func (r *ddosFirewallRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

func (r *ddosFirewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ddosFirewallRuleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.DDoS.CreateFirewallRule(ctx, &client.CreateDDoSFirewallRuleRequest{
		Network:   plan.IPAddress.ValueString(),
		Protocol:  int(plan.Protocol.ValueInt64()),
		DstPort:   int(plan.DstPort.ValueInt64()),
		Action:    int(plan.Action.ValueInt64()),
		TCPSyn:    int(plan.TCPSyn.ValueInt64()),
		TCPAck:    int(plan.TCPAck.ValueInt64()),
		TCPSynAck: int(plan.TCPSynAck.ValueInt64()),
		TCPRst:    int(plan.TCPRst.ValueInt64()),
		TCPFin:    int(plan.TCPFin.ValueInt64()),
		TCPAll:    int(plan.TCPAll.ValueInt64()),
		UDP:       int(plan.UDP.ValueInt64()),
		ICMP:      int(plan.ICMP.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating DDoS firewall rule", err.Error())
		return
	}
	mapDDoSRule(&plan, plan.IPAddress.ValueString(), rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ddosFirewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ddosFirewallRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule, err := r.client.DDoS.GetFirewallRule(ctx, state.IPAddress.ValueString(), int(state.RuleID.ValueInt64()))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DDoS firewall rule", err.Error())
		return
	}
	mapDDoSRule(&state, state.IPAddress.ValueString(), rule)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ddosFirewallRuleResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "DDoS firewall rules cannot be edited; every change replaces the rule.")
}

func (r *ddosFirewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ddosFirewallRuleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DDoS.DeleteFirewallRule(ctx, int(state.RuleID.ValueInt64())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting DDoS firewall rule", err.Error())
	}
}

func (r *ddosFirewallRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	i := strings.LastIndex(req.ID, "/")
	if i <= 0 {
		resp.Diagnostics.AddError("Invalid import ID", "Expected format: ip_address/rule_id")
		return
	}
	id, err := strconv.ParseInt(req.ID[i+1:], 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", "rule_id must be a number: "+err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_address"), req.ID[:i])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("rule_id"), id)...)
}

func mapDDoSRule(state *ddosFirewallRuleModel, ip string, rule *client.DDoSFirewallRule) {
	state.ID = types.StringValue(fmt.Sprintf("%s/%d", ip, rule.ID))
	state.RuleID = types.Int64Value(int64(rule.ID))
	state.IPAddress = types.StringValue(ip)
	state.Protocol = types.Int64Value(int64(rule.Protocol))
	state.DstPort = types.Int64Value(int64(rule.DstPort))
	state.Action = types.Int64Value(int64(rule.Action))
	state.ActionLabel = types.StringValue(rule.ActionLabel)
	state.TCPSyn = types.Int64Value(int64(rule.TCPSyn))
	state.TCPAck = types.Int64Value(int64(rule.TCPAck))
	state.TCPSynAck = types.Int64Value(int64(rule.TCPSynAck))
	state.TCPRst = types.Int64Value(int64(rule.TCPRst))
	state.TCPFin = types.Int64Value(int64(rule.TCPFin))
	state.TCPAll = types.Int64Value(int64(rule.TCPAll))
	state.UDP = types.Int64Value(int64(rule.UDP))
	state.ICMP = types.Int64Value(int64(rule.ICMP))
}

// ---- cubepath_ddos_prefix_list ----

func NewDDoSPrefixListResource() resource.Resource {
	return &ddosPrefixListResource{}
}

type ddosPrefixListResource struct {
	client *client.Client
}

type ddosPrefixListModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Entries     types.Set    `tfsdk:"entries"`
}

func (r *ddosPrefixListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ddos_prefix_list"
}

func (r *ddosPrefixListResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DDoS prefix list: a named set of source networks that protection profiles can " +
			"block or allow (prefix_list_ids). Up to 3 lists per organization and 100 entries per list. " +
			"Import with the list UUID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "The UUID of the prefix list.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Description:   "Name, unique in the organization, up to 64 characters. Changing it forces a new list.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Description:   "Description, up to 255 characters. Changing it forces a new list.",
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"entries": schema.SetAttribute{
				Description: "Networks in CIDR notation (a single IP is written as /32 or /128). Changed in place.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *ddosPrefixListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if c := configureClient(req.ProviderData, &resp.Diagnostics); c != nil {
		r.client = c
	}
}

// normalizeCIDR writes a network the way the API stores it.
func normalizeCIDR(s string) string {
	if _, n, err := net.ParseCIDR(s); err == nil {
		return n.String()
	}
	if ip := net.ParseIP(s); ip != nil {
		if ip.To4() != nil {
			return ip.String() + "/32"
		}
		return ip.String() + "/128"
	}
	return s
}

func (r *ddosPrefixListResource) syncEntries(ctx context.Context, id string, have, want []string, diags *diag.Diagnostics) {
	for i := range want {
		want[i] = normalizeCIDR(want[i])
	}
	add, remove := diffStrings(have, want)
	for _, n := range remove {
		if err := r.client.DDoS.DeletePrefixListEntry(ctx, id, n); err != nil && !isNotFound(err) {
			diags.AddError("Error removing a prefix list entry", err.Error())
			return
		}
	}
	for _, n := range add {
		if err := r.client.DDoS.AddPrefixListEntry(ctx, id, n); err != nil {
			diags.AddError("Error adding a prefix list entry", err.Error())
			return
		}
	}
}

func (r *ddosPrefixListResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ddosPrefixListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	list, err := r.client.DDoS.CreatePrefixList(ctx, plan.Name.ValueString(), plan.Description.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error creating DDoS prefix list", err.Error())
		return
	}
	plan.ID = types.StringValue(list.UUID)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	r.syncEntries(ctx, list.UUID, nil, stringsFromSet(ctx, plan.Entries, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ddosPrefixListResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ddosPrefixListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.DDoS.GetPrefixList(ctx, state.ID.ValueString()); err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DDoS prefix list", err.Error())
		return
	}
	r.readInto(ctx, &state, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ddosPrefixListResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ddosPrefixListModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	have, err := r.client.DDoS.ListPrefixListEntries(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Error reading the prefix list entries", err.Error())
		return
	}
	r.syncEntries(ctx, id, have, stringsFromSet(ctx, plan.Entries, &resp.Diagnostics), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	r.readInto(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *ddosPrefixListResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ddosPrefixListModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DDoS.DeletePrefixList(ctx, state.ID.ValueString()); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting DDoS prefix list", err.Error())
	}
}

func (r *ddosPrefixListResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// readInto refreshes name, description and entries from the API.
func (r *ddosPrefixListResource) readInto(ctx context.Context, state *ddosPrefixListModel, diags *diag.Diagnostics) {
	list, err := r.client.DDoS.GetPrefixList(ctx, state.ID.ValueString())
	if err != nil {
		diags.AddError("Error reading DDoS prefix list", err.Error())
		return
	}
	state.Name = types.StringValue(list.Name)
	if list.Description != "" {
		state.Description = types.StringValue(list.Description)
	} else {
		state.Description = types.StringNull()
	}
	entries, err := r.client.DDoS.ListPrefixListEntries(ctx, state.ID.ValueString())
	if err != nil {
		diags.AddError("Error reading the prefix list entries", err.Error())
		return
	}
	// Keep the configured spelling when it normalizes to the stored network.
	configured := map[string]string{}
	for _, e := range stringsFromSet(ctx, state.Entries, diags) {
		configured[normalizeCIDR(e)] = e
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if c, ok := configured[e]; ok {
			out = append(out, c)
		} else {
			out = append(out, e)
		}
	}
	state.Entries = stringSetOrNull(ctx, out, state.Entries, diags)
}
