resource "stalwart_domain" "example" {
  name = "example.com"
}

resource "stalwart_dkim" "example" {
  domain    = stalwart_domain.example.name
  algorithm = "Ed25519"
  selector  = "stalwart"
}

output "dns_records" {
  value = stalwart_dkim.example.dns_records
}
