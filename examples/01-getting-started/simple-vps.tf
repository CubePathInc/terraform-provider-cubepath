# Example: Deploy a simple VPS

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create SSH key
resource "cubepath_ssh_key" "main" {
  name       = "simple-vps-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Create project
resource "cubepath_project" "main" {
  name        = "simple-vps-project"
  description = "Simple VPS deployment"
}

# Deploy VPS
resource "cubepath_vps" "simple" {
  name          = "my-vps"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

# Outputs
output "vps_id" {
  value = cubepath_vps.simple.id
}

output "vps_ip" {
  value = cubepath_vps.simple.main_ip
}

output "vps_status" {
  value = cubepath_vps.simple.status
}
