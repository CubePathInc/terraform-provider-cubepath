package provider

import "fmt"

// bmRefConfig is the configuration of the resources that carry a baremetal reference. The
// echo fields take the server uuid, written as users do: from the id attribute.
func bmRefConfig(baseURL string) string {
	return fmt.Sprintf(`
provider "cubepath" {
  api_token   = "test-token"
  base_url    = %q
  max_retries = 0
}

resource "cubepath_baremetal" "web" {
  hostname   = "web1"
  project_id = %d
  location   = "us-mia-1"
  model_name = "c1.metal.plus"
  os_name    = "debian-12"
  password   = "Str0ngPassw0rd1"
}

resource "cubepath_network_route" "r" {
  network_id      = %d
  destination     = "192.168.10.0/24"
  next_hop_type   = "baremetal"
  next_hop_target = cubepath_baremetal.web.id
}

resource "cubepath_network_bgp_peer" "p" {
  network_id  = %d
  peer_type   = "baremetal"
  peer_target = cubepath_baremetal.web.id
  remote_asn  = 65001
}

resource "cubepath_lb_target" "t" {
  loadbalancer_id = %q
  listener_id     = %q
  target_type     = "baremetal"
  target_id       = cubepath_baremetal.web.id
}

resource "cubepath_alert_rule" "a" {
  project_id  = %d
  name        = "bm-network"
  target_type = "baremetal"
  target_id   = cubepath_baremetal.web.id
  metric_type = "network_in"
  operator    = "gt"
  threshold   = 80
  channel_ids = [%q]
}
`, baseURL, fakeProject, fakeNetwork, fakeNetwork, fakeLB, fakeListener, fakeProject, fakeChannel)
}

// floatingIPConfig assigns the floating IP of the fake to the server, by uuid.
func floatingIPConfig(baseURL, baremetal string) string {
	return fmt.Sprintf(`
provider "cubepath" {
  api_token   = "test-token"
  base_url    = %q
  max_retries = 0
}

resource "cubepath_floating_ip" "f" {
  ip_type             = "IPv4"
  location_name       = "us-mia-1"
  assign_baremetal_id = %q
}
`, baseURL, baremetal)
}
