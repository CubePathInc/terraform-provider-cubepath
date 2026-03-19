# Example: Rename a VPS without destroying it
#
# This example demonstrates that you can change the VPS name in Terraform
# and it will perform a rename operation instead of destroying and recreating the VPS.
#
# Previously, changing the name would trigger a destroy+create cycle.
# Now, the provider uses the API's rename endpoint to update the name in place.

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# SSH Key
resource "cubepath_ssh_key" "demo" {
  name       = "my-ssh-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Project
resource "cubepath_project" "app" {
  name        = "rename-demo"
  description = "Demo of VPS rename functionality"
}

# VPS - Try changing the name and running terraform apply
# The VPS will be renamed, not destroyed and recreated
resource "cubepath_vps" "web" {
  name          = "web-server-v1"  # Change this to "web-server-v2" and apply
  project_id    = cubepath_project.app.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_names = [cubepath_ssh_key.demo.name]

  timeouts {
    create = "15m"
  }
}

output "vps_id" {
  value       = cubepath_vps.web.id
  description = "VPS ID (will remain the same after rename)"
}

output "vps_name" {
  value       = cubepath_vps.web.name
  description = "Current VPS hostname"
}

output "vps_ip" {
  value = cubepath_vps.web.main_ip
}

# To test the rename functionality:
# 1. terraform apply (creates VPS with name "web-server-v1")
# 2. Change name to "web-server-v2" in the resource above
# 3. terraform apply (will show: ~ name: "web-server-v1" -> "web-server-v2")
# 4. Verify the VPS ID remains the same (not destroyed/recreated)
