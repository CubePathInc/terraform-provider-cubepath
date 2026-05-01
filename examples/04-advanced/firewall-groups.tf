# Example: Firewall Groups
#
# This example demonstrates how to create firewall groups and assign them to VPS instances.
# Firewall groups provide reusable security rules that can be applied to multiple servers.

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# SSH Key
resource "cubepath_ssh_key" "main" {
  name       = "firewall-demo-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Project
resource "cubepath_project" "main" {
  name        = "firewall-demo"
  description = "Firewall groups demonstration"
}

# Firewall Group: Web Server
# Allows HTTP, HTTPS, and SSH traffic
resource "cubepath_firewall_group" "web" {
  name       = "web-server"
  project_id = cubepath_project.main.id
  enabled    = true

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "80,443"
    source    = "0.0.0.0/0"
    comment   = "Allow HTTP and HTTPS from anywhere"
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "22"
    source    = "0.0.0.0/0"
    comment   = "Allow SSH from anywhere"
  }

  # Allow all outgoing traffic
  rule {
    direction = "out"
    protocol  = "tcp"
    comment   = "Allow all outgoing TCP traffic"
  }

  rule {
    direction = "out"
    protocol  = "udp"
    comment   = "Allow all outgoing UDP traffic"
  }
}

# Firewall Group: Database Server
# More restrictive - only allows traffic from private network
resource "cubepath_firewall_group" "database" {
  name       = "database-server"
  project_id = cubepath_project.main.id
  enabled    = true

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "5432"
    source    = "10.0.0.0/8"
    comment   = "PostgreSQL from private network only"
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "3306"
    source    = "10.0.0.0/8"
    comment   = "MySQL from private network only"
  }

  rule {
    direction = "in"
    protocol  = "tcp"
    port      = "22"
    source    = "10.0.0.0/8"
    comment   = "SSH from private network only"
  }

  # Limited outgoing traffic
  rule {
    direction = "out"
    protocol  = "tcp"
    port      = "80,443"
    comment   = "Allow HTTP/HTTPS for updates"
  }
}

# Firewall Group: ICMP
# Allows ping for monitoring
resource "cubepath_firewall_group" "icmp" {
  name       = "allow-ping"
  project_id = cubepath_project.main.id
  enabled    = true

  rule {
    direction = "in"
    protocol  = "icmp"
    comment   = "Allow ping"
  }
}

# Web Server VPS with firewall groups
resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  # Apply web server and ICMP firewall groups
  firewall_group_ids = [
    cubepath_firewall_group.web.id,
    cubepath_firewall_group.icmp.id
  ]

  timeouts {
    create = "15m"
  }
}

# Database VPS with firewall groups
resource "cubepath_vps" "db" {
  name          = "db-server"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  # Apply database and ICMP firewall groups
  firewall_group_ids = [
    cubepath_firewall_group.database.id,
    cubepath_firewall_group.icmp.id
  ]

  timeouts {
    create = "15m"
  }
}

# Outputs
output "web_server_ip" {
  value       = cubepath_vps.web.main_ip
  description = "Web server public IP"
}

output "db_server_ip" {
  value       = cubepath_vps.db.main_ip
  description = "Database server public IP"
}

output "firewall_groups" {
  value = {
    web      = cubepath_firewall_group.web.id
    database = cubepath_firewall_group.database.id
    icmp     = cubepath_firewall_group.icmp.id
  }
  description = "Firewall group IDs"
}
