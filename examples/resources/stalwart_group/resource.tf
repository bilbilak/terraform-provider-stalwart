resource "stalwart_domain" "example" {
  name = "example.com"
}

resource "stalwart_group" "info" {
  name        = "info-example"
  description = "Info and hello group"

  primary_email = "info@${stalwart_domain.example.name}"
  aliases = [
    "hello@${stalwart_domain.example.name}",
    "contact@${stalwart_domain.example.name}",
  ]

  enabled_permissions = ["email-send", "email-receive"]

  external_members = ["partner@otherdomain.com"]

  depends_on = [stalwart_domain.example]
}
