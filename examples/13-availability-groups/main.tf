# Example: Availability Groups for High Availability

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create project
resource "cubepath_project" "main" {
  name        = "ha-project"
  description = "High availability project"
}

# Create an availability group
# VPS in the same group are distributed across different physical hosts
resource "cubepath_availability_group" "web" {
  project_id    = cubepath_project.main.id
  name          = "web-servers"
  description   = "Web tier - spread across hosts for HA"
  location_name = "us-mia-1"
}

# Create SSH key
resource "cubepath_ssh_key" "main" {
  name       = "ha-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Deploy VPS instances in the availability group
resource "cubepath_vps" "web" {
  count = 3

  name          = "web-${count.index + 1}"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_names = [cubepath_ssh_key.main.name]

  availability_group_uuid = cubepath_availability_group.web.id

  timeouts {
    create = "15m"
  }
}

# Query existing availability groups
data "cubepath_availability_groups" "all" {
  project_id = cubepath_project.main.id

  depends_on = [cubepath_availability_group.web]
}

# Outputs
output "availability_group_uuid" {
  value = cubepath_availability_group.web.id
}

output "availability_group_strategy" {
  value = cubepath_availability_group.web.strategy
}

output "all_groups" {
  value = data.cubepath_availability_groups.all.groups
}
