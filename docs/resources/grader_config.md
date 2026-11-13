---
page_title: "codetelemetry_grader_config Resource - CodeTelemetry"
description: |-
  Manages listed check IDs in the workspace Grader draft.
---

# codetelemetry_grader_config (Resource)

Manages the listed check IDs in the workspace's Grader **draft**. Unrelated draft
checks and the workspace's requested features are preserved. Applying saves a
draft only: publish it separately on the CodeTelemetry Grader page or with the
MCP `publish_grader_config` tool. Destroy removes only the managed draft checks
and leaves published policy versions intact.

Use one state owner per workspace, and do not manage the same check IDs from two
states.

## Example Usage

```terraform
resource "codetelemetry_grader_config" "main" {
  checks_json = file("${path.module}/grader-checks.json")
}
```

`checks_json` is a JSON array of complete ValidationCheck objects. See
[examples/grader-checks.json](https://github.com/CodeTelemetry/terraform-provider-codetelemetry/blob/main/examples/grader-checks.json).
The provider validates the shape of each check during `plan`; the service
validates matcher semantics during `apply`, so a clean plan does not guarantee
that every matcher is accepted. JSON formatting and key order are ignored.

## Checks required before push

A check with `"beforePush": true` must also pass in the final edit-to-push window.
The service accepts that flag only when `matcherRule.include` names the commands,
in one of two forms:

- one `exactFingerprint` condition with operator `equals` or `in`, as saved by the
  Grader page or the MCP discovery tools; or
- `all` of exactly `source` `equals` `shell`, `executable` `equals`/`in`, and
  `packageScript` `equals`/`in`:

```json
{
  "include": {
    "all": [
      { "field": "source", "operator": "equals", "value": "shell" },
      { "field": "executable", "operator": "in", "value": ["npm", "pnpm", "yarn", "bun"] },
      { "field": "packageScript", "operator": "equals", "value": "test" }
    ]
  },
  "exclude": { "field": "normalizedArguments", "operator": "contains", "value": ["--help", "--watch"] }
}
```

Either form may add one `repositoryRelativeWorkingDirectory` `equals` condition to
scope it to a package directory. `exclude` may be omitted, an `exactFingerprint`
condition, or (for the package-script form) `normalizedArguments` `contains`. The
package-script form leaves arguments after the script open, so list any that must
not count. A before-push check takes no `detectorRule`, `applicabilityRule` or
`evidenceRequirement`, and its `satisfactionMode` cannot be `observed`.

Broader rules, such as a bare script name or an executable alone, are valid for
scoring but are refused with `beforePush`. The provider reports that refusal as
HTTP 400 during `apply` without the service's message.

## Schema

### Required

- `checks_json` (String, Sensitive) JSON array of complete ValidationCheck objects: at most 30 checks / 48 KiB. Only these IDs are managed.

### Read-Only

- `id` (String) Always `workspace`.
- `revision` (String) Opaque server revision of the complete draft. Concurrent edits require a refresh and a new plan.
- `workspace_binding` (String) Fingerprint of the service origin and authenticated workspace identity.

## Import

```shell
terraform import codetelemetry_grader_config.main workspace
```

The import ID is literally `workspace`; credentials identify the workspace.
Importing adopts every check visible in the draft, so reconcile unrelated checks
with your configuration before applying.
