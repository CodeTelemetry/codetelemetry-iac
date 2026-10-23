# Verified release candidates

The repository is `CodeTelemetry/terraform-provider-codetelemetry`. It was seeded
from a hardened source snapshot, not transferred or mirrored from the earlier
private repository. The license is MPL-2.0. Public visibility and package
publication are separate launch steps.

## Main builds

After Terraform/Pulumi matrix validation and publication security pass on the
exact `main` commit, the candidate pipeline:

- Builds Linux, macOS, and Windows archives for amd64 and arm64, with the project
  license and Go dependency notices.
- Packs the compiled Node SDK as a local-bridge candidate. This SDK currently
  requires the matching executable in the consumer project's `./bin` directory;
  it is not a standalone Registry-backed npm release.
- Records the repository, source revision, workflow run, toolchain and platform
  matrix in `BUILD.json`, and hashes all payloads in `SHA256SUMS`.
- Signs `SHA256SUMS` with Cosign using the GitHub Actions OIDC identity, then checks
  its certificate issuer, exact workflow identity, source SHA, and every payload
  checksum. Sigstore's public transparency service records signing metadata,
  including the repository/workflow identity and artifact digest; source and
  credentials are not uploaded there.
- Tests fresh Terraform filesystem-mirror installation and npm-tarball loading,
  then stores a `signed-candidate-SHA-ATTEMPT` Actions artifact for 14 days.

The candidate version is `0.0.0-dev.RUN_NUMBER`. These builds never create a GitHub
release or push a Terraform Registry/npm version. Candidate installation tests
are independent of the Registry; they do not establish Registry distribution.

## Repository controls pending activation

GitHub currently rejects branch protection for this private organization repository
under its plan. Default workflow permissions are read-only and the release
verification environment allows only `main`, but these are not a substitute for
enforced PR/required-check protection. Upgrade the organization's plan or approve
public visibility after review, then enable required checks, PR reviews, stale
review dismissal, conversation resolution, and admin enforcement. Private
vulnerability reporting also needs activation/verification before public launch.
Do not treat these unavailable controls as configured.

## Authenticated verification

The next job downloads the artifact from that same run and verifies its signature,
source SHA and checksums again before using it. It uses the `release-verification`
environment, restricted to the `main` branch. Live jobs are serialized and running
main workflows are not automatically canceled by new pushes.

Configure these **environment variables**, using a dedicated non-production
principal/workspace with no customer settings:

| Variable | Meaning |
| --- | --- |
| `SMOKE_ENDPOINT` | HTTPS service origin, without a path/query/fragment |
| `SMOKE_EXPECTED_ISSUER` | Exact Cognito ID-token issuer |
| `SMOKE_EXPECTED_SUB` | Exact subject of the isolated test principal |
| `SMOKE_DEDICATED_WORKSPACE` | Literal `true`, confirming this is a disposable workspace |

Configure the environment **secret** `SMOKE_ID_TOKEN` through GitHub's secret UI
or a secure credential rotation mechanism. Never paste a token into an issue,
chat, command argument, or repository file. It must be a service-valid Cognito ID
token with at least 15 minutes remaining when the smoke runner starts. Claim
comparison is a configuration guard; the service verifies authentication.

A static ID token expires. Continuous unattended main verification therefore
requires an approved token acquisition/rotation mechanism for that dedicated
account. No production credentials are copied and no refresh mechanism is
invented by this setup. Until the workspace and a current token are configured,
the authenticated job deliberately fails before contacting the service. A signed
candidate is not considered fully verified for release while that job is failing.

The runner refuses nonempty alarms or non-default enrichment before writing. It
installs the built artifacts in temporary consumers, tests Terraform create,
update, read/no-drift, import, and destroy, then Pulumi create, update, refresh,
no-drift and destroy. Test rules are disabled. Cleanup runs on ordinary errors,
and a failed cleanup fails the job; inspect and reset the dedicated workspace
before retrying. A runner crash, manual cancellation, or hard timeout may prevent
cleanup, so verify this workspace before rerunning. The singleton API prevents
isolating concurrent writers within a shared account; never use a production or
shared account.

State, plans, credentials, and raw CLI output are not uploaded. Commands have
bounded runtimes and buffered output; service responses are bounded to 2 MiB.

## Independently verify a download

Use Cosign 3.1.4 and Node 24. Obtain the expected source SHA and candidate version
from the trusted workflow run, not from an unverified manifest. Download that run's
artifact and run from a trusted checkout:

```bash
node scripts/verify-artifacts.mjs /absolute/path/to/download EXPECTED_SHA 0.0.0-dev.RUN_NUMBER
```

The verification requires this certificate identity:

```
https://github.com/CodeTelemetry/terraform-provider-codetelemetry/.github/workflows/codetelemetry-providers.yml@refs/heads/main
```

and issuer `https://token.actions.githubusercontent.com`. Do not disable certificate,
transparency-log, or checksum checks. Only the dedicated signing job has OIDC signing permission. Packaging and
consumer execution have read-only permissions and no live credential access.
Signing establishes origin and integrity;
it does not replace the security or authenticated service checks.

## Promote a public release

Before adding release promotion, configure the Terraform Registry namespace and
organization-controlled GPG key/fingerprint for its required detached checksum
signature. A Cosign bundle supplements that signature; it is not a substitute for
Terraform Registry's signing requirements. Reserve/verify npm publishing rights
before using `@codetelemetry/pulumi-codetelemetry` publicly. Choose a Registry-backed
bridge configuration or portable plugin distribution for a standalone SDK.

Promotion must select an exact fully verified commit, use a stable version and
matching artifacts, and pass fresh final installation checks. Public visibility,
Registry onboarding, GPG signing and npm promotion are not performed by the
candidate workflow. See [the publication review](PUBLICATION_REVIEW.md).
