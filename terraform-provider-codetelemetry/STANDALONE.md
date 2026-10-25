# Standalone validation

This repository contains two sibling packages, `terraform-provider-codetelemetry`
and `pulumi-codetelemetry`, backed by a shared provider implementation.

The canonical [validation guide](../docs/VALIDATION.md) covers both suites,
prerequisites, CI triggers, coverage, and limitations. From the repository root:

```bash
bash scripts/validate.sh
```

Use `--terraform` or `--pulumi` to select one suite. The command formerly located
in this package forwards to that repository-level entrypoint for compatibility.

The shared source home is `CodeTelemetry/codetelemetry-iac`. See the
[public release plan](../docs/PUBLIC_RELEASE_PLAN.md) for ownership, distribution,
licensing, signing, consumer-installation tests, and the private-source boundary.
