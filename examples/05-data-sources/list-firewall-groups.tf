# Example: List Firewall Groups
#
# This example shows how to query all firewall groups in your organization

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Get all firewall groups
data "cubepath_firewall_groups" "all" {}

# Output all firewall groups
output "all_firewall_groups" {
  value = data.cubepath_firewall_groups.all.groups
}

# Example: Filter firewall groups by project
# This uses local values to filter the results
locals {
  # Replace with your project ID
  target_project_id = 123

  # Filter groups by project ID
  project_firewall_groups = [
    for group in data.cubepath_firewall_groups.all.groups :
    group if group.project_id == local.target_project_id
  ]
}

output "project_firewall_groups" {
  value       = local.project_firewall_groups
  description = "Firewall groups for specific project"
}

# Example: Find groups by name pattern
locals {
  web_firewall_groups = [
    for group in data.cubepath_firewall_groups.all.groups :
    group if can(regex("web", group.name))
  ]
}

output "web_related_groups" {
  value       = local.web_firewall_groups
  description = "Firewall groups with 'web' in the name"
}

# Example: List only enabled groups
locals {
  enabled_groups = [
    for group in data.cubepath_firewall_groups.all.groups :
    group if group.enabled
  ]
}

output "enabled_firewall_groups" {
  value       = local.enabled_groups
  description = "Only enabled firewall groups"
}

# Example: Groups with VPS assigned
locals {
  groups_in_use = [
    for group in data.cubepath_firewall_groups.all.groups :
    group if group.vps_count > 0
  ]
}

output "firewall_groups_in_use" {
  value       = local.groups_in_use
  description = "Firewall groups currently assigned to VPS instances"
}
