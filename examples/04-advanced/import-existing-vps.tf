# Example: Import existing VPS into Terraform
# This example shows how to manage existing resources with Terraform

terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {}

# Define the existing VPS resource
# Before applying, import it with:
# terraform import cubepath_vps.existing <vps-id>
resource "cubepath_vps" "existing" {
  name          = "existing-server"
  project_id    = 123  # Replace with your project ID
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  ssh_key_names = ["my-key"]  # Replace with your SSH key name

  timeouts {
    create = "15m"
  }

  lifecycle {
    # Prevent accidental deletion
    prevent_destroy = true
  }
}

# Import command:
# terraform import cubepath_vps.existing 21028
