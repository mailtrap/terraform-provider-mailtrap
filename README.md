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

Acceptance tests are gated behind `TF_ACC` and run against an in-memory mock, so no live credentials are required:

```bash
TF_ACC=1 go test ./...
```

To exercise the provider locally without publishing, use a [development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) that points `mailtrap/mailtrap` at your `go install` output.

## Contributing

Bug reports and pull requests are welcome on GitHub. Please follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Released under the [MIT License](LICENSE).
