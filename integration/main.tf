# Root module exercised by the integration test suite (tests/mailtrap.tftest.hcl)
# against the real Mailtrap API. Run it via scripts/integration-test.sh, which
# builds the provider and wires it in through dev_overrides — no `terraform init`
# is needed because mailtrap is the only required provider.

terraform {
  required_providers {
    mailtrap = {
      source = "mailtrap/mailtrap"
    }
  }
}

# Credentials come from the MAILTRAP_API_TOKEN environment variable.
provider "mailtrap" {}

data "mailtrap_account" "current" {}

resource "mailtrap_project" "test" {
  name = "${local.name}-project"
}

resource "mailtrap_sandbox" "test" {
  project_id = mailtrap_project.test.id
  name       = "${local.name}-sandbox"
}

resource "mailtrap_email_template" "test" {
  name      = "${local.name}-template"
  category  = "${local.name}-category"
  subject   = "Hello from ${local.name}, {{name}}!"
  body_html = "<h1>Hello {{name}}</h1><p>Sent by ${local.name}.</p>"
  body_text = "Hello {{name}}, sent by ${local.name}."
}

resource "mailtrap_contact_list" "test" {
  name = "${local.name}-list"
}

resource "mailtrap_contact_field" "test" {
  name      = "${local.name}-field"
  data_type = "text"
  merge_tag = "${replace(local.name, "-", "_")}_field"
}

resource "mailtrap_webhook" "test" {
  url          = "https://example.com/hooks/${local.name}"
  webhook_type = "email_sending"
  # Required by the API for email_sending webhooks; changing it forces
  # replacement, so it stays constant across the update run.
  sending_stream = "transactional"
  event_types    = ["delivery", "bounce", "spam_complaint"]
}

resource "mailtrap_api_token" "test" {
  # The API has no update endpoint for tokens, so the name deliberately excludes
  # var.name_suffix — the update test run must not force a replacement.
  name = "${var.name_prefix}-token"

  resources = [{
    resource_type = "account"
    resource_id   = data.mailtrap_account.current.id
    access_level  = 100
  }]
}

resource "mailtrap_domain" "test" {
  # Placeholder domain that is never DNS-verified; the tests only exercise
  # create/read/delete. domain_name forces replacement, so no var.name_suffix.
  domain_name = "${var.name_prefix}.example.com"
}

data "mailtrap_project" "test" {
  id = mailtrap_project.test.id
}

data "mailtrap_sandbox" "test" {
  id = mailtrap_sandbox.test.id
}

data "mailtrap_domain" "test" {
  id = mailtrap_domain.test.id
}
