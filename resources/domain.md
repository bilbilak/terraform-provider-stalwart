---
page_title: "stalwart_domain Resource - terraform-provider-stalwart"
subcategory: ""
description: |-
  Manages a domain in Stalwart Mail Server.
---

# stalwart_domain (Resource)

Manages a domain in Stalwart Mail Server. When created, the resource automatically generates RSA and Ed25519 DKIM keys and retrieves DNS configuration information including MTA-STS policy and DKIM selectors.

## Example Usage

```terraform
resource "stalwart_domain" "example" {
  name        = "example.com"
  description = "Example Domain"
}

# Access computed DKIM information
output "dkim_rsa_selector" {
  value = stalwart_domain.example.dkim.rsa.selector
}

output "dkim_ed25519_public_key" {
  value     = stalwart_domain.example.dkim.ed25519.public_key
  sensitive = true
}

output "sts_policy_id" {
  value = stalwart_domain.example.sts_policy
}
```

## Schema

### Required

- `name` (String) The domain name (e.g., "example.com"). Changing this forces a new resource to be created.
- `description` (String) Description of the domain for identification purposes.

### Read-Only

- `id` (Number) The unique principal ID assigned by Stalwart Mail Server.
- `sts_policy` (String) The MTA-STS policy ID extracted from the DNS records. This value is used for the `_mta-sts` TXT record.
- `dkim` (Object) DKIM key information containing both RSA and Ed25519 keys. See [Nested Schema for dkim](#nested-schema-for-dkim).

<a id="nested-schema-for-dkim"></a>
### Nested Schema for `dkim`

Read-Only:

- `rsa` (Object) RSA DKIM key information
  - `selector` (String) The DKIM selector for the RSA key (e.g., "202509r")
  - `public_key` (String) The RSA public key to be published in DNS TXT records
- `ed25519` (Object) Ed25519 DKIM key information
  - `selector` (String) The DKIM selector for the Ed25519 key (e.g., "202509e")
  - `public_key` (String) The Ed25519 public key to be published in DNS TXT records

## Import

Domains can be imported using the principal ID:

```shell
terraform import stalwart_domain.example 123
```

Where `123` is the principal ID of the domain in Stalwart Mail Server.
