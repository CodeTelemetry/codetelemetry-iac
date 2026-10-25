# Public source and package release plan

`CodeTelemetry/codetelemetry-iac` is the shared source repository for Terraform
(HCL) and Pulumi (TypeScript). It was created from the hardened source snapshot;
private application history, pull requests, issues, and customer data were not
imported. The source uses MPL-2.0.

Source visibility and stable package distribution are separate. The source can
be built locally using either integration's README. Main builds produce signed
candidates, with fresh consumer installation and authenticated lifecycle checks
for both integrations; these builds do not publish stable packages.

## Public source controls

Before changing visibility, require a clean exact-head independent code review,
both validation matrix jobs, publication security, and a successful signed main
candidate including authenticated smoke and cleanup under the renamed repository
identity. Enable and verify main protection and private vulnerability reporting
when public visibility makes those controls available. Keep live credentials in
the main-only `release-verification` environment.

## Terraform / HCL distribution

The binary remains `terraform-provider-codetelemetry`, and the provider source
address remains `CodeTelemetry/codetelemetry`. The shared repository rename does
not change resource names or saved state identities.

The [Terraform Registry requires](https://developer.hashicorp.com/terraform/registry/providers/publishing)
a public GitHub repository named `terraform-provider-{NAME}`. Before Registry
onboarding, create a separate `CodeTelemetry/terraform-provider-codetelemetry`
distribution repository, select the exact reviewed source revision from this
monorepo, and configure organization-controlled GPG signing with detached checksum
signatures. Cosign signatures supplement that requirement. Prove a fresh Registry
installation before announcing a supported Terraform release.

## Pulumi / TypeScript distribution

The generated local SDK is `@codetelemetry/pulumi-codetelemetry`. It currently
uses Pulumi's pinned Terraform provider bridge and needs the matching executable
at runtime. Choose a portable or Registry-backed distribution, verify npm scope
publishing rights, and prove installation into a clean project before promoting
a stable npm package. Other Pulumi SDK languages are not generated or validated
by this repository.

## Ongoing verification

Maintain the isolated CI account and rotate its environment password as needed.
Require both live lifecycles and cleanup on every main candidate. Recheck
third-party notices and dependency advisories for the exact release artifacts.
See [release setup and verification](RELEASING.md) for signing identities,
credential boundaries, and verification commands.
