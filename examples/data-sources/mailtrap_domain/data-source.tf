data "mailtrap_domain" "example" {
  id = 12345
}

# DNS records required to authenticate the domain.
output "dns_records" {
  value = data.mailtrap_domain.example.dns_records
}
