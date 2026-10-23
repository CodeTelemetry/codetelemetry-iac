terraform {
  required_version = ">= 1.5"
  required_providers {
    codetelemetry = {
      source = "ElliotGardiner/codetelemetry"
    }
  }
}

# Set CODETELEMETRY_ENDPOINT and CODETELEMETRY_TOKEN in the environment.
provider "codetelemetry" {}

resource "codetelemetry_alarm_rules" "main" {
  rules_json = file("${path.module}/rules.json")
}

resource "codetelemetry_enrichment_config" "main" {
  mint_floor            = 0.6
  merge_threshold       = 0.86
  consolidation_cadence = "nightly"
  budget_pct            = 0.25
}
