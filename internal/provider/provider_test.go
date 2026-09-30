package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// Every resource and data source schema must be valid: the framework rejects, for
// example, a default on an attribute that is not computed.
func TestProviderSchemasAreValid(t *testing.T) {
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("%s: %s", d.Summary, d.Detail)
		}
	}
	for _, name := range []string{
		"cubepath_managed_database", "cubepath_managed_database_database", "cubepath_managed_database_user",
		"cubepath_alert_channel", "cubepath_alert_rule",
		"cubepath_ddos_protection_profile", "cubepath_ddos_firewall_rule", "cubepath_ddos_prefix_list",
		"cubepath_network_bgp_peer", "cubepath_lb_target", "cubepath_lb_health_check", "cubepath_dns_record_health_check",
	} {
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("resource %s is not registered", name)
		}
	}
	for _, name := range []string{
		"cubepath_managed_database_plans", "cubepath_managed_database_credentials",
		"cubepath_ddos_protected_ips", "cubepath_ddos_countries", "cubepath_ddos_asns",
		"cubepath_transcoder_job", "cubepath_transcoder_jobs",
		"cubepath_nat_gateways", "cubepath_kubernetes_loadbalancers", "cubepath_vps_isos", "cubepath_baremetal_models",
	} {
		if _, ok := resp.DataSourceSchemas[name]; !ok {
			t.Errorf("data source %s is not registered", name)
		}
	}
}
