# Validate the provider

Run the complete repository validation from the repository root:

```bash
bash scripts/validate.sh
```

Requires Go 1.26+ and Terraform 1.13.5. The command stops with a nonzero exit
status on failure. No live CodeTelemetry, AWS, or Terraform Cloud credentials are
needed. Downloading tools and dependencies requires network access.

This checks Go formatting and dependency consistency, runs `go vet`, checks the
Terraform examples, and runs every Go test with `TF_ACC=1`, the race detector,
a fresh test run, and a five-minute timeout. It builds the provider afterward.
The acceptance tests drive actual Terraform through create, refresh, updates,
imports, drift detection/repair, and destroy against a local HTTP test server.
HTTP unit tests cover request bounds, authentication, cancellation, redirects,
malformed responses, and revision-conflict rejection.

Plain `go test ./...` without `TF_ACC=1` skips Terraform acceptance tests and is
not equivalent to this validation command.

## Continuous integration

Validation runs in each GitHub Actions matrix job: Go 1.26.11 / Node 22 and
Go 1.27.4 / Node 24. Node runs the release-script tests. All pull requests, every
push to `main`, manual dispatches, and merge-queue groups trigger validation
without path filters. A push to `main` checks that pushed commit; pull requests
check the merged result with the current base branch. The workflow verifies that
the checkout matches the triggering SHA, records that revision in the job
summary, lints itself with `actionlint`, and checks the validation entrypoint
with ShellCheck. `main` protection requires both matrix checks and Publication
security.

The security job scans fetched Git history using Gitleaks, checks reachable Go
vulnerabilities in production and test code using govulncheck, and audits the
locked npm dependency graph of the CI authentication helper. The script tests
also check artifact admission, credential redaction and SRP authentication
failure handling. Scanner findings and network errors fail the job; there is no
silent skip.

Tool overrides are supported for local installations outside PATH (use absolute
paths): `GO_BINARY` and `TF_ACC_TERRAFORM_PATH`.

## Limits of these tests

The lifecycle suite exercises actual Terraform and the provider code, but the
HTTP servers are test fixtures. They do not start the live CodeTelemetry
application or track changes in its private repository. The main-only
authenticated smoke test covers the live service; see below.

## Signed candidates and live verification

On main, separate jobs build and sign the exact source revision, independently
verify its identity and checksums, and install the Terraform archive into a
clean consumer through a filesystem mirror. A main-only job obtains a fresh ID
token for an isolated test account and runs Terraform
create/update/import/no-drift/destroy. Cleanup must restore empty alarms and
default enrichment. These checks exercise the live CodeTelemetry service in
addition to the local fixture suite.

See [release setup](RELEASING.md) for credentials, signature verification and
Terraform Registry releases. Ordinary PR validation uses local fixtures and
requires no live credentials.
