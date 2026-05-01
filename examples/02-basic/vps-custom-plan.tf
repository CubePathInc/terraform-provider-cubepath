# Example: VPS with different plans and locations

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
  name       = "multi-location-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "multi-location-project"
  description = "VPS across different locations"
}

# Small VPS in Miami
resource "cubepath_vps" "miami_small" {
  name          = "miami-web"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.nano"
  template_name = "debian-12"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

# Medium VPS in Houston
resource "cubepath_vps" "houston_medium" {
  name          = "houston-app"
  project_id    = cubepath_project.main.id
  location      = "us-hou-1"
  plan_name     = "gp.small"
  template_name = "ubuntu-22"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

# Large VPS in Barcelona
resource "cubepath_vps" "barcelona_large" {
  name          = "barcelona-db"
  project_id    = cubepath_project.main.id
  location      = "eu-bcn-1"
  plan_name     = "gp.medium"
  template_name = "ubuntu-24"
  ssh_key_ids   = [tonumber(cubepath_ssh_key.main.id)]

  timeouts {
    create = "15m"
  }
}

output "servers" {
  value = {
    miami = {
      ip    = cubepath_vps.miami_small.main_ip
      plan  = cubepath_vps.miami_small.plan_name
      vcpus = cubepath_vps.miami_small.vcpus
      ram   = "${cubepath_vps.miami_small.ram / 1024}GB"
    }
    houston = {
      ip    = cubepath_vps.houston_medium.main_ip
      plan  = cubepath_vps.houston_medium.plan_name
      vcpus = cubepath_vps.houston_medium.vcpus
      ram   = "${cubepath_vps.houston_medium.ram / 1024}GB"
    }
    barcelona = {
      ip    = cubepath_vps.barcelona_large.main_ip
      plan  = cubepath_vps.barcelona_large.plan_name
      vcpus = cubepath_vps.barcelona_large.vcpus
      ram   = "${cubepath_vps.barcelona_large.ram / 1024}GB"
    }
  }
}
