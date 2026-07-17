resource "mailtrap_webhook" "example" {
  url          = "https://example.com/mailtrap/events"
  webhook_type = "email_sending"
  event_types  = ["delivery", "bounce", "spam_complaint"]
}

# Secret for verifying payload signatures. Only available when the webhook
# is created by Terraform (never returned again by the API).
output "signing_secret" {
  value     = mailtrap_webhook.example.signing_secret
  sensitive = true
}
