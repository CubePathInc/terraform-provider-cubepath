# Example: DDoS mitigation for a game server IP with Premium protection

terraform {
  required_providers {
    cubepath = {
      source  = "cubepathinc/cubepath"
      version = "~> 0.8"
    }
  }
}

provider "cubepath" {}

variable "ip_address" {
  description = "An IP with Premium DDoS protection (see cubepath_ddos_protected_ips)"
  type        = string
}

data "cubepath_ddos_protected_ips" "all" {}

# Source networks allowed to reach the admin ports
resource "cubepath_ddos_prefix_list" "office" {
  name    = "office"
  entries = ["203.0.113.0/24", "198.51.100.10"]
}

resource "cubepath_ddos_protection_profile" "game" {
  ip_address = var.ip_address

  udp_threshold_pps     = 50000
  tcp_syn_threshold_pps = 200

  # Block two countries and only allow the office on the prefix list filter
  country_mode     = 1
  countries        = ["KP", "IR"]
  prefix_list_mode = 2
  prefix_list_ids  = [cubepath_ddos_prefix_list.office.id]
}

# Validate FiveM traffic and rate limit TCP SYNs per source on the web port
resource "cubepath_ddos_firewall_rule" "fivem" {
  ip_address = var.ip_address
  protocol   = 17
  dst_port   = 30120
  action     = 16
}

resource "cubepath_ddos_firewall_rule" "https" {
  ip_address = var.ip_address
  protocol   = 6
  dst_port   = 443
  action     = 60
  tcp_syn    = 100
}
