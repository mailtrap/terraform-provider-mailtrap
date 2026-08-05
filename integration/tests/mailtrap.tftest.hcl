# Integration test suite for the Mailtrap provider, run against the real API
# with `terraform test` / `tofu test` (see scripts/integration-test.sh).
#
# Runs execute sequentially against shared state; the framework destroys
# everything in reverse order once the last run finishes. On a mid-run failure
# it still attempts the destroy and reports any leftovers — orphaned resources
# are named with var.name_prefix (tftest-* in CI), which makes manual cleanup
# straightforward.

run "create" {
  # Account data source (also feeds the api_token resource scope).
  assert {
    condition     = data.mailtrap_account.current.id > 0
    error_message = "account data source returned a non-positive id"
  }

  assert {
    condition     = length(data.mailtrap_account.current.access_levels) > 0
    error_message = "account data source returned no access levels"
  }

  # Project.
  assert {
    condition     = mailtrap_project.test.name == "${var.name_prefix}${var.name_suffix}-project"
    error_message = "project name does not match the configuration"
  }

  assert {
    condition     = mailtrap_project.test.share_links.admin != "" && mailtrap_project.test.share_links.viewer != ""
    error_message = "project share links were not populated"
  }

  # Sandbox.
  assert {
    condition     = mailtrap_sandbox.test.project_id == mailtrap_project.test.id
    error_message = "sandbox is not attached to the created project"
  }

  assert {
    condition     = mailtrap_sandbox.test.username != "" && mailtrap_sandbox.test.password != ""
    error_message = "sandbox SMTP credentials were not populated"
  }

  assert {
    condition     = length(mailtrap_sandbox.test.smtp_ports) > 0
    error_message = "sandbox SMTP ports were not populated"
  }

  # Email template.
  assert {
    condition     = mailtrap_email_template.test.uuid != ""
    error_message = "email template uuid was not populated"
  }

  assert {
    condition     = mailtrap_email_template.test.name == "${var.name_prefix}${var.name_suffix}-template"
    error_message = "email template name does not match the configuration"
  }

  # Contact list and contact field.
  assert {
    condition     = mailtrap_contact_list.test.id > 0
    error_message = "contact list id was not populated"
  }

  assert {
    condition     = mailtrap_contact_field.test.merge_tag == "${replace("${var.name_prefix}${var.name_suffix}", "-", "_")}_field"
    error_message = "contact field merge tag does not match the configuration"
  }

  # Webhook. signing_secret is only returned on creation.
  assert {
    condition     = length(mailtrap_webhook.test.signing_secret) > 0
    error_message = "webhook signing_secret was not returned on creation"
  }

  assert {
    condition     = mailtrap_webhook.test.active == true
    error_message = "webhook did not default to active"
  }

  # API token. The full token value is only returned on creation.
  assert {
    condition     = length(mailtrap_api_token.test.token) > 4
    error_message = "api token value was not returned on creation"
  }

  assert {
    condition     = length(mailtrap_api_token.test.last_4_digits) == 4
    error_message = "api token last_4_digits was not populated"
  }

  # Sending domain: placeholder domain, never DNS-verified — assertions must
  # not depend on verification status.
  assert {
    condition     = mailtrap_sending_domain.test.domain_name == "${var.name_prefix}.example.com"
    error_message = "sending domain name does not match the configuration"
  }

  assert {
    condition     = mailtrap_sending_domain.test.dns_verified == false
    error_message = "placeholder sending domain unexpectedly reports verified DNS"
  }

  assert {
    condition     = length(mailtrap_sending_domain.test.dns_records) > 0
    error_message = "sending domain DNS records were not populated"
  }

  # Data sources reading the created resources.
  assert {
    condition     = data.mailtrap_project.test.name == mailtrap_project.test.name
    error_message = "project data source does not match the created project"
  }

  assert {
    condition     = data.mailtrap_sandbox.test.project_id == mailtrap_project.test.id
    error_message = "sandbox data source does not match the created sandbox"
  }

  assert {
    condition     = data.mailtrap_sending_domain.test.domain_name == mailtrap_sending_domain.test.domain_name
    error_message = "sending domain data source does not match the created domain"
  }
}

# OpenTofu does not support run.<name> references inside assert conditions,
# so the create run's ids are handed to later runs via the expected_ids
# variable — that works with both CLIs.
run "update" {
  variables {
    name_suffix  = "-updated"
    expected_ids = run.create.ids
  }

  # Every mutable resource must be updated in place: same id as after create.
  assert {
    condition     = mailtrap_project.test.id == var.expected_ids.project
    error_message = "project was replaced instead of updated in place"
  }

  assert {
    condition     = mailtrap_sandbox.test.id == var.expected_ids.sandbox
    error_message = "sandbox was replaced instead of updated in place"
  }

  assert {
    condition     = mailtrap_email_template.test.id == var.expected_ids.email_template
    error_message = "email template was replaced instead of updated in place"
  }

  assert {
    condition     = mailtrap_contact_list.test.id == var.expected_ids.contact_list
    error_message = "contact list was replaced instead of updated in place"
  }

  assert {
    condition     = mailtrap_contact_field.test.id == var.expected_ids.contact_field
    error_message = "contact field was replaced instead of updated in place"
  }

  assert {
    condition     = mailtrap_webhook.test.id == var.expected_ids.webhook
    error_message = "webhook was replaced instead of updated in place"
  }

  # api_token and sending_domain configs are unchanged by design (no update
  # endpoint / domain_name forces replacement) — they must survive untouched.
  assert {
    condition     = mailtrap_api_token.test.id == var.expected_ids.api_token
    error_message = "api token was unexpectedly replaced"
  }

  assert {
    condition     = mailtrap_sending_domain.test.id == var.expected_ids.sending_domain
    error_message = "sending domain was unexpectedly replaced"
  }

  # The renames actually took effect.
  assert {
    condition     = mailtrap_project.test.name == "${var.name_prefix}-updated-project"
    error_message = "project name was not updated"
  }

  assert {
    condition     = mailtrap_sandbox.test.name == "${var.name_prefix}-updated-sandbox"
    error_message = "sandbox name was not updated"
  }

  assert {
    condition     = mailtrap_email_template.test.name == "${var.name_prefix}-updated-template"
    error_message = "email template name was not updated"
  }

  assert {
    condition     = mailtrap_contact_list.test.name == "${var.name_prefix}-updated-list"
    error_message = "contact list name was not updated"
  }

  assert {
    condition     = mailtrap_webhook.test.url == "https://example.com/hooks/${var.name_prefix}-updated"
    error_message = "webhook url was not updated"
  }
}

# Idempotency signal: planning the update configuration again must be a no-op.
# The assertions below catch replacements (a replacement makes the planned ids
# unknown); pending in-place updates are not assertable from inside a run
# block, so integration-test.sh additionally checks this run's captured JSON
# plan and fails unless every resource change is a no-op.
run "plan_after_update" {
  command = plan

  variables {
    name_suffix = "-updated"
    # run.update already proved these match run.create's ids. (OpenTofu only
    # exposes the previous run here, so run.create is not referencable.)
    expected_ids = run.update.ids
  }

  assert {
    condition     = mailtrap_project.test.id == var.expected_ids.project
    error_message = "follow-up plan wants to replace the project"
  }

  assert {
    condition     = mailtrap_sandbox.test.id == var.expected_ids.sandbox
    error_message = "follow-up plan wants to replace the sandbox"
  }

  assert {
    condition     = mailtrap_email_template.test.id == var.expected_ids.email_template
    error_message = "follow-up plan wants to replace the email template"
  }

  assert {
    condition     = mailtrap_contact_list.test.id == var.expected_ids.contact_list
    error_message = "follow-up plan wants to replace the contact list"
  }

  assert {
    condition     = mailtrap_contact_field.test.id == var.expected_ids.contact_field
    error_message = "follow-up plan wants to replace the contact field"
  }

  assert {
    condition     = mailtrap_webhook.test.id == var.expected_ids.webhook
    error_message = "follow-up plan wants to replace the webhook"
  }

  assert {
    condition     = mailtrap_api_token.test.id == var.expected_ids.api_token
    error_message = "follow-up plan wants to replace the api token"
  }

  assert {
    condition     = mailtrap_sending_domain.test.id == var.expected_ids.sending_domain
    error_message = "follow-up plan wants to replace the sending domain"
  }
}
