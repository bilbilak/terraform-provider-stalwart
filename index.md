---
page_title: "Stalwart Provider"
subcategory: ""
description: |-
  Terraform provider for managing Stalwart Mail Server resources.
---

# Stalwart Provider

The Stalwart provider allows you to manage [Stalwart Mail Server](https://stalw.art/) resources using Terraform. It provides resources for managing domains, email accounts, and groups, enabling infrastructure-as-code for your email infrastructure.

## Example Usage

```terraform
terraform {
  required_providers {
    stalwart = {
      source  = "bilbilak/stalwart"
      version = "~> 0.1"
    }
  }
}

provider "stalwart" {
  server_hostname = "mail.example.com"
  api_key         = var.stalwart_api_key
}

# Create a domain
resource "stalwart_domain" "example" {
  name        = "example.com"
  description = "Example Domain"
}

# Create an email group
resource "stalwart_group" "support" {
  name        = "support"
  description = "Support Team"
  email       = "support@example.com"

  aliases = [
    "help@example.com"
  ]
}

# Create an email account
resource "stalwart_account" "admin" {
  name        = "admin"
  description = "System Administrator"
  email       = "admin@example.com"

  groups  = [stalwart_group.support.name]
  roles   = ["user", "admin"]
  secrets = [var.admin_password]
}
```

## Authentication

The provider requires two configuration parameters:

- `server_hostname` - The hostname of your Stalwart Mail Server (without https://)
- `api_key` - API key for authentication with the Stalwart API

These can be provided either directly in the provider configuration block or via environment variables.

### Provider Configuration

```terraform
provider "stalwart" {
  server_hostname = "mail.example.com"
  api_key         = "your-api-key-here"
}
```

### Environment Variables

```bash
export STALWART_SERVER_HOSTNAME="mail.example.com"
export STALWART_API_KEY="your-api-key-here"
```

When environment variables are set, the provider configuration block can be empty:

```terraform
provider "stalwart" {
  # Configuration will be read from environment variables
}
```

## Schema

### Required

- `server_hostname` (String) The hostname of the Stalwart Mail Server (without protocol). Can also be set via the `STALWART_SERVER_HOSTNAME` environment variable.
- `api_key` (String, Sensitive) API key for authenticating with the Stalwart API. Can also be set via the `STALWART_API_KEY` environment variable.

## Resources

The provider supports the following resources:

- [stalwart_domain](resources/domain.md) - Manage email domains
- [stalwart_group](resources/group.md) - Manage email groups/distribution lists
- [stalwart_account](resources/account.md) - Manage email accounts/users
