# Validate Terraform and Pulumi

Run the complete repository validation from the repository root:

```bash
bash scripts/validate.sh
```

Requires Go 1.26+, Node.js 22+, npm, Terraform 1.13.5, and Pulumi 3.268.0. The command
runs **both** integrations and stops with a nonzero exit status on failure. No
live CodeTelemetry, AWS, Terraform Cloud, or Pulumi Cloud credentials are needed.
Downloading tools, dependencies, and the bridge plugin requires network access.

## Terraform provider

```bash
bash scripts/validate.sh --terraform
```

This checks Go formatting and dependency consistency, runs `go vet`, checks the
Terraform examples, and runs every Go test with `TF_ACC=1`, the race detector,
a fresh test run, and a five-minute timeout. It builds the provider afterward.
The acceptance tests drive actual Terraform through create, refresh, updates,
imports, drift detection/repair, and destroy against a local HTTP test server.
HTTP unit tests cover request bounds, authentication, cancellation, redirects,
malformed responses, and revision-conflict rejection.

Plain `go test ./...` without `TF_ACC=1` skips Terraform acceptance tests and is
not equivalent to this validation command.

`--terraform` requires Go and Terraform; Node and Pulumi are not needed for this suite.

## Pulumi integration

```bash
bash scripts/validate.sh --pulumi
```

This builds the shared provider, generates the TypeScript SDK with the pinned
Terraform bridge, installs dependencies using `npm ci`, typechecks the program,
and drives the real Pulumi CLI through preview, create, updates, no-op plans,
revision conflicts, drift refresh/repair, imports, and destroy. It verifies
that preview and refresh do not write configuration and that tokens and webhook
credentials are encrypted in state. HTTP and state backends are temporary and
local. Setup must leave the committed package metadata and Pulumi declaration
unchanged. The `.npmrc` keeps generated local SDK installation consistent between
npm major versions.

`--pulumi` requires Go, Node, npm, and Pulumi; the Terraform CLI is not needed.

The Pulumi suite builds its provider binary itself; a Terraform-only test pass
does not replace running this suite.

## Continuous integration

Both suites run as separately named steps in each GitHub Actions matrix job:
Go 1.26.10 / Node 22 and Go 1.27.3 / Node 24. All pull requests, every push to `main`,
manual dispatches, and merge-queue groups trigger validation without path filters.
A push to `main` checks that pushed commit; pull requests check the merged result
with the current base branch. The workflow verifies that the checkout matches the triggering SHA, records that
revision in the job summary, lints itself with `actionlint`, and checks both
validation entrypoints with ShellCheck.
Both matrix checks and Publication security are required by `main` protection.
The security job scans fetched Git history using Gitleaks, checks reachable Go
vulnerabilities in production and test code using govulncheck, and audits the locked npm dependency graph.
Scanner findings and network errors fail the job; there is no silent skip.

Tool overrides are supported for local installations outside PATH (use absolute
paths): `GO_BINARY`, `PULUMI_BINARY`, and `TF_ACC_TERRAFORM_PATH`.
The old `terraform-provider-codetelemetry/scripts/validate.sh` path forwards to
the repository-level command for compatibility.

## Limits of these tests

The lifecycle suites exercise the actual infrastructure tools and provider code,
but the HTTP servers are test fixtures. They do not currently start the live
CodeTelemetry application, track changes in its private repository, or prove
registry installation. Application compatibility and consumer distribution need
separate validation. See the [public release plan](PUBLIC_RELEASE_PLAN.md).

Release artifact integrity and authenticated checks are described in [release setup](RELEASING.md). Ordinary PR validation uses local fixtures and requires no live credentials.
