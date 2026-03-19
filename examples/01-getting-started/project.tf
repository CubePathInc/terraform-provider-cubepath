# Example: Create a project

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Create a project to organize your resources
resource "cubepath_project" "example" {
  name        = "my-project"
  description = "My first CubePath project"
}

# Outputs
output "project_id" {
  description = "Project ID"
  value       = cubepath_project.example.id
}

output "project_name" {
  description = "Project name"
  value       = cubepath_project.example.name
}

output "created_at" {
  description = "Project creation timestamp"
  value       = cubepath_project.example.created_at
}
