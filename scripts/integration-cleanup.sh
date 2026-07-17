#!/usr/bin/env bash
#
# Deletes leftover integration-test resources from the Mailtrap test account,
# making test runs idempotent: a failed or interrupted run can orphan
# resources (the account's plan allows a single project, and api_token names
# must be unique, so leftovers make the next run fail with 422s).
#
# Everything whose name starts with the `tftest` prefix is deleted — this
# also covers CI's per-run `tftest-<run_id>-<tool>` prefixes. The token used
# for authentication is never deleted, even if its name matches.
#
# Usage:
#   MAILTRAP_API_TOKEN=... ./scripts/integration-cleanup.sh

set -euo pipefail

if [[ -z "${MAILTRAP_API_TOKEN:-}" ]]; then
  echo "error: MAILTRAP_API_TOKEN must be set" >&2
  exit 2
fi

if ! command -v jq > /dev/null; then
  echo "error: jq is required (https://jqlang.org)" >&2
  exit 2
fi

api="https://mailtrap.io/api"
failed=0

# CI sets TF_VAR_name_prefix to tftest-<run_id>-<tool>, which the base prefix
# already covers; a custom local prefix gets its own sweep pass.
prefixes=("tftest")
if [[ -n "${TF_VAR_name_prefix:-}" && "${TF_VAR_name_prefix}" != tftest* ]]; then
  prefixes+=("$TF_VAR_name_prefix")
fi

req() { # method path
  curl -fsS -X "$1" -H "Api-Token: $MAILTRAP_API_TOKEN" "$api$2"
}

delete_matching() { # label list_path items_jq name_jq prefix delete_path_fmt
  local label="$1" list_path="$2" items_jq="$3" name_jq="$4" prefix="$5" delete_path_fmt="$6"
  local id name
  req GET "$list_path" \
    | jq -r --arg p "$prefix" "$items_jq | select($name_jq | startswith(\$p)) | [.id, $name_jq] | @tsv" \
    | while IFS=$'\t' read -r id name; do
      echo "    deleting $label $name (id $id)"
      # shellcheck disable=SC2059
      if ! req DELETE "$(printf "$delete_path_fmt" "$id")" > /dev/null; then
        echo "warning: failed to delete $label $name (id $id)" >&2
        # The subshell exit code surfaces the failure to the caller.
        exit 1
      fi
    done || failed=1
}

account_id="$(req GET /accounts | jq -r '.[0].id')"

for prefix in "${prefixes[@]}"; do
  echo "==> Sweeping leftover '$prefix*' resources from account $account_id"

  # Deleting a project also deletes its sandboxes (inboxes).
  delete_matching "project" "/accounts/$account_id/projects" \
    '.[]' '.name' "$prefix" "/accounts/$account_id/projects/%s"

  delete_matching "email template" "/accounts/$account_id/email_templates" \
    '.[]' '.name' "$prefix" "/accounts/$account_id/email_templates/%s"

  delete_matching "contact list" "/accounts/$account_id/contacts/lists" \
    '.[]' '.name' "$prefix" "/accounts/$account_id/contacts/lists/%s"

  delete_matching "contact field" "/accounts/$account_id/contacts/fields" \
    '.[]' '.name' "$prefix" "/accounts/$account_id/contacts/fields/%s"

  # Webhooks have no name; the test config encodes the prefix in the URL.
  delete_matching "webhook" "/accounts/$account_id/webhooks" \
    '.data[]' '.url' "https://example.com/hooks/$prefix" "/accounts/$account_id/webhooks/%s"

  # Never delete the token this script authenticates with.
  auth_last4="${MAILTRAP_API_TOKEN: -4}"
  delete_matching "api token" "/accounts/$account_id/api_tokens" \
    ".[] | select(.last_4_digits != \"$auth_last4\")" '.name' "$prefix" \
    "/accounts/$account_id/api_tokens/%s"

  # The test config uses <prefix>.example.com.
  delete_matching "sending domain" "/accounts/$account_id/sending_domains" \
    '.data[]' '.domain_name' "$prefix" "/accounts/$account_id/sending_domains/%s"
done

if [[ "$failed" -ne 0 ]]; then
  echo "error: some leftovers could not be deleted; the test run would hit 422s" >&2
  exit 1
fi
