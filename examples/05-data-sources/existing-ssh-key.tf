# Example: Use existing SSH key with data source

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Look up existing SSH key by name
data "cubepath_ssh_key" "existing" {
  name = "demo-terraform-key"  # Replace with your SSH key name
}

# Create project
resource "cubepath_project" "main" {
  name        = "my-project"
  description = "Project using existing SSH key"
}

# Deploy VPS using the existing SSH key
resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"

  # Reference the existing SSH key
  ssh_key_names = [data.cubepath_ssh_key.existing.name]

  timeouts {
    create = "15m"
  }
}

# Outputs
output "ssh_key_info" {
  value = {
    name        = data.cubepath_ssh_key.existing.name
    fingerprint = data.cubepath_ssh_key.existing.fingerprint
    type        = data.cubepath_ssh_key.existing.key_type
  }
}

output "vps_ip" {
  value = cubepath_vps.web.main_ip
}
