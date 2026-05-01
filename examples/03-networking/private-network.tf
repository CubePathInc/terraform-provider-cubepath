# Example: VPS with private network

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
  name       = "private-network-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "private-network-project"
  description = "VPS with private networking"
}

# Create private network
resource "cubepath_network" "private" {
  name       = "app-network"
  project_id = cubepath_project.main.id
  location   = "us-mia-1"
  ip_range   = "10.0.1.0"
  prefix     = 24
}

# VPS in private network
resource "cubepath_vps" "app" {
  name          = "app-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  network_id    = cubepath_network.private.id
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

output "network_cidr" {
  value = cubepath_network.private.cidr
}

output "vps_public_ip" {
  value = cubepath_vps.app.main_ip
}

output "vps_private_ip" {
  value = cubepath_vps.app.private_ip
}
