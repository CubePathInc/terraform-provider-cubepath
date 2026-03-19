# Example: Deploy multiple VPS instances with count

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
  name       = "multi-vps-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_project" "main" {
  name        = "multi-vps-project"
  description = "Multiple VPS deployment"
}

# Deploy 3 web servers
resource "cubepath_vps" "web" {
  count = 3

  name          = "web-${count.index + 1}"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.micro"
  template_name = "debian-12"
  ssh_key_names = [cubepath_ssh_key.main.name]

  timeouts {
    create = "15m"
  }
}

output "web_servers" {
  value = {
    for idx, server in cubepath_vps.web :
    server.name => {
      ip     = server.main_ip
      status = server.status
      vcpus  = server.vcpus
      ram_gb = server.ram / 1024
    }
  }
}
