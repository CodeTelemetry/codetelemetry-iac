---
page_title: "codetelemetry_alarm_rules Resource - CodeTelemetry"
description: |-
  Owns the entire workspace alarm rule list.
---

# codetelemetry_alarm_rules (Resource)

Owns the **entire** alarm rule list of the authenticated workspace, including
recommended, custom TQL, and legacy rules. Use exactly one resource per
workspace. Requires a management token; workspace contexts created for ingest
cannot manage alarm rules.

~> **Destroy clears all alarm rules.** To stop managing rules while keeping them,
remove the resource from state instead, and consider
`lifecycle { prevent_destroy = true }`.

## Example Usage

```terraform
resource "codetelemetry_alarm_rules" "main" {
  rules_json = file("${path.module}/rules.json")
}
```

`rules_json` is a JSON array of complete AlertRule objects. Include every
defaulted field (`cooldownMinutes`, `severity`, `evaluateEveryMinutes`,
`notifyOnRecovery`, `origin`) so server defaults do not change the planned value.
See [examples/rules.json](https://github.com/CodeTelemetry/terraform-provider-codetelemetry/blob/main/examples/rules.json).
JSON formatting and key order are ignored.

Updates and destroy send the revision in state. If the rules changed after the
last refresh, the service returns a conflict instead of overwriting the edit;
refresh and plan again.

## Schema

### Required

- `rules_json` (String, Sensitive) JSON array of complete AlertRule objects (maximum 60 rules / 300,000 bytes). Sensitive because webhook URLs can contain secrets.

### Read-Only

- `id` (String) Always `workspace`.
- `revision` (Number) Server revision used to reject concurrent edits.
- `workspace_binding` (String) Fingerprint of the service origin and workspace identity.

## Import

```shell
terraform import codetelemetry_alarm_rules.main workspace
```

The import ID is literally `workspace`; credentials identify the workspace.
Creating this resource refuses an existing nonempty rule list, so import before
adopting existing rules.
