# Consumed by later run blocks in tests/mailtrap.tftest.hcl (via the
# expected_ids variable) to assert that updates happened in place. OpenTofu
# does not allow run.<name> references inside assert conditions, so the ids
# are passed forward through run-level variables instead.

output "ids" {
  value = {
    account        = data.mailtrap_account.current.id
    project        = mailtrap_project.test.id
    sandbox        = mailtrap_sandbox.test.id
    email_template = mailtrap_email_template.test.id
    contact_list   = mailtrap_contact_list.test.id
    contact_field  = mailtrap_contact_field.test.id
    webhook        = mailtrap_webhook.test.id
    api_token      = mailtrap_api_token.test.id
    sending_domain = mailtrap_sending_domain.test.id
  }
}
