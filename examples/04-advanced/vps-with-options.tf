# Example: VPS with IPv4, Backups, Firewall and Cloud-init

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
  name       = "options-vps-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Create project
resource "cubepath_project" "main" {
  name        = "options-vps-project"
  description = "VPS with advanced options"
}

# Create firewall group
resource "cubepath_firewall_group" "web" {
  project_id = cubepath_project.main.id
  name       = "web-firewall"
  enabled    = true

  rules = [
    {
      direction = "in"
      protocol  = "tcp"
      port      = "22"
      source    = "0.0.0.0/0"
      comment   = "SSH"
    },
    {
      direction = "in"
      protocol  = "tcp"
      port      = "80"
      source    = "0.0.0.0/0"
      comment   = "HTTP"
    },
    {
      direction = "in"
      protocol  = "tcp"
      port      = "443"
      source    = "0.0.0.0/0"
      comment   = "HTTPS"
    }
  ]
}

# Deploy VPS with all options
resource "cubepath_vps" "full" {
  name          = "full-options-vps"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  # New options
  ipv4               = true # Enable IPv4 (default: true, adds $1.50/month)
  enable_backups     = true # Enable automatic backups
  firewall_group_ids = [cubepath_firewall_group.web.id]

  # Custom cloud-init configuration
  custom_cloudinit = <<-EOF
    #cloud-config
    packages:
      - nginx
      - curl
    runcmd:
      - systemctl enable nginx
      - systemctl start nginx
  EOF

  timeouts {
    create = "15m"
  }
}

# Deploy VPS with IPv6 only (no IPv4 to save costs)
resource "cubepath_vps" "ipv6_only" {
  name          = "ipv6-only-vps"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  ipv4 = false # IPv6 only (saves $1.50/month)

  timeouts {
    create = "15m"
  }
}

# Outputs
output "full_vps_ip" {
  value = cubepath_vps.full.main_ip
}

output "full_vps_ipv6" {
  value = cubepath_vps.full.ipv6
}

output "ipv6_only_vps_ipv6" {
  value       = cubepath_vps.ipv6_only.ipv6
  description = "This VPS only has IPv6"
}
