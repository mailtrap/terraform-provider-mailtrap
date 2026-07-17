resource "mailtrap_project" "example" {
  name = "My Project"
}

resource "mailtrap_sandbox" "example" {
  project_id = mailtrap_project.example.id
  name       = "Staging"
}

# SMTP credentials for the sandbox.
output "smtp_username" {
  value     = mailtrap_sandbox.example.username
  sensitive = true
}
