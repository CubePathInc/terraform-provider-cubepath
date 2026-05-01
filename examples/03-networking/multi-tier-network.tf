# Example: Multi-tier architecture with private network
# Web servers + Database server on private network

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
  name       = "multi-tier-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "multi-tier-app"
  description = "Multi-tier application infrastructure"
}

# Private network for internal communication
resource "cubepath_network" "backend" {
  name       = "backend-network"
  project_id = cubepath_project.main.id
  location   = "us-mia-1"
  ip_range   = "10.0.10.0"
  prefix     = 24
}

# Web servers (2 instances)
resource "cubepath_vps" "web" {
  count = 2

  name          = "web-${count.index + 1}"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.starter"
  template_name = "debian-12"
  network_id    = cubepath_network.backend.id
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

# Database server
resource "cubepath_vps" "database" {
  name          = "db-primary"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.medium"
  template_name = "ubuntu-22"
  network_id    = cubepath_network.backend.id
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

output "network_info" {
  value = {
    cidr     = cubepath_network.backend.cidr
    ip_range = cubepath_network.backend.ip_range
  }
}

output "web_servers" {
  value = {
    for idx, server in cubepath_vps.web :
    server.name => {
      public_ip  = server.main_ip
      private_ip = server.private_ip
    }
  }
}

output "database" {
  value = {
    public_ip  = cubepath_vps.database.main_ip
    private_ip = cubepath_vps.database.private_ip
    specs = {
      vcpus = cubepath_vps.database.vcpus
      ram   = "${cubepath_vps.database.ram / 1024}GB"
    }
  }
}
