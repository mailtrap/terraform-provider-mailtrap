resource "mailtrap_sending_domain" "example" {
  domain_name            = "mail.example.com"
  open_tracking_enabled  = true
  click_tracking_enabled = true
}

# The DNS records to publish for domain verification.
output "dns_records" {
  value = mailtrap_sending_domain.example.dns_records
}
