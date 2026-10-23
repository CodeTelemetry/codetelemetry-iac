# Code Telemetry with Pulumi

Manage Code Telemetry workspace alarms and enrichment settings from TypeScript.
This project generates a local `@pulumi/codetelemetry` SDK using Pulumi's
[Terraform provider bridge](https://www.pulumi.com/registry/packages/terraform-provider/installation-configuration/)
and the sibling [Terraform provider](../terraform-provider-codetelemetry/README.md).
Both tools run the same API client, validation, and CRUD implementation, including
alarm revision guards.

Resources exposed by the generated SDK:

- `codetelemetry.AlarmRules`: owns the entire workspace alarm list through
  `rulesJson`. Maximum 60 rules and 300,000 bytes. Use one resource per workspace,
  because the API replaces the whole list.
- `codetelemetry.EnrichmentConfig`: sets `mintFloor`, `mergeThreshold`,
  `consolidationCadence`, and `budgetPct`. These are the existing classifier settings;
  custom grader definitions have no API in this repository.
- `codetelemetry.Provider`: accepts `endpoint` and secret `token`, falling back to
  `CODETELEMETRY_ENDPOINT` and `CODETELEMETRY_TOKEN` in the environment.

## Setup and deployment

Requires Node.js 22+, npm, Go 1.26+, and Pulumi CLI 3.268.0+. Terraform CLI is not
required. From this directory:

```bash
npm run setup
npm run typecheck
```

Setup builds the sibling Terraform provider into `bin/`, generates the SDK into
`sdks/`, and installs Node dependencies with `npm ci` using the committed lockfile. The bridge is pinned to 1.5.0 and Pulumi's
Node SDK to 3.268.0. Generated SDKs, binaries, and dependencies are ignored by Git.
Run setup after cloning, moving the project, changing hosts, or modifying the
shared provider. SDK generation uses a temporary local backend and requires no
Pulumi Cloud account. `GO_BINARY` and `PULUMI_BINARY` can select binaries outside
PATH. Keep the `bin/` directory available when running Pulumi.

Choose your normal Pulumi backend, or use a local one:

```bash
pulumi login --local
pulumi stack init dev
export CODETELEMETRY_ENDPOINT=https://your-code-telemetry-host
# Supply a current Cognito ID token via your shell/CI secret manager as
# CODETELEMETRY_TOKEN, or use `pulumi config set --secret token`.
pulumi preview
pulumi up
```

The service accepts Cognito **ID tokens**, not ingest keys or access tokens. Tokens
expire; supply a current token for later operations. The provider has no token
refresh or service-account flow. HTTPS is required except on loopback development.
Credentials select the workspace; do not change an existing stack to a different
Cognito subject.

The sample in [index.ts](index.ts) reads the complete alarm JSON from the shared
Terraform example. To use your own configuration:

```bash
pulumi config set rulesPath /absolute/path/to/rules.json
pulumi config set budgetPct 0.4
pulumi config set consolidationCadence weekly
pulumi preview
pulumi up
```

Relative `rulesPath` values are relative to the Pulumi project. Numeric settings
`mintFloor`, `mergeThreshold`, and `budgetPct` accept fractions from 0 to 1; cadence
accepts `nightly`, `weekly`, or `manual`. Omitting a setting uses service defaults
of 0.6, 0.86, 0.25, and `nightly`. Zero fractions are preserved. The alarm JSON uses
the service's AlertRule schema, with explicit defaulted fields; see the
[shared example](../terraform-provider-codetelemetry/examples/rules.json).

The generated SDK can also be used directly:

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as codetelemetry from "@pulumi/codetelemetry";

const provider = new codetelemetry.Provider("workspace", {
  endpoint: "https://your-code-telemetry-host",
  token: new pulumi.Config().requireSecret("token"),
});
const enrichment = new codetelemetry.EnrichmentConfig("enrichment", {
  mintFloor: 0.6,
  mergeThreshold: 0.86,
  consolidationCadence: "nightly",
  budgetPct: 0.25,
}, { provider });
```

For another project, first generate its SDK with `pulumi package add
terraform-provider@1.5.0 /absolute/path/to/bin/terraform-provider-codetelemetry`.
The SDK needs the binary at that path at runtime. This is a local, unpublished
integration; no package registry release or hosting is required.

## Adoption and ownership

Use exactly one state owner per workspace. Do not manage the same settings from
both Terraform and Pulumi, or from multiple stacks. To adopt existing settings
into the example, first configure desired values matching the current API values:

```bash
pulumi config set importAlarms true
pulumi config set importEnrichment true
pulumi preview
pulumi up
pulumi config rm importAlarms
pulumi config rm importEnrichment
```

These set Pulumi's resource `import` option to the literal ID `workspace` for each
resource. The token selects the workspace. Imports require matching configuration;
inspect differences before proceeding. Alarm creation refuses an existing nonempty
list without adoption. Enrichment is an always-present service singleton: creating
its resource sets all four settings.

For UI edits or other drift, run `pulumi refresh`, inspect `pulumi preview`, then
apply the desired configuration. Alarm writes and destruction carry the stored
revision; HTTP 409 means another edit won and you should refresh and preview
again. Enrichment's API currently uses last-write-wins semantics, so avoid
concurrent writers. Preview and refresh perform no configuration writes.

**`pulumi destroy` clears all alarm rules and restores enrichment defaults.**
Telemetry and classification results remain. Set `pulumi config set protect true`
and run `pulumi up` to protect these resources from deletion. To relinquish
management while preserving remote settings, use Pulumi's `retainOnDelete`
resource option or remove the resources from state.

The SDK marks tokens and rule JSON as secret, and the example explicitly uses
`pulumi.secret` for alarm JSON. Webhook credentials remain encrypted in state and
hidden from normal CLI output; secure your backend and its encryption keys.

## Validation

For full validation of both integrations, see [Validate Terraform and Pulumi](../docs/VALIDATION.md).

After setup:

```bash
npm ci
npm test
```

The test runs the real Pulumi CLI, bridge, and shared provider against a bounded
local HTTP server and temporary local state backend. It checks preview, create,
updates, no-op plans, HTTP 409 conflicts, drift refresh/repair, both imports,
destroy behavior, and encrypted secret state. It ignores the caller's service
endpoint, token, backend, and stack. It does not contact a live Code Telemetry
service or create a cloud stack. Dependencies and the bridge plugin must already
be installed by setup.

Shared HTTP limits, validation, and Terraform lifecycle tests live in the sibling
provider. Run those as described in its README when changing shared behavior.

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
