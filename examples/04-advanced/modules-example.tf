# Example: Using variables and modules pattern
# Demonstrates best practices for reusable infrastructure

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Variables
variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "ssh_public_key_path" {
  description = "Path to SSH public key"
  type        = string
  default     = "~/.ssh/id_rsa.pub"
}

variable "vps_count" {
  description = "Number of VPS instances"
  type        = number
  default     = 2
}

variable "location" {
  description = "Deployment location"
  type        = string
  default     = "us-mia-1"
}

# Locals for computed values
locals {
  project_name = "${var.environment}-infrastructure"
  vps_plan     = var.environment == "prod" ? "gp.pro" : "gp.micro"
  common_tags = {
    environment = var.environment
    managed_by  = "terraform"
  }
}

# Resources
resource "cubepath_ssh_key" "main" {
  name       = "${var.environment}-key"
  public_key = file(var.ssh_public_key_path)
}

resource "cubepath_project" "main" {
  name        = local.project_name
  description = "Infrastructure for ${var.environment} environment"
}

resource "cubepath_network" "main" {
  name       = "${var.environment}-network"
  project_id = cubepath_project.main.id
  location   = var.location
  ip_range   = "10.0.${var.environment == "prod" ? "1" : "10"}.0"
  prefix     = 24
}

resource "cubepath_vps" "app" {
  count = var.vps_count

  name          = "${var.environment}-app-${count.index + 1}"
  project_id    = cubepath_project.main.id
  location      = var.location
  plan_name     = local.vps_plan
  template_name = "debian-12"
  network_id    = cubepath_network.main.id
  ssh_key_names = [cubepath_ssh_key.main.name]

  timeouts {
    create = "15m"
  }
}

# Outputs
output "environment" {
  value = var.environment
}

output "project_id" {
  value = cubepath_project.main.id
}

output "vps_instances" {
  value = {
    for idx, vps in cubepath_vps.app :
    vps.name => {
      ip         = vps.main_ip
      private_ip = vps.private_ip
      plan       = vps.plan_name
    }
  }
}
