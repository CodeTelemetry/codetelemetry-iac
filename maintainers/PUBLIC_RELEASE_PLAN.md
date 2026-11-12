# Public source and Registry release plan

`CodeTelemetry/terraform-provider-codetelemetry` is the source and distribution
repository for the Terraform provider. It was created from a hardened source
snapshot; private application history, pull requests, issues, and customer data
were not imported. The source uses MPL-2.0.

The repository was briefly named `codetelemetry-iac` while it also held the
Pulumi integration. It was renamed back because the
[Terraform Registry requires](https://developer.hashicorp.com/terraform/registry/providers/publishing)
a public GitHub repository named `terraform-provider-{NAME}`, and Pulumi moved to
[CodeTelemetry/pulumi-codetelemetry](https://github.com/CodeTelemetry/pulumi-codetelemetry).
Candidates signed under the old name use that name's workflow identity and are
not accepted by the current verifier.

## Terraform Registry distribution

The binary is `terraform-provider-codetelemetry`, the provider source address is
`CodeTelemetry/codetelemetry`, and the Go module is
`github.com/CodeTelemetry/terraform-provider-codetelemetry`. Resource names and
saved state identities do not depend on the repository name.

A release is a `vX.Y.Z` tag on a `main` commit whose candidate pipeline passed,
including the authenticated smoke test. The [release workflow](../.github/workflows/release.yml)
rebuilds that commit with the stable version, signs the checksum file with the
organization release GPG key, and publishes a GitHub release that the Registry
ingests. Reference documentation for the Registry lives in `docs/`.

One-time Registry onboarding is manual: sign in to the Terraform Registry with a
GitHub account that administers the CodeTelemetry organization, add the public
key in [release-signing-key.asc](../release-signing-key.asc) to the
`CodeTelemetry` namespace, and publish this repository as a provider. Prove a
fresh `terraform init` from the Registry before announcing a release.

## Ongoing verification

Maintain the isolated CI account and rotate its environment password as needed.
Require the live lifecycle and cleanup on every main candidate. Recheck
third-party notices and dependency advisories for the exact release artifacts.
See [release setup and verification](RELEASING.md) for signing identities,
credential boundaries, and verification commands.
