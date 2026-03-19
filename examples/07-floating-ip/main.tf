# Example: Floating IP management
# Acquire a floating IP and assign it to a VPS

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
  name = "floating-ip-example"
}

resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  ssh_key_names = ["my-key"]
}

# Acquire and assign a floating IP to the VPS
resource "cubepath_floating_ip" "web" {
  ip_type       = "IPv4"
  location_name = "us-mia-1"
  assign_vps_id = cubepath_vps.web.id
  reverse_dns   = "web.example.com"
}

output "floating_ip" {
  value = cubepath_floating_ip.web.address
}
