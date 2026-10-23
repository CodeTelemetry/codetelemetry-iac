# Public CodeTelemetry release plan

The fresh private repository is `CodeTelemetry/terraform-provider-codetelemetry`.
It starts from the hardened source snapshot; earlier private GitHub pull-request
refs were not imported. It uses MPL-2.0. The Go module and provider source address
now use CodeTelemetry, and generated local npm metadata uses
`@codetelemetry/pulumi-codetelemetry` (publishing rights still need verification).

[Release setup and verification](RELEASING.md) describes the implemented main
candidate build, signing and consumer checks, the main-only authenticated smoke
job, and the exact environment configuration. Until live credentials are
configured and that smoke passes, a signed candidate is not cleared for release.

Remaining launch work:

- Choose the dedicated non-production smoke principal and a maintained credential
  acquisition/rotation mechanism. Run both authenticated lifecycles successfully.
- Register the final Terraform Registry namespace and configure its GPG signing
  key and detached checksum signature. Cosign does not replace this requirement.
- Prove Registry installation, choose a portable/Registry-backed Pulumi bridge,
  and verify npm scope rights before promoting a stable package.
- Recheck private vulnerability reporting, organization independent review
  integration, required checks and final release artifact notices.
- Make the repository public and announce a release only when explicitly requested
  and the launch checks pass. No main build publishes a package automatically.

GitHub currently blocks branch protection for this private organization repository
under its plan. Required checks and admin enforcement must be activated after a
plan upgrade or approved public visibility; they are not currently enforced.
