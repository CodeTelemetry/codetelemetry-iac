# Code Telemetry providers

Terraform and Pulumi support for Code Telemetry workspace alarms and enrichment configuration.

- [Terraform provider](terraform-provider-codetelemetry/README.md): owns the workspace alarm rule list and enrichment settings through the authenticated API.
- [Pulumi integration](pulumi-codetelemetry/README.md): generates a TypeScript SDK from the same provider implementation.
- [Validate Terraform and Pulumi](docs/VALIDATION.md): full local checks and the GitHub Actions matrix.

## Validate Terraform and Pulumi

With Go 1.26+, Node 22+, npm, Terraform 1.13.5, and Pulumi 3.268.0 installed:

```bash
bash scripts/validate.sh
```

Validation runs all provider unit and acceptance tests, the Go race detector, TypeScript checks, and Pulumi lifecycle tests against local HTTP servers. No cloud credentials are needed.

To run a single suite, use `bash scripts/validate.sh --terraform` or `bash scripts/validate.sh --pulumi`. See the [validation guide](docs/VALIDATION.md) for each suite’s coverage and setup.

The GitHub Actions workflow runs both suites on pull requests, pushes to `main`, and merge-queue groups using Go 1.26 / Node 22 and Go 1.27 / Node 24.

Use exactly one Terraform state or Pulumi stack per workspace. Destroy clears alarms and restores enrichment defaults; consult the package documentation before adoption or deployment.

The planned public home is a fresh repository in the CodeTelemetry organization; see the [public release plan](docs/PUBLIC_RELEASE_PLAN.md). The repository remains private for now. The fresh repository will start from the squashed snapshot so existing private pull-request history is not carried into the public repository.

This repository contains unpublished integrations. Registry distribution, signing, and release automation are future work. The Code Telemetry service is separate; see the self-contained [API contract](docs/API_CONTRACT.md).

See [SECURITY.md](SECURITY.md), [contributing](CONTRIBUTING.md), and the [publication review](docs/PUBLICATION_REVIEW.md) before distributing this code.

## License

This project is licensed under the [Mozilla Public License 2.0](LICENSE) (SPDX: `MPL-2.0`). Third-party dependencies retain their own licenses.
