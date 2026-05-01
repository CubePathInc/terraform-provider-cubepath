# Example: VPS using an existing SSH key (no need to create it)
# This is useful when you already have SSH keys in your CubePath account

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create a project
resource "cubepath_project" "main" {
  name        = "my-project"
  description = "My first project"
}

# Deploy VPS using existing SSH key by name
resource "cubepath_vps" "simple" {
  name          = "my-vps"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"

  # Reference existing SSH key by name (replace with your key name)
  ssh_key_ids = [12]

  timeouts {
    create = "15m"
  }
}

# Outputs
output "vps_ip" {
  value = cubepath_vps.simple.main_ip
}

output "vps_status" {
  value = cubepath_vps.simple.status
}
