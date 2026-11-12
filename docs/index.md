---
page_title: "CodeTelemetry Provider"
description: |-
  Manage CodeTelemetry workspace alarm rules, enrichment settings, and Grader draft checks.
---

# CodeTelemetry Provider

Manage [CodeTelemetry](https://codetelemetry.com) workspace configuration through
its authenticated API: alarm rules, enrichment settings, and Grader draft checks.
The provider configures an existing CodeTelemetry service. It does not create AWS
CloudWatch alarms or deploy the service.

The provider is experimental. Pin a version and commit your `.terraform.lock.hcl`.

## Example Usage

```terraform
terraform {
  required_providers {
    codetelemetry = {
      source  = "CodeTelemetry/codetelemetry"
      version = "~> 0.1"
    }
  }
}

# Uses the repository's workspace binding or CODETELEMETRY_WORKSPACE.
provider "codetelemetry" {}

resource "codetelemetry_grader_config" "main" {
  checks_json = file("${path.module}/grader-checks.json")
}
```

## Authentication

Use one of two methods. Never put credentials in Terraform files.

**Named workspace context (recommended).** Create a context once with
`npx -y @codetelemetry/connect bootstrap <name>` or the CodeTelemetry MCP
`bootstrap_workspace` tool. The provider then finds the repository's
`.codetelemetry/workspace.json` binding, or uses the `workspace` argument or
`CODETELEMETRY_WORKSPACE`. Credentials stay in private per-user storage and never
enter Terraform state. Contexts created for ingest support
`codetelemetry_grader_config` only; alarm rules need a management token.

**Management token.** Set `CODETELEMETRY_ENDPOINT` and a current Cognito **ID
token** in `CODETELEMETRY_TOKEN`. Ingest keys and access tokens are not accepted.
Tokens expire and the provider does not refresh them. Set `organization_id` or
`CODETELEMETRY_ORGANIZATION_ID` to manage an organization instead of your
personal scope.

## One state owner per workspace

Each resource is a singleton for the authenticated workspace. Use exactly one
Terraform state to own a workspace's settings, and avoid concurrent edits from
the CodeTelemetry UI. Every resource records a `workspace_binding` fingerprint
of the service origin and workspace identity, and refuses to refresh, update, or
destroy with credentials for a different workspace.

Terraform state and saved plans contain alarm rules and check definitions in
plaintext even though they are marked sensitive. Protect your backend.

## Schema

### Optional

- `endpoint` (String) Service origin. Falls back to `CODETELEMETRY_ENDPOINT`. HTTPS is required except on loopback.
- `organization_id` (String) Explicit organization ID, or `CODETELEMETRY_ORGANIZATION_ID`. Never falls back to personal scope.
- `token` (String, Sensitive) Cognito ID token, or `CODETELEMETRY_TOKEN`. Ingest keys are not accepted.
- `workspace` (String) Named Connect/MCP workspace context, or `CODETELEMETRY_WORKSPACE`. Repository bindings are discovered automatically. Credentials remain outside Terraform state.
