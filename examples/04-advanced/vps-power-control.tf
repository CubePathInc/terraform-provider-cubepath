# Example: VPS Power Control
#
# This example demonstrates how to control the power state of a VPS.
# The power_state attribute can be set to "running" or "stopped".

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
  name       = "power-control-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Create project
resource "cubepath_project" "main" {
  name        = "power-control-project"
  description = "VPS power control example"
}

# Deploy VPS with explicit power state
resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_names = [cubepath_ssh_key.main.name]

  # Power state control
  # Set to "running" to start the VPS (default)
  # Set to "stopped" to stop the VPS
  power_state = "running"

  timeouts {
    create = "15m"
  }
}

# Example: VPS that starts stopped (useful for scheduled workloads)
resource "cubepath_vps" "batch_worker" {
  name          = "batch-worker"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_names = [cubepath_ssh_key.main.name]

  # Start in stopped state - useful for batch jobs
  # Can be started later by changing to "running"
  power_state = "stopped"

  timeouts {
    create = "15m"
  }
}

# Outputs
output "web_server_ip" {
  value = cubepath_vps.web.main_ip
}

output "web_server_power_state" {
  value = cubepath_vps.web.power_state
}

output "batch_worker_power_state" {
  value = cubepath_vps.batch_worker.power_state
}
