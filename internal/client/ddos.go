package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// DDoSProtectionProfile is the DDoS mitigation profile of an IP with Premium protection.
// Levels are 0-10; default_action 0=FILTER 1=ACCEPT 2=DROP; the *_mode fields are
// 0=off 1=blacklist 2=whitelist.
type DDoSProtectionProfile struct {
	TCPValidationLevel    int  `json:"tcp_validation_level"`
	TCPValidationSymLevel int  `json:"tcp_validation_sym_level"`
	UDPValidationLevel    int  `json:"udp_validation_level"`
	InvalidFilterLevel    int  `json:"invalid_filter_level"`
	FragmentedFilterLevel int  `json:"fragmented_filter_level"`
	AmplificationUDPLevel int  `json:"amplification_udp_level"`
	AmplificationTCPLevel int  `json:"amplification_tcp_level"`
	ICMPRateLimitLevel    int  `json:"icmp_rate_limit_level"`
	SamePacketSizeLevel   int  `json:"same_packet_size_level"`
	StatefulFirewallLevel int  `json:"stateful_firewall_level"`
	DefaultAction         int  `json:"default_action"`
	CountryMode           int  `json:"country_mode"`
	ASNMode               int  `json:"asn_mode"`
	PrefixListMode        int  `json:"prefix_list_mode"`
	UDPThresholdPPS       int  `json:"udp_threshold_pps"`
	TCPThresholdPPS       int  `json:"tcp_threshold_pps"`
	TCPSynThresholdPPS    int  `json:"tcp_syn_threshold_pps"`
	TCPAckThresholdPPS    int  `json:"tcp_ack_threshold_pps"`
	ICMPThresholdPPS      int  `json:"icmp_threshold_pps"`
	UDPThresholdMbps      int  `json:"udp_threshold_mbps"`
	TCPThresholdMbps      int  `json:"tcp_threshold_mbps"`
	TCPSynThresholdMbps   int  `json:"tcp_syn_threshold_mbps"`
	TCPAckThresholdMbps   int  `json:"tcp_ack_threshold_mbps"`
	ICMPThresholdMbps     int  `json:"icmp_threshold_mbps"`
	SynFloodThreshold     int  `json:"syn_flood_threshold"`
	SynFloodBlockSecs     int  `json:"syn_flood_block_secs"`
	AlwaysOnMitigation    *int `json:"always_on_mitigation,omitempty"`
	SymmetricRouting      *int `json:"symmetric_routing,omitempty"`
}

// DDoSProtectedIP is a single IP with Premium protection
type DDoSProtectedIP struct {
	Network            string `json:"network"`
	IPType             string `json:"ip_type"`
	ProtectionType     string `json:"protection_type"`
	LocationName       string `json:"location_name"`
	HasProfile         bool   `json:"has_profile"`
	FirewallRulesCount int    `json:"firewall_rules_count"`
}

// DDoSProtectedSubnetIP is one host of a protected IPv4 subnet
type DDoSProtectedSubnetIP struct {
	Address            string `json:"address"`
	HasProfile         bool   `json:"has_profile"`
	FirewallRulesCount int    `json:"firewall_rules_count"`
}

// DDoSProtectedSubnet is a subnet with Premium protection
type DDoSProtectedSubnet struct {
	Network            string                  `json:"network"`
	Prefix             int                     `json:"prefix"`
	IPType             string                  `json:"ip_type"`
	ProtectionType     string                  `json:"protection_type"`
	LocationName       string                  `json:"location_name"`
	HasProfile         bool                    `json:"has_profile"`
	FirewallRulesCount int                     `json:"firewall_rules_count"`
	IPAddresses        []DDoSProtectedSubnetIP `json:"ip_addresses"`
}

// DDoSProtectedIPs lists the organization's IPs and subnets with Premium protection
type DDoSProtectedIPs struct {
	SingleIPs []DDoSProtectedIP     `json:"single_ips"`
	Subnets   []DDoSProtectedSubnet `json:"subnets"`
}

// DDoSCountry is a country of the geo-blocking catalog
type DDoSCountry struct {
	ISOCode string `json:"iso_code"`
	Name    string `json:"name"`
}

// DDoSASN is an ASN of the ASN filtering catalog
type DDoSASN struct {
	ASN  int64  `json:"asn"`
	Name string `json:"name"`
}

// DDoSPrefixList is a list of source prefixes usable in protection profiles
type DDoSPrefixList struct {
	UUID         string `json:"uuid"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	IsGlobal     bool   `json:"is_global"`
	EntriesCount int    `json:"entries_count"`
}

// DDoSFirewallRule is a firewall rule of the DDoS mitigation platform on one IP
type DDoSFirewallRule struct {
	ID          int    `json:"id"`
	Network     string `json:"network"`
	Protocol    int    `json:"protocol"`
	DstPort     int    `json:"dst_port"`
	Action      int    `json:"action"`
	ActionLabel string `json:"action_label"`
	TCPSyn      int    `json:"tcp_syn"`
	TCPAck      int    `json:"tcp_ack"`
	TCPSynAck   int    `json:"tcp_synack"`
	TCPRst      int    `json:"tcp_rst"`
	TCPFin      int    `json:"tcp_fin"`
	TCPAll      int    `json:"tcp_all"`
	UDP         int    `json:"udp"`
	ICMP        int    `json:"icmp"`
}

// CreateDDoSFirewallRuleRequest creates a DDoS firewall rule
type CreateDDoSFirewallRuleRequest struct {
	Network   string `json:"network"`
	Protocol  int    `json:"protocol"`
	DstPort   int    `json:"dst_port"`
	Action    int    `json:"action"`
	TCPSyn    int    `json:"tcp_syn"`
	TCPAck    int    `json:"tcp_ack"`
	TCPSynAck int    `json:"tcp_synack"`
	TCPRst    int    `json:"tcp_rst"`
	TCPFin    int    `json:"tcp_fin"`
	TCPAll    int    `json:"tcp_all"`
	UDP       int    `json:"udp"`
	ICMP      int    `json:"icmp"`
}

// DDoSService handles the DDoS Mitigation API calls
type DDoSService struct {
	client *Client
}

// NewDDoSService creates a new DDoS Mitigation service
func NewDDoSService(client *Client) *DDoSService {
	return &DDoSService{client: client}
}

// ListProtectedIPs lists the organization's IPs and subnets with Premium protection
func (s *DDoSService) ListProtectedIPs(ctx context.Context) (*DDoSProtectedIPs, error) {
	var result DDoSProtectedIPs
	if err := s.client.Get(ctx, "/ddos-mitigation/ips", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetProfile retrieves the protection profile of an IP. The API answers with the defaults
// when no profile is stored.
func (s *DDoSService) GetProfile(ctx context.Context, ip string) (*DDoSProtectionProfile, error) {
	var result DDoSProtectionProfile
	if err := s.client.Get(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// PutProfile creates or fully replaces the protection profile of an IP
func (s *DDoSService) PutProfile(ctx context.Context, ip string, profile *DDoSProtectionProfile) error {
	return s.client.Put(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip), profile, nil)
}

// DeleteProfile deletes the protection profile of an IP, resetting it to the defaults
func (s *DDoSService) DeleteProfile(ctx context.Context, ip string) error {
	return s.client.Delete(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip))
}

// GetProfileCountries lists the countries assigned to a profile
func (s *DDoSService) GetProfileCountries(ctx context.Context, ip string) ([]DDoSCountry, error) {
	var result struct {
		Countries []DDoSCountry `json:"countries"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip)+"/countries", &result); err != nil {
		return nil, err
	}
	return result.Countries, nil
}

// SetProfileCountries replaces the countries assigned to a profile
func (s *DDoSService) SetProfileCountries(ctx context.Context, ip string, isoCodes []string) error {
	if isoCodes == nil {
		isoCodes = []string{}
	}
	return s.client.Put(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip)+"/countries", map[string][]string{"iso_codes": isoCodes}, nil)
}

// GetProfileASNs lists the ASNs assigned to a profile
func (s *DDoSService) GetProfileASNs(ctx context.Context, ip string) ([]DDoSASN, error) {
	var result struct {
		ASNs []DDoSASN `json:"asns"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip)+"/asns", &result); err != nil {
		return nil, err
	}
	return result.ASNs, nil
}

// SetProfileASNs replaces the ASNs assigned to a profile
func (s *DDoSService) SetProfileASNs(ctx context.Context, ip string, asns []int64) error {
	if asns == nil {
		asns = []int64{}
	}
	return s.client.Put(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip)+"/asns", map[string][]int64{"asns": asns}, nil)
}

// GetProfilePrefixLists lists the prefix lists assigned to a profile
func (s *DDoSService) GetProfilePrefixLists(ctx context.Context, ip string) ([]DDoSPrefixList, error) {
	var result struct {
		PrefixLists []DDoSPrefixList `json:"prefix_lists"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip)+"/prefix-lists", &result); err != nil {
		return nil, err
	}
	return result.PrefixLists, nil
}

// SetProfilePrefixLists replaces the prefix lists assigned to a profile
func (s *DDoSService) SetProfilePrefixLists(ctx context.Context, ip string, uuids []string) error {
	if uuids == nil {
		uuids = []string{}
	}
	return s.client.Put(ctx, "/ddos-mitigation/profiles/"+url.PathEscape(ip)+"/prefix-lists", map[string][]string{"uuids": uuids}, nil)
}

// ListCountries lists the geo-blocking country catalog
func (s *DDoSService) ListCountries(ctx context.Context) ([]DDoSCountry, error) {
	var result struct {
		Countries []DDoSCountry `json:"countries"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/countries", &result); err != nil {
		return nil, err
	}
	return result.Countries, nil
}

// ListASNs lists the ASN catalog. search is an ASN number or a name fragment; empty lists everything.
func (s *DDoSService) ListASNs(ctx context.Context, search string) ([]DDoSASN, error) {
	path := "/ddos-mitigation/asns"
	if search != "" {
		path += "?search=" + url.QueryEscape(search)
	}
	var result struct {
		ASNs []DDoSASN `json:"asns"`
	}
	if err := s.client.Get(ctx, path, &result); err != nil {
		return nil, err
	}
	return result.ASNs, nil
}

// ListPrefixLists lists the organization's prefix lists and the global ones
func (s *DDoSService) ListPrefixLists(ctx context.Context) ([]DDoSPrefixList, error) {
	var result struct {
		PrefixLists []DDoSPrefixList `json:"prefix_lists"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/prefix-lists", &result); err != nil {
		return nil, err
	}
	return result.PrefixLists, nil
}

// GetPrefixList finds a prefix list by UUID. The API has no detail route, so it searches the list.
func (s *DDoSService) GetPrefixList(ctx context.Context, uuid string) (*DDoSPrefixList, error) {
	lists, err := s.ListPrefixLists(ctx)
	if err != nil {
		return nil, err
	}
	for i := range lists {
		if lists[i].UUID == uuid {
			return &lists[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "Prefix list not found"}
}

// CreatePrefixList creates a prefix list. The API does not return the new UUID, so the
// list is looked up by name afterwards (names are unique within the organization).
func (s *DDoSService) CreatePrefixList(ctx context.Context, name, description string) (*DDoSPrefixList, error) {
	body := map[string]interface{}{"name": name}
	if description != "" {
		body["description"] = description
	}
	if err := s.client.Post(ctx, "/ddos-mitigation/prefix-lists", body, nil); err != nil {
		return nil, err
	}
	lists, err := s.ListPrefixLists(ctx)
	if err != nil {
		return nil, err
	}
	for i := range lists {
		if !lists[i].IsGlobal && lists[i].Name == name {
			return &lists[i], nil
		}
	}
	return nil, fmt.Errorf("prefix list %q was created but could not be found", name)
}

// DeletePrefixList deletes a prefix list, its entries and its assignments
func (s *DDoSService) DeletePrefixList(ctx context.Context, uuid string) error {
	return s.client.Delete(ctx, "/ddos-mitigation/prefix-lists/"+url.PathEscape(uuid))
}

// ListPrefixListEntries lists the networks of a prefix list, in CIDR notation
func (s *DDoSService) ListPrefixListEntries(ctx context.Context, uuid string) ([]string, error) {
	var result []struct {
		Network string `json:"network"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/prefix-lists/"+url.PathEscape(uuid)+"/entries", &result); err != nil {
		return nil, err
	}
	entries := make([]string, 0, len(result))
	for _, e := range result {
		entries = append(entries, e.Network)
	}
	return entries, nil
}

// AddPrefixListEntry adds a network to a prefix list
func (s *DDoSService) AddPrefixListEntry(ctx context.Context, uuid, network string) error {
	return s.client.Post(ctx, "/ddos-mitigation/prefix-lists/"+url.PathEscape(uuid)+"/entries", map[string]string{"network": network}, nil)
}

// DeletePrefixListEntry removes a network from a prefix list
func (s *DDoSService) DeletePrefixListEntry(ctx context.Context, uuid, network string) error {
	return s.client.Delete(ctx, "/ddos-mitigation/prefix-lists/"+url.PathEscape(uuid)+"/entries/"+network)
}

// ListFirewallRules lists the DDoS firewall rules of an IP
func (s *DDoSService) ListFirewallRules(ctx context.Context, ip string) ([]DDoSFirewallRule, error) {
	var result struct {
		Rules []DDoSFirewallRule `json:"rules"`
	}
	if err := s.client.Get(ctx, "/ddos-mitigation/firewall-rules/"+ip, &result); err != nil {
		return nil, err
	}
	return result.Rules, nil
}

// GetFirewallRule finds a rule by ID among the rules of an IP. It returns a 404 APIError
// when the rule does not exist.
func (s *DDoSService) GetFirewallRule(ctx context.Context, ip string, id int) (*DDoSFirewallRule, error) {
	rules, err := s.ListFirewallRules(ctx, ip)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].ID == id {
			return &rules[i], nil
		}
	}
	return nil, &APIError{StatusCode: 404, Message: "Not Found", Detail: "Rule not found"}
}

// CreateFirewallRule creates a rule on one IP. The API does not return the rule ID, so the
// rule is looked up afterwards by protocol and port (unique per IP).
func (s *DDoSService) CreateFirewallRule(ctx context.Context, req *CreateDDoSFirewallRuleRequest) (*DDoSFirewallRule, error) {
	if err := s.client.Post(ctx, "/ddos-mitigation/firewall-rules", req, nil); err != nil {
		return nil, err
	}
	rules, err := s.ListFirewallRules(ctx, req.Network)
	if err != nil {
		return nil, err
	}
	for i := range rules {
		if rules[i].Protocol == req.Protocol && rules[i].DstPort == req.DstPort {
			return &rules[i], nil
		}
	}
	return nil, fmt.Errorf("the rule was created but could not be found on %s", req.Network)
}

// DeleteFirewallRule deletes a rule by ID
func (s *DDoSService) DeleteFirewallRule(ctx context.Context, id int) error {
	return s.client.Delete(ctx, "/ddos-mitigation/firewall-rules/"+strconv.Itoa(id))
}
