# Example: Deploy a Baremetal Server

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
  name       = "baremetal-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Create project
resource "cubepath_project" "main" {
  name        = "baremetal-project"
  description = "Baremetal server deployment"
}

# Deploy Baremetal server
resource "cubepath_baremetal" "web" {
  hostname      = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  model_name    = "bm.e3-1230v5"
  os_name       = "debian-12"
  password      = "SecurePassword123!"
  ssh_key_names = [cubepath_ssh_key.main.name]

  # Optional: custom user (defaults to root)
  # user = "admin"

  # Optional: disk layout
  # disk_layout_name = "raid1"

  # Optional: enable monitoring
  monitoring_enable = true

  # Optional: power state control
  power_state = "running"

  timeouts {
    create = "30m"
    delete = "10m"
  }
}

# Outputs
output "baremetal_id" {
  value = cubepath_baremetal.web.id
}

output "baremetal_ip" {
  value = cubepath_baremetal.web.main_ip
}

output "baremetal_ipv6" {
  value = cubepath_baremetal.web.ipv6
}

output "baremetal_status" {
  value = cubepath_baremetal.web.status
}

output "baremetal_cpu" {
  value = cubepath_baremetal.web.cpu
}

output "baremetal_ram" {
  value = "${cubepath_baremetal.web.ram} GB"
}

output "baremetal_storage" {
  value = cubepath_baremetal.web.disk_size
}
