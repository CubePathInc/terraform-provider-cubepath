# CubePath Terraform Provider - Examples

Complete examples demonstrating how to use the CubePath Terraform provider.

## Quick Start

Set your API token:
```bash
export CUBE_API_TOKEN="your-api-token"
```

## Working with SSH Keys

You have two options when deploying VPS instances:

### Option 1: Use Existing SSH Key (Recommended)
If you already have SSH keys in your CubePath account, just reference them by name:

```hcl
resource "cubepath_vps" "web" {
  # ... other config ...
  ssh_key_ids = [12]  # No need to create it
}
```

See `01-getting-started/vps-with-existing-key.tf` for a complete example.

### Option 2: Create New SSH Key with Terraform
Let Terraform manage the SSH key lifecycle:

```hcl
resource "cubepath_ssh_key" "main" {
  name       = "my-terraform-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

resource "cubepath_vps" "web" {
  # ... other config ...
  ssh_key_ids = [tonumber(cubepath_ssh_key.main.id)]
}
```

See `01-getting-started/simple-vps.tf` for a complete example.

**Note:** If you get "SSH key already exists" error, either use Option 1 (reference by name) or import the existing key with `terraform import cubepath_ssh_key.main <key-id>`.

## Example Categories

### 01. Getting Started
Basic examples for learning the provider:
- `ssh-key.tf` - Create and manage SSH keys
- `project.tf` - Create a project
- `simple-vps.tf` - Deploy VPS (creates new SSH key)
- `vps-with-existing-key.tf` - Deploy VPS using existing SSH key

### 02. Basic VPS Deployment
Different VPS authentication and configuration options:
- `vps-ssh-key.tf` - VPS with SSH key authentication
- `vps-password.tf` - VPS with password authentication
- `vps-ssh-and-password.tf` - VPS with both methods
- `vps-custom-plan.tf` - VPS with different plans and locations
- `multiple-vps.tf` - Deploy multiple VPS with count

### 03. Networking
Private network examples:
- `private-network.tf` - VPS with private network
- `multi-tier-network.tf` - Multi-tier architecture (web + database)

### 04. Advanced
Advanced patterns and techniques:
- `import-existing-vps.tf` - Import existing VPS into Terraform
- `modules-example.tf` - Variables, locals, and module patterns
- `vps-rename-example.tf` - Rename VPS without destroying it
- `firewall-groups.tf` - Create and assign firewall groups to VPS

### 05. Data Sources
Query CubePath API for available resources:
- `list-locations.tf` - List all available locations
- `list-plans.tf` - List VPS plans and pricing
- `list-templates.tf` - List OS templates
- `existing-ssh-key.tf` - Look up existing SSH key by name
- `list-firewall-groups.tf` - List and filter firewall groups
- `deploy-with-data-sources.tf` - Deploy VPS using data sources

### 06. Baremetal
Baremetal server deployment examples.

### 07. Floating IP
Floating IP management:
- `main.tf` - Acquire floating IP, assign to VPS, configure reverse DNS

### 08. DNS
DNS zone and record management:
- `main.tf` - Create zone, A/CNAME/MX/TXT records

### 09. Load Balancer
Load balancer with listeners:
- `main.tf` - Create LB, HTTP listener, VPS targets

### 10. CDN
CDN zone with origins and rules:
- `main.tf` - Create CDN zone, origins, cache rules, WAF rules

### 11. New Data Sources
Query DNS zones, LB plans, and CDN plans:
- `main.tf` - List DNS zones, LB plans, CDN plans

### 12. Kubernetes
Kubernetes cluster deployment:
- `main.tf` - Create cluster, node pools, addons

### 13. Availability Groups
High availability with availability groups:
- `main.tf` - Create availability group, deploy VPS across different physical hosts, query groups

## Usage

Navigate to any example directory and run:

```bash
cd 01-getting-started
terraform init
terraform apply
```

For examples with variables:
```bash
terraform apply -var="root_password=SecurePass123"
```

## Clean Up

Always destroy resources after testing:
```bash
terraform destroy
```

## Documentation

For full provider documentation, see the main [README](../README.md).
