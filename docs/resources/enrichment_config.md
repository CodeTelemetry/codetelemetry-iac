---
page_title: "codetelemetry_enrichment_config Resource - CodeTelemetry"
description: |-
  Workspace classifier and enrichment settings.
---

# codetelemetry_enrichment_config (Resource)

Manages the four classifier enrichment settings of the authenticated workspace.
Use one resource per workspace. Organization scope is not supported for this
resource.

Destroy restores the service defaults (`0.6`, `0.86`, `nightly`, `0.25`); it does
not delete classified telemetry. The enrichment API has no conditional write, so
updates are last-write-wins: avoid concurrent edits from the UI.

## Example Usage

```terraform
resource "codetelemetry_enrichment_config" "main" {
  mint_floor            = 0.6
  merge_threshold       = 0.86
  consolidation_cadence = "nightly"
  budget_pct            = 0.25
}
```

## Schema

### Required

- `budget_pct` (Number) Enrichment budget as a fraction from 0 to 1 (0.25 means 25%).
- `consolidation_cadence` (String) `nightly`, `weekly`, or `manual`.
- `merge_threshold` (Number) Near-duplicate embedding similarity, between 0 and 1.
- `mint_floor` (Number) Label reuse confidence, between 0 and 1.

### Read-Only

- `config_revision` (Number) Server-owned cache invalidation revision.
- `id` (String) Always `workspace`.
- `workspace_binding` (String) Fingerprint of the service origin and workspace identity.

## Import

```shell
terraform import codetelemetry_enrichment_config.main workspace
```

The import ID is literally `workspace`; credentials identify the workspace.
