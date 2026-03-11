resource "stalwart_domain" "example" {
  name = "example.com"
}

resource "stalwart_dkim" "example" {
  domain = stalwart_domain.example.name
}

data "stalwart_dns_records" "example" {
  domain = stalwart_dkim.example.domain
}

output "dns_records" {
  value = data.stalwart_dns_records.example.records
}
