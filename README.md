# CodeTelemetry IaC

Manage CodeTelemetry workspace alarms and enrichment settings with **Terraform
(HCL)** or **Pulumi (TypeScript)**. This repository is the shared source for both
integrations: the Go provider implements the API behavior, and Pulumi generates a
TypeScript SDK through its Terraform provider bridge.

The project is experimental. Source builds and signed CI candidates are available;
there is no stable Terraform Registry or npm release yet.

## Choose an integration

| | Terraform / HCL | Pulumi / TypeScript |
| --- | --- | --- |
| Package | [`terraform-provider-codetelemetry`](terraform-provider-codetelemetry/README.md) | [`pulumi-codetelemetry`](pulumi-codetelemetry/README.md) |
| Alarm resource | `codetelemetry_alarm_rules` / `rules_json` | `AlarmRules` / `rulesJson` |
| Enrichment resource | `codetelemetry_enrichment_config` | `EnrichmentConfig` |
| State owner | One Terraform state per workspace | One Pulumi stack per workspace |
| Local setup | Build the Go provider and configure a CLI development override | Run `npm run setup` to build the provider and generate/install the SDK |
| Preview / apply | `terraform plan` / `terraform apply` | `pulumi preview` / `pulumi up` |
| Tool requirements | Go 1.26+ and Terraform 1.5+ | Go 1.26+, Node.js 22+, npm and Pulumi 3.268.0+ |

Clone the shared repository, then follow the setup guide for your chosen tool:

```bash
git clone https://github.com/CodeTelemetry/codetelemetry-iac.git
cd codetelemetry-iac
```

### Terraform: declare resources in HCL

After following the [provider build and CLI override setup](terraform-provider-codetelemetry/README.md#build-and-use-locally),
copy the complete [Terraform example](terraform-provider-codetelemetry/examples/main.tf)
and [alarm JSON](terraform-provider-codetelemetry/examples/rules.json) into your
working directory. Its resource declarations look like:

```hcl
resource "codetelemetry_alarm_rules" "main" {
  rules_json = file("${path.module}/rules.json")
}

resource "codetelemetry_enrichment_config" "main" {
  mint_floor            = 0.6
  merge_threshold       = 0.86
  consolidation_cadence = "nightly"
  budget_pct            = 0.25
}
```

Supply `CODETELEMETRY_ENDPOINT` and a current `CODETELEMETRY_TOKEN` through your
shell or secret manager, then run `terraform plan` and `terraform apply`.
The source-build guide uses a development override; follow its instructions
before running these commands. Registry installation is not available yet.

### Pulumi: declare resources in TypeScript

From `pulumi-codetelemetry/`, run `npm run setup`, select your backend, and create a
stack as described in the [Pulumi setup guide](pulumi-codetelemetry/README.md#setup-and-deployment).
The generated SDK exposes the same resources using TypeScript names:

```typescript
import * as fs from "node:fs";
import * as pulumi from "@pulumi/pulumi";
import * as codetelemetry from "@codetelemetry/pulumi-codetelemetry";

new codetelemetry.AlarmRules("alarms", {
  rulesJson: pulumi.secret(fs.readFileSync("../terraform-provider-codetelemetry/examples/rules.json", "utf8")),
});

new codetelemetry.EnrichmentConfig("enrichment", {
  mintFloor: 0.6,
  mergeThreshold: 0.86,
  consolidationCadence: "nightly",
  budgetPct: 0.25,
});
```

Use the same endpoint and ID-token environment variables, then run
`pulumi preview` and `pulumi up`. Terraform CLI is not required for this path.
The generated local SDK needs its matching provider executable at runtime;
`npm run setup` prepares both. See the guide before using it in another project.

## Shared behavior and ownership

Both integrations manage the **entire alarm list** and the four existing
classifier enrichment settings. Custom grader definitions do not currently have
an API in this project. They configure an existing CodeTelemetry service.

Credentials must contain a current Cognito **ID token**; ingest keys and access
tokens are not accepted. The token identifies the workspace. Use exactly one
Terraform state **or** one Pulumi stack to own a workspace's settings; avoid
concurrent Terraform, Pulumi, and UI writers. Import existing settings before
adopting them and inspect the resulting plan or preview.

**Destroy clears all alarms and restores enrichment defaults.** Protect production
state and encryption keys. Terraform sensitive values remain in state and saved
plans; Pulumi encrypts values marked secret. Both guides cover adoption, drift,
revision conflicts, workspace identity, and deletion protection.

## Validate Terraform and Pulumi

With Go 1.26+, Node.js 22+, npm, Terraform 1.13.5, and Pulumi 3.268.0 installed:

```bash
bash scripts/validate.sh
```

This runs both integrations: provider acceptance tests and race checks, SDK
build/typechecking, and real Pulumi lifecycle tests against local HTTP fixtures.
No live credentials are needed. Select one suite with `--terraform` or `--pulumi`;
see the [validation guide](docs/VALIDATION.md) for coverage and CI details.

## Signed candidates and package publishing

Every push to `main` validates both integrations, builds six platform archives and
a local-bridge SDK tarball, signs the candidate manifest, verifies signatures and
fresh installs, and runs authenticated Terraform and Pulumi lifecycles in an
isolated test workspace. A candidate is verified only when the whole build passes,
including cleanup. See [release verification](docs/RELEASING.md).

The repository name covers both IaC tools. Terraform executable names, resource
types, and the `CodeTelemetry/codetelemetry` provider source address remain
Terraform-specific. A future Terraform Registry release requires a separate
`terraform-provider-codetelemetry` distribution repository; npm/Registry package
promotion is separate from publishing this source. See the
[package release plan](docs/PUBLIC_RELEASE_PLAN.md).

## Contributing and security

Read [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), and the
[publication review](docs/PUBLICATION_REVIEW.md). Report vulnerabilities privately
through the repository's Security tab.

## License

[Mozilla Public License 2.0](LICENSE) (SPDX: `MPL-2.0`). Third-party dependencies
retain their own licenses.
