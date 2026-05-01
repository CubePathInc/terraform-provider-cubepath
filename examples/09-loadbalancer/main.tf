# Example: Load balancer with listeners and targets

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
  name = "lb-example"
}

# Create two VPS instances as targets
resource "cubepath_vps" "web" {
  count         = 2
  name          = "web-${count.index + 1}"
  project_id    = cubepath_project.main.id
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  ssh_key_ids   = [12]
}

# Create a load balancer
resource "cubepath_loadbalancer" "main" {
  name          = "web-lb"
  plan_name     = "lb.small"
  location_name = "us-mia-1"
  project_id    = cubepath_project.main.id
}

# HTTP listener
resource "cubepath_lb_listener" "http" {
  loadbalancer_id = cubepath_loadbalancer.main.id
  name            = "http"
  protocol        = "http"
  source_port     = 80
  target_port     = 8080
  algorithm       = "round_robin"
}

output "lb_ip" {
  value = cubepath_loadbalancer.main.ip_address
}
