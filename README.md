# CodeTelemetry Terraform provider

Manage CodeTelemetry workspace alarms, enrichment settings and selected Grader
draft checks with Terraform. The provider configures an existing CodeTelemetry
service through its authenticated API. It does not create AWS CloudWatch alarms
or deploy the service.

The provider is experimental: `0.x` releases may change behavior between minor
versions, so pin a version and commit your `.terraform.lock.hcl`.

## Install

```hcl
terraform {
  required_providers {
    codetelemetry = {
      source  = "CodeTelemetry/codetelemetry"
      version = "~> 0.1"
    }
  }
}

provider "codetelemetry" {}
```

Run `terraform init`, then `terraform plan` and `terraform apply`. Reference
documentation is on the [Terraform Registry](https://registry.terraform.io/providers/CodeTelemetry/codetelemetry/latest/docs)
and in [docs/](docs/index.md); complete configurations are in [examples/](examples).

Resources:

- `codetelemetry_alarm_rules`: owns the **entire** `/api/alert-rules` list, including
  recommended, custom TQL, and legacy rules. `rules_json` is a JSON array using the
  service's existing AlertRule schema. Limit: 60 rules and 300,000 bytes. The API
  validates queries, privacy settings, channels, and rule limits before saving.
- `codetelemetry_enrichment_config`: owns the four existing classifier settings at
  `/api/enrichment-config`: `mint_floor`, `merge_threshold`, `consolidation_cadence`,
  and `budget_pct`. Fractions range from 0 to 1; cadence is `nightly`, `weekly`, or
  `manual`. `config_revision` is read-only.
- `codetelemetry_grader_config`: manages the listed check IDs in the workspace
  Grader draft and preserves unrelated checks. It never publishes a policy.

Use one resource of each kind per workspace and one Terraform state owner. An
alias with another token can manage another workspace. The Cognito subject selects
scope; do not switch an existing state's credentials to a different subject.
The provider uses the same API and validation paths as the web application and
makes no storage or evaluator changes.

## Authentication

Supply `CODETELEMETRY_ENDPOINT` and a current `CODETELEMETRY_TOKEN` through your
shell or secret manager, or use a named workspace context as described below.

The provider also accepts `endpoint` and sensitive `token` arguments. Prefer the
environment for credentials. The current API requires a **Cognito ID token**, not
an ingest key or access token. Tokens expire; supply a fresh one for later runs.
There is no service-account credential or token-refresh flow in this provider.
HTTPS is required except for loopback development. Requests have a 30-second
HTTP timeout, a 350,000-byte request bound, and a 2 MiB response bound. Redirects
and automatic retries are disabled. Remote response bodies are omitted from
errors to avoid exposing secrets.

`rules_json` uses complete rule objects, including explicit `cooldownMinutes`,
`severity`, `evaluateEveryMinutes`, `notifyOnRecovery`, and `origin`, to avoid
server default insertion changing Terraform's planned value. See the complete
example. IDs are stable, caller-chosen rule identifiers. JSON formatting and key
order differences are ignored. `revision` is the server's read-only alarm revision.
Webhook URLs may hold credentials: the entire JSON attribute is marked sensitive,
but Terraform still stores it in state. Protect your backend and saved plans.

## Named organization contexts

Connect/MCP and this provider share the version-1 workspace format. Create a
context with `@codetelemetry/connect bootstrap <name>` or MCP's
`bootstrap_workspace`, then run Terraform inside the bound repository. The
`.codetelemetry/workspace.json` binding is nonsecret. Credentials are loaded from
`CODETELEMETRY_CONTEXT_HOME`, or the default per-user CodeTelemetry configuration
directory; they never become a provider argument or resource state attribute.

```hcl
provider "codetelemetry" {
  workspace = "acme"
}

resource "codetelemetry_grader_config" "instructions" {
  checks_json = file("grader-checks.json")
}
```

With no explicit `workspace`, the provider discovers the repository binding or
uses `CODETELEMETRY_WORKSPACE`. It verifies the account, organization and billing
scope live. A funded ingest budget is not required to configure a policy. Context
credentials use the existing Grader agent API, preserving unrelated checks and
requested features. Destroy removes only managed IDs; publication remains an
explicit MCP operation. Do not manage the same checks from two states.

For existing management-token automation, set `organization_id` or
`CODETELEMETRY_ORGANIZATION_ID`. Requests then include the explicit organization
selection; the service authorizes current membership and permissions. Omitting
organization selection retains the personal behavior and existing state identity.
Changing organization, account or endpoint refuses existing bound state until an
intentional import. Legacy unbound state is never automatically adopted into an
organization. Ingest contexts support Grader configuration only; alarm rules need
a management token. Organization enrichment is currently unsupported and fails
before HTTP rather than changing personal settings.

A context supports credential rotation without changing its identity. Conflicting
explicit endpoint/token/organization values and an existing repository binding
fail closed. Context files are bounded to 64 KiB, checked for version, links,
ownership and private POSIX permissions. Protect Terraform state and plans because
check definitions and alert destinations may still be sensitive.

## Adoption, changes, and destruction

```bash
terraform import codetelemetry_alarm_rules.main workspace
terraform import codetelemetry_enrichment_config.main workspace
```

Both import IDs are literally `workspace`; credentials identify the workspace.
Write matching configuration after importing and review the plan. Creation of
alarm rules refuses an existing nonempty list, requiring import before adoption.
Enrichment configuration is a service singleton that always has defaults; creating
its resource sets all four settings, including any existing values.

Refresh detects edits made through the UI. Alarm updates and destruction use the
revision from Terraform state. A change after refresh returns HTTP 409 rather
than overwriting that edit; refresh and plan again. Creation uses a guarded
read-before-write as well. Enrichment writes currently lack a conditional-revision
API and use last-write-wins semantics; avoid concurrent UI/Terraform writers.

**Destroy clears all alarm rules and restores enrichment defaults** (0.6, 0.86,
`nightly`, 0.25). It retains telemetry and classification results. To stop managing
settings while preserving them, remove them from state instead, or use a Terraform
`removed` block with `destroy = false` on Terraform versions supporting it.
Consider `lifecycle { prevent_destroy = true }` for production settings.

A 404, 401, or 503 is an error, not evidence that the workspace configuration was
deleted. The enrichment API returns defaults when its store is absent locally,
but writes return 503. The provider preserves this existing behavior.

## Workspace identity and upgrades

Resources store a SHA-256 fingerprint of the service origin and the Cognito ID
token's issuer and subject. Refresh, update, and destroy reject a different
workspace before making an HTTP request. Token renewal within the same workspace
is supported. This guard is not JWT authentication; the service verifies tokens.

Legacy state without `workspace_binding` binds to the configured credentials on
its first refresh. Verify the original endpoint and workspace before upgrading
old state. To manage a different workspace, use separate state/a stack or import
it intentionally. Do not remove the binding to bypass a mismatch.

## Build from source

Requires Go 1.26+ and Terraform 1.5+. Build with a patched Go toolchain; see
[SECURITY.md](SECURITY.md).

```bash
go build -o "$PWD/bin/terraform-provider-codetelemetry" .
```

To run Terraform against that build, create a CLI configuration with a
development override and set `TF_CLI_CONFIG_FILE` to it. Development overrides
skip `terraform init`.

```hcl
provider_installation {
  dev_overrides {
    "CodeTelemetry/codetelemetry" = "/absolute/path/to/terraform-provider-codetelemetry/bin"
  }
  direct {}
}
```

## Validate

With Go 1.26+ and Terraform 1.13.5 installed:

```bash
bash scripts/validate.sh
```

This runs formatting and dependency checks, `go vet`, race tests, and acceptance
tests that drive real Terraform create, refresh, update, drift repair, import and
destroy against local HTTP fixtures. No live credentials are needed. See the
[validation guide](maintainers/VALIDATION.md) and the
[API contract](maintainers/API_CONTRACT.md).

## Releases

Every push to `main` builds six platform archives, signs the candidate manifest
with Cosign, verifies a fresh install, and runs an authenticated Terraform
lifecycle in an isolated test workspace. Pushing a `vX.Y.Z` tag on a commit that
passed that pipeline builds the Terraform Registry release and signs its
checksums with the [release GPG key](release-signing-key.asc). See
[release verification](maintainers/RELEASING.md).

## Pulumi

The Pulumi TypeScript integration that bridges this provider lives in
[CodeTelemetry/pulumi-codetelemetry](https://github.com/CodeTelemetry/pulumi-codetelemetry).

## Contributing and security

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Report
vulnerabilities privately through the repository's Security tab.

## License

[Mozilla Public License 2.0](LICENSE) (SPDX: `MPL-2.0`). Third-party dependencies
retain their own licenses.
