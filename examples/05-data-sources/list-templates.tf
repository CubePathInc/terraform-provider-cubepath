# Example: List all available OS templates

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Get all available OS templates
data "cubepath_vps_templates" "all" {}

# Output all templates
output "all_templates" {
  description = "All available OS templates"
  value = [
    for tmpl in data.cubepath_vps_templates.all.templates :
    tmpl.os_name
  ]
}

# Output grouped by OS
output "templates_by_os" {
  description = "Templates grouped by operating system"
  value = {
    debian = [
      for tmpl in data.cubepath_vps_templates.all.templates :
      tmpl.os_name if length(regexall("(?i)debian", tmpl.os_name)) > 0
    ]
    ubuntu = [
      for tmpl in data.cubepath_vps_templates.all.templates :
      tmpl.os_name if length(regexall("(?i)ubuntu", tmpl.os_name)) > 0
    ]
    alma = [
      for tmpl in data.cubepath_vps_templates.all.templates :
      tmpl.os_name if length(regexall("(?i)alma", tmpl.os_name)) > 0
    ]
  }
}

# Output template count
output "template_count" {
  description = "Total number of available templates"
  value       = length(data.cubepath_vps_templates.all.templates)
}
