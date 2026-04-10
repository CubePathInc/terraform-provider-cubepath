# Terraform Provider for CubePath Cloud

Official Terraform provider for managing infrastructure on CubePath Cloud.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for development)

## Quick Start

### 1. Set up your API token

```bash
export CUBE_API_TOKEN="your-api-token-here"
```

### 2. Install the provider

```bash
make build
make install
```

### 3. Create your first configuration

See the [examples](./examples) directory for complete usage examples.

## Using the Provider

```hcl
terraform {
  required_providers {
    cubepath = {
      source  = "cubepath/cubepath"
      version = "~> 1.0"
    }
  }
}

provider "cubepath" {
  # API token can be set via CUBE_API_TOKEN environment variable
  # api_token = var.cubepath_api_token
}

# Option 1: Create an SSH key with Terraform
resource "cubepath_ssh_key" "main" {
  name       = "my-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Option 2: Use an existing SSH key (no resource needed)
# Just reference it by name in VPS: ssh_key_names = ["existing-key-name"]

# Create a project
resource "cubepath_project" "app" {
  name        = "my-app"
  description = "My application infrastructure"
}

# Create a private network
resource "cubepath_network" "private" {
  name       = "app-network"
  project_id = cubepath_project.app.id
  location   = "us-mia-1"
  ip_range   = "10.0.1.0"
  prefix     = 24
}

# Create a VPS with private network
resource "cubepath_vps" "web" {
  name          = "web-server"
  project_id    = cubepath_project.app.id
  location      = "us-mia-1"
  plan_name     = "gp.pro"
  template_name = "debian-12"
  network_id    = cubepath_network.private.id
  ssh_key_names = [cubepath_ssh_key.main.name]
}
```

## Available Resources & Data Sources

### Resources
- `cubepath_ssh_key` - Manage SSH keys
- `cubepath_project` - Manage projects
- `cubepath_network` - Manage private networks
- `cubepath_vps` - Manage VPS instances
- `cubepath_baremetal` - Manage baremetal servers
- `cubepath_firewall_group` - Manage firewall groups
- `cubepath_floating_ip` - Manage floating IPs
- `cubepath_dns_zone` - Manage DNS zones
- `cubepath_dns_record` - Manage DNS records
- `cubepath_load_balancer` - Manage load balancers
- `cubepath_lb_listener` - Manage load balancer listeners
- `cubepath_cdn_zone` - Manage CDN zones
- `cubepath_cdn_origin` - Manage CDN origins
- `cubepath_cdn_rule` - Manage CDN cache rules
- `cubepath_cdn_waf_rule` - Manage CDN WAF rules
- `cubepath_kubernetes_cluster` - Manage Kubernetes clusters
- `cubepath_kubernetes_node_pool` - Manage Kubernetes node pools
- `cubepath_kubernetes_addon` - Manage Kubernetes addons
- `cubepath_availability_group` - Manage availability groups for high availability

### Data Sources
- `cubepath_locations` - List available locations
- `cubepath_vps_plans` - List available VPS plans and pricing
- `cubepath_vps_templates` - List available OS templates
- `cubepath_ssh_key` - Look up an existing SSH key
- `cubepath_firewall_groups` - List firewall groups
- `cubepath_dns_zones` - List DNS zones
- `cubepath_lb_plans` - List load balancer plans
- `cubepath_cdn_plans` - List CDN plans
- `cubepath_kubernetes_versions` - List Kubernetes versions
- `cubepath_kubernetes_plans` - List Kubernetes node plans
- `cubepath_kubernetes_addons` - List Kubernetes addons
- `cubepath_kubeconfig` - Get cluster kubeconfig
- `cubepath_availability_groups` - List availability groups for a project

## Examples

See the [examples](./examples) directory for complete usage examples:
- `complete-vps-deployment.tf` - Basic VPS deployment
- `vps-with-password.tf` - VPS with password authentication
- `complete-infrastructure.tf` - **Full infrastructure with networks, data sources, and multiple VPS**
```

## Useful Commands

### Testing your configuration

```bash
# Validate configuration
terraform validate

# Format configuration files
terraform fmt

# Preview changes
terraform plan

# Apply changes
terraform apply

# Destroy resources
terraform destroy
```

### Import existing resources

```bash
# Import a VPS (get ID from dashboard or API)
terraform import cubepath_vps.existing <vps-id>

# Import a project
terraform import cubepath_project.existing <project-id>
```

### Debugging

```bash
# Enable debug logging
export TF_LOG=DEBUG
terraform apply

# Trace-level logging (verbose)
export TF_LOG=TRACE
terraform apply
```

## Troubleshooting

### Error: "Missing API Token"

Make sure you've set the API token:

```bash
export CUBE_API_TOKEN="your-token"
echo $CUBE_API_TOKEN  # Verify it's set
```

### Error: "Provider not found"

Reinstall the provider:

```bash
make install
```

### VPS creation timeout

Increase the timeout in your resource configuration:

```hcl
resource "cubepath_vps" "web" {
  # ... other config ...

  timeouts {
    create = "25m"  # Increase from default
  }
}
```

### Error: "SSH key already exists"

If you get this error, you have 3 options:

**Option 1: Use the existing key** (recommended)
```hcl
resource "cubepath_vps" "web" {
  # ... other config ...
  ssh_key_names = ["your-existing-key-name"]  # Just reference by name
}
```

**Option 2: Import the existing key**
```bash
terraform import cubepath_ssh_key.main <key-id>
```

**Option 3: Change the name**
```hcl
resource "cubepath_ssh_key" "main" {
  name = "my-key-2"  # Use a different name
  # ...
}
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

### Testing

#### Acceptance Tests

In order to run the full suite of Acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

## License

Apache 2.0
