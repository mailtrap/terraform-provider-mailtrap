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
exec "$tool" -chdir="$repo_root/integration" test
