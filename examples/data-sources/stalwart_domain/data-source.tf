data "stalwart_domain" "example" {
  name = "example.com"
}

output "domain_description" {
  value = data.stalwart_domain.example.description
}
