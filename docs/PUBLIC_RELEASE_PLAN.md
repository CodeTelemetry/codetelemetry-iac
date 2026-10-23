# Planned public CodeTelemetry repository

The intended public home is a **fresh repository in the CodeTelemetry GitHub
organization**, seeded from the squashed, hardened source snapshot. The current
ElliotGardiner repository remains private. Do not transfer it for publication:
GitHub-owned pull-request refs and cached commits can retain its earlier history
even after branch history is replaced. Do not mirror private refs, issues, or
pull requests into the new repository. Verify the fresh repository's history and
content before making it public.

The project uses [Mozilla Public License 2.0](../LICENSE) (SPDX: `MPL-2.0`).
See the [publication review](PUBLICATION_REVIEW.md)
for the technical assessment and remaining release work.

## Before organization setup and publication

- Confirm the organization repository name and Terraform Registry namespace.
  Terraform Registry requires a `terraform-provider-{NAME}` repository; decide
  whether the Terraform provider needs a separate release repository.
- Retain MPL-2.0 and audit third-party notices in distributed binaries and SDKs.
  Scan the exact source snapshot and release artifacts for secrets and internal data.
- Configure independent Codex reviews, required CI matrix checks and publication
  security checks, pull-request protection, and stale review dismissal in the new
  repository. Enable and verify private vulnerability reporting there.
- Update Go module/import paths, provider addresses, examples, SDK generation,
  documentation links, and badges together. Choose a Pulumi/npm namespace owned
  by CodeTelemetry; the current local `@pulumi/codetelemetry` name does not grant
  rights to publish to Pulumi's scope.
- Keep ordinary PR validation independent of cloud tokens and private source.
  Authenticated builds must run only from trusted `main`, with narrowly scoped
  secrets configured in the CodeTelemetry organization.

## Planned build on updates to main

This build is planned for the CodeTelemetry organization; the current private
repository only runs fixture-based validation and security scanning. Signing and
live-service credentials have not been configured, and the release build below
is not implemented yet.

Every update to `main` in the organization repository should trigger a trusted
build for that exact commit. The build must:

1. Pass the full Terraform and Pulumi validation suites and publication security
   checks before packaging. Record the exact source revision and pinned tools.
2. Build versioned provider artifacts for the supported OS/architecture matrix,
   package the selected Pulumi SDKs, include license notices, and generate signed
   checksums using the organization's configured release identity. Verify the
   signatures and checksums before consuming the artifacts.
3. Test fresh Terraform and Pulumi consumer installations using those built
   artifacts and final package identities, rather than development overrides.
4. Run an authenticated smoke test in a dedicated non-production workspace using
   a short-lived credential. Test create/read/update/import/destroy and isolate
   each run. Destroy clears alarms and resets enrichment, so never use customer
   or shared production configuration. Always clean up, including on failure;
   fail the build if cleanup fails. Redact tokens and omit state/plans from logs
   and public artifacts.
5. Promote a public release only after all checks pass and the version/release
   policy is satisfied. A push to `main` triggers verification; it must not publish
   a new registry or npm version implicitly. Preserve artifacts and provenance
   needed to trace the supported release to its source commit.

Use narrowly scoped signing and smoke-test credentials, with GitHub environments
or workload identity where supported. Do not expose them to fork PRs or execute
untrusted PR artifacts in the privileged build. Define build timeouts, smoke-test
cleanup limits, supported API versions, and the response to an API compatibility
failure before enabling the build.

## Public launch

Create and seed the new organization repository only when requested. Verify it
contains no pre-snapshot history or private pull-request refs. Recheck branch
protection, Actions permissions, security reporting, and package publishing
access in that repository. Announce an installable release after the trusted
build and consumer tests pass against the final artifacts and identities.
