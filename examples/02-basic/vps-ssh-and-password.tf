# Example: VPS with both SSH key and password authentication
# Provides SSH key-based access (recommended) and password backup access

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

resource "cubepath_ssh_key" "main" {
  name       = "dual-auth-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "dual-auth-project"
  description = "VPS with dual authentication"
}

resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.starter"
  template_name = "debian-12"

  # Both authentication methods
  ssh_key_names = [cubepath_ssh_key.main.name]
  password      = var.root_password

  timeouts {
    create = "15m"
  }
}

variable "root_password" {
  description = "Root password (min 8 chars, uppercase, lowercase, number)"
  type        = string
  sensitive   = true
}

output "ssh_key_fingerprint" {
  value = cubepath_ssh_key.main.fingerprint
}

output "vps_ip" {
  value = cubepath_vps.web.main_ip
}

output "vps_status" {
  value = cubepath_vps.web.status
}
