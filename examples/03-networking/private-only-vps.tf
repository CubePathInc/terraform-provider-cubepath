# Example: VPS without public IPs (private network only)
#
# Disabling both ipv4 and ipv6 deploys a VPS with no public address.
# A network_id is required so the VPS keeps private connectivity.
# Useful for backend/internal services reached only via VPN, a bastion,
# or a load balancer in the same network.

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

resource "cubepath_ssh_key" "main" {
  name       = "private-only-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "private-only-project"
  description = "VPS reachable only through the private network"
}

resource "cubepath_network" "internal" {
  name       = "internal"
  project_id = cubepath_project.main.id
  location   = "us-mia-1"
  ip_range   = "10.0.1.0"
  prefix     = 24
}

resource "cubepath_vps" "db" {
  name          = "db-internal"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  network_id    = cubepath_network.internal.id
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  ipv4 = false
  ipv6 = false

  timeouts {
    create = "15m"
  }
}

output "private_ip" {
  value = cubepath_vps.db.private_ip
}
