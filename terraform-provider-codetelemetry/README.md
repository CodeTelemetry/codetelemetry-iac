# Code Telemetry Terraform provider

Manage the authenticated workspace's application alarms and enrichment settings.
This is a local, unpublished provider built with the Terraform Plugin Framework.
It does not create AWS CloudWatch alarms or deploy the Code Telemetry service.
For the same resources in TypeScript, see the [Pulumi integration](../pulumi-codetelemetry/README.md), which bridges this provider without duplicating its behavior.

Resources:

- `codetelemetry_alarm_rules`: owns the **entire** `/api/alert-rules` list, including
  recommended, custom TQL, and legacy rules. `rules_json` is a JSON array using the
  service's existing AlertRule schema. Limit: 60 rules and 300,000 bytes. The API
  validates queries, privacy settings, channels, and rule limits before saving.
- `codetelemetry_enrichment_config`: owns the four existing classifier settings at
  `/api/enrichment-config`: `mint_floor`, `merge_threshold`, `consolidation_cadence`,
  and `budget_pct`. Fractions range from 0 to 1; cadence is `nightly`, `weekly`, or
  `manual`. `config_revision` is read-only. This resource manages the current
  enrichment knobs; custom grader definitions do not have an API in this repository.

Use one resource of each kind per workspace and one Terraform state owner. An
alias with another token can manage another workspace. The Cognito subject selects
scope; do not switch an existing state's credentials to a different subject.
The provider uses the same API and validation paths as the web application and
makes no storage or evaluator changes.

## Build and use locally

Requires Go 1.26+ and Terraform 1.5+. From this directory:

```bash
go build -o "$PWD/bin/terraform-provider-codetelemetry" .
```

Create a Terraform CLI configuration, replacing the absolute directory below with
the `bin` directory you just built:

```hcl
provider_installation {
  dev_overrides {
    "CodeTelemetry/codetelemetry" = "/absolute/path/to/terraform-provider-codetelemetry/bin"
  }
  direct {}
}
```

Set `TF_CLI_CONFIG_FILE` to that file. Development overrides allow `plan` and
`apply` without installing from the registry; skip `terraform init` for the
standalone example. Copy `examples/main.tf` and `examples/rules.json` into a working
directory, then:

```bash
export CODETELEMETRY_ENDPOINT=https://your-code-telemetry-host
# Supply a current Cognito ID token through your shell/CI secret manager.
# CODETELEMETRY_TOKEN must be set before running Terraform.
terraform plan
terraform apply
```

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

## Validation

For full validation of both integrations, see [Validate Terraform and Pulumi](../docs/VALIDATION.md).

```bash
go test ./...
go vet ./...
TF_ACC=1 go test ./...  # needs Terraform on PATH; only a local test HTTP server
terraform fmt -check examples
```

Set `TF_ACC_TERRAFORM_PATH` to an absolute Terraform binary if necessary. The
acceptance test runs actual Terraform create, refresh, update, drift repair,
import, and destroy through protocol v6. Unit tests cover conflict rejection,
request cancellation, response bounds, authentication, redirection, and malformed
responses. No test contacts a live Code Telemetry deployment.

The self-contained [API contract](../docs/API_CONTRACT.md) describes the service boundary used by both integrations.

## Workspace identity and upgrades

Resources store a SHA-256 fingerprint of the service origin and the Cognito ID
token's issuer and subject. Refresh, update, and destroy reject a different
workspace before making an HTTP request. Token renewal within the same workspace
is supported. This guard is not JWT authentication; the service verifies tokens.

Legacy state without `workspace_binding` binds to the configured credentials on
its first refresh. Verify the original endpoint and workspace before upgrading
old state. To manage a different workspace, use separate state/a stack or import
it intentionally. Do not remove the binding to bypass a mismatch.

Security dependency fixes raise the local build minimum to Go 1.26; CI uses patched
Go 1.26.10 and 1.27.3 toolchains. See [SECURITY.md](../SECURITY.md).
