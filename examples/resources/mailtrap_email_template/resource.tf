resource "mailtrap_email_template" "example" {
  name      = "Welcome"
  category  = "Onboarding"
  subject   = "Welcome aboard, {{name}}!"
  body_html = "<h1>Hello {{name}}</h1><p>Thanks for signing up.</p>"
  body_text = "Hello {{name}}, thanks for signing up."
}
