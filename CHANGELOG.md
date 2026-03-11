# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2026-03-11

### Added

- `stalwart_domain` resource — registers a domain principal in Stalwart.
- `stalwart_dkim` resource — generates an Ed25519 or RSA-SHA-256 DKIM signing
  key for a domain; exposes generated DNS records as a computed attribute.
- `stalwart_group` resource — manages a mailing list (`list`) or shared-folder
  group (`group`) principal, with `primary_email`, `aliases`,
  `enabled_permissions`, `disabled_permissions`, and `external_members`.
- `stalwart_account` resource — manages an individual user account with
  `primary_email`, `aliases`, `member_of`, `quota`, `roles`, and `password`
  (write-only, sensitive). Named `account` to match Stalwart terminology.
- `stalwart_dns_records` data source — reads all DNS records Stalwart requires
  for a domain (MX, SPF, DKIM TXT, DMARC TXT).
- `stalwart_domain` data source — reads name and description of a registered domain.
- `stalwart_account` data source — reads account attributes; password and secrets
  are never exposed (not returned by the Stalwart API).
- `stalwart_group` data source — reads group attributes including permissions and
  external members.
- Provider authentication via bearer token, `STALWART_TOKEN` environment
  variable, or HTTP Basic (username + password).
- Domain deletion guard — the provider refuses to delete a domain while any
  user or group still has an email address on it, and reports the offending
  principals by name.
- Ordered email handling — `primary_email` is always stored at index 0 in
  Stalwart's emails array; updates perform a full ordered replace to preserve
  the primary designation.
- Correct resource creation and destruction order enforced through implicit
  Terraform references and explicit `depends_on` where plain strings are used.
- GoReleaser configuration for multi-platform release builds (Linux, macOS,
  Windows, FreeBSD; amd64, arm64, 386, arm).
- GitHub Actions release workflow triggered on `v*` tags.
- `terraform-registry-manifest.json` declaring protocol version `6.0`.
- Full provider documentation in `docs/` compatible with the Terraform
  Provider Registry.
- Multi-domain example in `examples/multi-domain/` demonstrating the
  recommended `for_each` pattern with variable-driven configuration.

[Unreleased]: https://github.com/bilbilak/terraform-provider-stalwart/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/bilbilak/terraform-provider-stalwart/releases/tag/v1.2.0
