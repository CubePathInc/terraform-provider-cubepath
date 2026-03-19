# Example: VPS with password authentication

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

resource "cubepath_project" "main" {
  name        = "password-vps-project"
  description = "VPS with password authentication"
}

resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.micro"
  template_name = "ubuntu-22"
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

output "vps_ip" {
  value = cubepath_vps.web.main_ip
}

output "vps_status" {
  value = cubepath_vps.web.status
}
