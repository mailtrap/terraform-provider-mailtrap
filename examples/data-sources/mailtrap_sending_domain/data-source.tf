data "mailtrap_sending_domain" "example" {
  id = 12345
}

# DNS records required to authenticate the domain.
output "dns_records" {
  value = data.mailtrap_sending_domain.example.dns_records
}
