# Terraform Provider for Mailtrap

[![CI](https://github.com/mailtrap/terraform-provider-mailtrap/actions/workflows/ci.yml/badge.svg)](https://github.com/mailtrap/terraform-provider-mailtrap/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Manage [Mailtrap](https://mailtrap.io) resources as code with Terraform or OpenTofu. Built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework) and the [`mailtrap-go`](https://github.com/mailtrap/mailtrap-go) client.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0 or [OpenTofu](https://opentofu.org) >= 1.6
- A Mailtrap API token — create one under [API Tokens](https://mailtrap.io/api-tokens)
- [Go](https://go.dev/dl/) >= 1.26 (to build from source)

## Usage

```terraform
terraform {
  required_providers {
    mailtrap = {
      source = "mailtrap/mailtrap"
    }
  }
}

provider "mailtrap" {
  api_token = var.mailtrap_api_token # or set the MAILTRAP_API_TOKEN env var
}
```

The API token may be supplied via the `api_token` argument or the `MAILTRAP_API_TOKEN` environment variable. Keep it out of version control — prefer the environment variable or a secrets manager.

Per-resource and data-source documentation is published on the Terraform Registry.

## Development

```bash
go build ./...   # build
go vet ./...     # vet
go test ./...    # tests
```

To exercise the provider locally without publishing, use a [development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) that points `mailtrap/mailtrap` at your `go install` output.

### Integration tests

The `integration/` module wires every resource and data source together and is exercised end-to-end against the real Mailtrap API with the [native test framework](https://developer.hashicorp.com/terraform/language/tests) (`terraform test` / `tofu test`). CI runs the suite with both Terraform and OpenTofu against a dedicated test account — on pushes to `main` and same-repo pull requests (after the fast checks pass), plus nightly and on manual dispatch; see `.github/workflows/integration.yml` and the `integration` job in `.github/workflows/ci.yml`.

To run it locally you need a Mailtrap API token with admin access to a **test** account — the suite creates and destroys real resources:

```bash
MAILTRAP_API_TOKEN=... ./scripts/integration-test.sh terraform
MAILTRAP_API_TOKEN=... ./scripts/integration-test.sh tofu
```

The script builds the provider and injects it via a `dev_overrides` CLI configuration, so no `terraform init` or registry access is needed. Resource names are prefixed with `var.name_prefix` (default `tftest`); set `TF_VAR_name_prefix` to isolate runs. The framework destroys everything it created, even on failure — and before each run the script sweeps any orphaned `tftest*` resources left by an interrupted run (via `scripts/integration-cleanup.sh`, which requires `jq` and can also be run standalone), so runs are idempotent. Because the sweep deletes by name prefix and the test account's plan allows a single project, runs against the same account must not overlap.

### Documentation

The `docs/` directory is generated — do not edit it by hand. Pages are rendered by [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs) from the provider schema (attribute descriptions in the Go source) and the configuration examples under `examples/`. After changing either, regenerate and commit the result:

```bash
go generate ./...
```

CI fails if `docs/` is out of date.

## Contributing

Bug reports and pull requests are welcome on GitHub. Please follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Released under the [MIT License](LICENSE).
