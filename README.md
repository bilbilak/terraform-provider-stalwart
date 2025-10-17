# Terraform Provider for Stalwart Mail Server

This provider allows you to manage [Stalwart Mail Server](https://stalw.art/) resources using Terraform.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.21 (for development)
- Stalwart Mail Server with API access enabled

## Using the Provider

### Installation

This provider is available through the Terraform Registry. To use it, add the following to your Terraform configuration:

```hcl
terraform {
  required_providers {
    stalwart = {
      source  = "bilbilak/stalwart"
      version = "~> 1.0"
    }
  }
}
```

### Authentication

Configure the provider with your Stalwart server hostname and API key:

```hcl
provider "stalwart" {
  server_hostname = "mail.example.com"
  api_key         = var.stalwart_api_key
}
```

Or use environment variables:

```bash
export STALWART_SERVER_HOSTNAME="mail.example.com"
export STALWART_API_KEY="your-api-key-here"
```

### Example Usage

```hcl
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
    "help@example.com",
    "contact@example.com"
  ]
}

# Create an email account
resource "stalwart_account" "john" {
  name        = "john"
  description = "John Doe"
  email       = "john@example.com"

  aliases = [
    "j.doe@example.com"
  ]

  groups  = [stalwart_group.support.name]
  roles   = ["user"]
  secrets = [var.john_password]
}
```

## Documentation

Full documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/bilbilak/stalwart/latest/docs).

## Development

### Building the Provider

```bash
git clone https://github.com/bilbilak/terraform-provider-stalwart
cd terraform-provider-stalwart
go build
```

### Testing

```bash
go test ./...
```

### Local Development

For local development and testing, you can use provider development overrides. Create `~/.terraformrc` (Linux/macOS) or `%APPDATA%\terraform.rc` (Windows):

```hcl
provider_installation {
  dev_overrides {
    "bilbilak/stalwart" = "/path/to/terraform-provider-stalwart"
  }

  direct {}
}
```

Then build the provider and use it in your Terraform configurations without running `terraform init`.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the _AGPLv3_. See the [LICENSE](LICENSE.md) file for details.

## Support

- **Issues**: [GitHub Issues](https://github.com/bilbilak/terraform-provider-stalwart/issues)
- **Stalwart Documentation**: [https://stalw.art/docs/](https://stalw.art/docs/)
