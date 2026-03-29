terraform {
  required_providers {
    cubepath = {
      source = "cubepath/cubepath"
    }
  }
}

provider "cubepath" {}

# --- Data Sources ---

data "cubepath_kubernetes_versions" "available" {}

data "cubepath_kubernetes_plans" "all" {}

data "cubepath_kubernetes_addons" "available" {}

# --- Project ---

resource "cubepath_project" "k8s" {
  name        = "kubernetes-project"
  description = "Project for Kubernetes clusters"
}

# --- Kubernetes Cluster ---

resource "cubepath_kubernetes_cluster" "main" {
  name       = "my-cluster"
  project_id = cubepath_project.k8s.id
  location   = "us-mia-1"
  plan       = "gp.small"
  version    = "1.31"

  ha_control_plane   = false
  initial_node_count = 2

  pod_cidr     = "10.42.0.0/16"
  service_cidr = "10.43.0.0/16"
}

# --- Additional Node Pool ---

resource "cubepath_kubernetes_node_pool" "workers" {
  cluster_id = cubepath_kubernetes_cluster.main.id
  name       = "workers"
  plan       = "gp.medium"
  node_count = 3
  auto_scale = true

  labels = {
    "role"        = "worker"
    "environment" = "production"
  }

  taints {
    key    = "dedicated"
    value  = "workers"
    effect = "NoSchedule"
  }
}

# --- Addon: Ingress NGINX ---

resource "cubepath_kubernetes_addon" "ingress_nginx" {
  cluster_id = cubepath_kubernetes_cluster.main.id
  slug       = "ingress-nginx"
}

# --- Addon: Cert Manager with custom values ---

resource "cubepath_kubernetes_addon" "cert_manager" {
  cluster_id = cubepath_kubernetes_cluster.main.id
  slug       = "cert-manager"

  values = jsonencode({
    installCRDs = true
  })
}

# --- Kubeconfig ---

data "cubepath_kubernetes_kubeconfig" "main" {
  cluster_id = cubepath_kubernetes_cluster.main.id
}

# --- Outputs ---

output "cluster_id" {
  value = cubepath_kubernetes_cluster.main.id
}

output "cluster_endpoint" {
  value = cubepath_kubernetes_cluster.main.api_endpoint
}

output "kubeconfig" {
  value     = data.cubepath_kubernetes_kubeconfig.main.kubeconfig
  sensitive = true
}
