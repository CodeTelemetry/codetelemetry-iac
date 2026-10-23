import * as pulumi from "@pulumi/pulumi";
import * as codetelemetry from "@codetelemetry/pulumi-codetelemetry";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const config = new pulumi.Config();
const provider = new codetelemetry.Provider("workspace", {
  endpoint: config.get("endpoint"),
  token: config.getSecret("token"),
});

// Share the complete AlertRule example with Terraform. Set rulesPath to a JSON
// file of your own; its path is relative to this Pulumi project.
const rulesPath = config.get("rulesPath") ?? "../terraform-provider-codetelemetry/examples/rules.json";
const rulesJson = JSON.stringify(JSON.parse(readFileSync(resolve(rulesPath), "utf8")));
const protect = config.getBoolean("protect") ?? false;

const alarms = new codetelemetry.AlarmRules("alarms", {
  rulesJson: pulumi.secret(rulesJson),
}, {
  provider,
  protect,
  import: config.getBoolean("importAlarms") ? "workspace" : undefined,
});

const enrichment = new codetelemetry.EnrichmentConfig("enrichment", {
  mintFloor: config.getNumber("mintFloor") ?? 0.6,
  mergeThreshold: config.getNumber("mergeThreshold") ?? 0.86,
  consolidationCadence: config.get("consolidationCadence") ?? "nightly",
  budgetPct: config.getNumber("budgetPct") ?? 0.25,
}, {
  provider,
  protect,
  import: config.getBoolean("importEnrichment") ? "workspace" : undefined,
});

export const alarmRevision = alarms.revision;
export const enrichmentRevision = enrichment.configRevision;
