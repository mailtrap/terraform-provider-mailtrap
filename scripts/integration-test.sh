#!/usr/bin/env bash
#
# Runs the integration test suite (integration/tests/*.tftest.hcl) against the
# real Mailtrap API using a locally built provider binary.
#
# Usage:
#   MAILTRAP_API_TOKEN=... ./scripts/integration-test.sh [terraform|tofu]
#
# The provider is wired in through a dev_overrides CLI configuration, so no
# `terraform init` (and no registry access) is required.
#
# Before running, leftover tftest-* resources from previous (failed) runs are
# swept via scripts/integration-cleanup.sh (requires jq), so runs are
# idempotent — no manual account cleanup is needed between runs.

set -euo pipefail

tool="${1:-terraform}"
case "$tool" in
  terraform | tofu) ;;
  *)
    echo "usage: $0 [terraform|tofu]" >&2
    exit 2
    ;;
esac

if ! command -v "$tool" > /dev/null; then
  echo "error: $tool is not installed or not on PATH" >&2
  exit 2
fi

if [[ -z "${MAILTRAP_API_TOKEN:-}" ]]; then
  echo "error: MAILTRAP_API_TOKEN must be set (integration tests hit the real Mailtrap API)" >&2
  exit 2
fi

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

echo "==> Cleaning up leftovers from previous runs"
"$repo_root/scripts/integration-cleanup.sh"

echo "==> Building provider"
go build -o "$work_dir/terraform-provider-mailtrap" "$repo_root"

cat > "$work_dir/dev-overrides.tfrc" << EOF
provider_installation {
  dev_overrides {
    "mailtrap/mailtrap" = "$work_dir"
  }
  direct {}
}
EOF

# Honored by both Terraform and OpenTofu.
export TF_CLI_CONFIG_FILE="$work_dir/dev-overrides.tfrc"

echo "==> Running $tool test"
test_log="$work_dir/test-output.jsonl"
# -json -verbose exposes each run's machine-readable plan (test_plan
# messages), needed for the no-op check below; jq renders the stream back
# into the usual human-readable progress lines as it goes.
"$tool" -chdir="$repo_root/integration" test -json -verbose \
  | tee "$test_log" \
  | jq -r 'if .type == "diagnostic" then "\(.diagnostic.severity): \(.diagnostic.summary)\n\(.diagnostic.detail // empty)" else ."@message" // empty end'

# The test framework cannot assert "the plan is empty" from inside a run
# block — the id assertions in plan_after_update only catch replacements, not
# pending in-place updates — so the full no-op guarantee is enforced here from
# the captured JSON plan.
echo "==> Verifying the follow-up plan is a full no-op"
if ! jq -r 'select(.type == "test_plan" and ."@testrun" == "plan_after_update") | "seen"' "$test_log" | grep -q seen; then
  echo "error: no captured plan for run \"plan_after_update\" — was the run renamed?" >&2
  exit 1
fi
pending="$(jq -r 'select(.type == "test_plan" and ."@testrun" == "plan_after_update")
  | .test_plan.resource_changes[]?
  | select(.change.actions != ["no-op"])
  | "  \(.address): \(.change.actions | join(", "))"' "$test_log")"
if [[ -n "$pending" ]]; then
  echo "error: the follow-up plan still wants changes:" >&2
  echo "$pending" >&2
  exit 1
fi
echo "    all resources plan as no-op"
