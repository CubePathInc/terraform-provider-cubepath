# Example: Deploy VPS using data sources to select plan dynamically

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

variable "location" {
  description = "Deployment location"
  type        = string
  default     = "us-mia-1"
}

variable "min_vcpus" {
  description = "Minimum vCPUs required"
  type        = number
  default     = 4
}

variable "min_ram_gb" {
  description = "Minimum RAM in GB"
  type        = number
  default     = 8
}

# Get available locations
data "cubepath_locations" "all" {}

# Get VPS plans for selected location
data "cubepath_vps_plans" "location" {
  location = var.location
}

# Get available OS templates
data "cubepath_vps_templates" "all" {}

# Filter plans that meet requirements
locals {
  suitable_plans = [
    for plan in data.cubepath_vps_plans.location.plans :
    plan if plan.cpu >= var.min_vcpus && plan.ram / 1024 >= var.min_ram_gb
  ]

  # Select cheapest suitable plan
  selected_plan = local.suitable_plans[
    index(local.suitable_plans[*].price_per_hour, min(local.suitable_plans[*].price_per_hour...))
  ]
}

# Deploy resources
resource "cubepath_ssh_key" "main" {
  name       = "data-source-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "data-source-project"
  description = "Project deployed using data sources"
}

resource "cubepath_vps" "optimized" {
  name          = "optimized-vps"
  project_id    = cubepath_project.main.id
  location      = var.location
  plan_name     = local.selected_plan.plan_name
  template_name = "debian-12"
  ssh_key_names = [cubepath_ssh_key.main.name]

  timeouts {
    create = "15m"
  }
}

# Outputs
output "available_locations" {
  value = [for loc in data.cubepath_locations.all.locations : loc.location_name]
}

output "selected_plan" {
  value = {
    name            = local.selected_plan.plan_name
    vcpus           = local.selected_plan.cpu
    ram_gb          = local.selected_plan.ram / 1024
    storage_gb      = local.selected_plan.storage
    price_per_hour  = local.selected_plan.price_per_hour
    price_per_month = local.selected_plan.price_per_hour * 24 * 30
  }
}

output "vps_details" {
  value = {
    ip     = cubepath_vps.optimized.main_ip
    status = cubepath_vps.optimized.status
    plan   = cubepath_vps.optimized.plan_name
  }
}
