# Example: VPS with SSH key authentication
# This example creates a new SSH key with Terraform
#
# Alternative: If you already have an SSH key in CubePath, you can use it directly:
#   ssh_key_ids = [12]
# (no need to create the cubepath_ssh_key resource)

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create a new SSH key (skip this if using an existing key)
resource "cubepath_ssh_key" "main" {
  name       = "production-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "ssh-vps-project"
  description = "VPS with SSH key authentication"
}

resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

output "vps_ip" {
  value = cubepath_vps.web.main_ip
}

output "vps_specs" {
  value = {
    vcpus     = cubepath_vps.web.vcpus
    ram_gb    = cubepath_vps.web.ram / 1024
    storage   = cubepath_vps.web.storage
    bandwidth = cubepath_vps.web.bandwidth
  }
}
