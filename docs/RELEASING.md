# Verified release candidates

The repository is `CodeTelemetry/codetelemetry-iac`. It was seeded
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

## Repository controls

The source home is `CodeTelemetry/codetelemetry-iac`; Terraform HCL and Pulumi
TypeScript share this candidate pipeline. Only this repository's main workflow
identity is accepted for new candidate signatures. Candidates signed before the
rename use the old identity and must be verified with that revision's trusted
verifier; do not broaden the current identity policy to accept both.

Public source launch requires main protection for both matrix jobs and Publication
security, stale-review dismissal, conversation resolution, and admin enforcement.
Private vulnerability reporting must be enabled and verified. These controls were
unavailable under the private organization's plan, so verify them after making
the repository public. The default workflow token is read-only, and the
release-verification environment permits only main.

## Authenticated verification

Downstream jobs use the producing job's immutable artifact ID, including on
failed-job reruns; they do not infer an artifact name from the consumer attempt.
The live job downloads the artifact from that same run and verifies its signature,
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
| `SMOKE_USER_POOL_ID` | Cognito pool matching the expected issuer |
| `SMOKE_CLIENT_ID` | Existing dashboard SRP client ID accepted by the service |
| `SMOKE_USERNAME` | Dedicated CI-only account username |

Configure the environment **secret** `SMOKE_PASSWORD` through GitHub's secret UI
or an authorized operator process using private stdin. The account is dedicated
to provider CI, has no privileged Cognito groups, customer-team memberships, or
ingest keys, and starts with empty alarms and default enrichment. It is an
isolated test workspace on the live service; it is not a separate service deployment.

The main-only smoke job installs the pinned, audited SRP client without credentials.
Its final step signs in with Cognito's existing `USER_SRP_AUTH` flow, obtains a fresh
ID token for that run, and validates issuer, subject, audience, token use and expiry
before contacting the service. It does not change the pool/client's auth policy.
The auth subprocess has a 60-second timeout, bounded output, and memory-only token
storage. SDK failures and interactive MFA/password-reset challenges fail closed.

The password and returned token stay out of command arguments, `GITHUB_ENV`, logs,
state uploads, and artifacts. The short-lived token travels through a captured
private pipe; the password is removed before invoking Terraform, Pulumi or npm.
No static ID-token secret or expiring refresh-token secret is required. Rotate the
Cognito password and the environment secret together if access must be replaced.
Do not enter a customer's or maintainer's password in this environment.

Never execute `scripts/auth/login.mjs` as a raw Actions step: it is an internal
helper whose stdout contains the token for its caller. The workflow invokes
`scripts/run-authenticated-smoke.mjs`, which captures that output privately and
runs both lifecycles. Ordinary PR tests use mocked auth and need no credentials.

The runner refuses nonempty alarms or non-default enrichment before writing. It
installs the built artifacts in temporary consumers, tests Terraform create,
update, read/no-drift, import, and destroy, then Pulumi create, update, refresh,
no-drift and destroy. Test rules are disabled. Terraform import checks save a private recovery checkpoint before removing a
state binding and restore it if import fails, so destroy still owns each resource.
Cleanup runs on ordinary errors,
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
https://github.com/CodeTelemetry/codetelemetry-iac/.github/workflows/codetelemetry-providers.yml@refs/heads/main
```

and issuer `https://token.actions.githubusercontent.com`. Do not disable certificate,
transparency-log, or checksum checks. Only the dedicated signing job has OIDC signing permission. Packaging and
consumer execution have read-only permissions and no live credential access.
Signing establishes origin and integrity;
it does not replace the security or authenticated service checks.

## Promote a public release

Before adding release promotion, create the separate Terraform distribution
repository described in the [package release plan](PUBLIC_RELEASE_PLAN.md), then
configure the Terraform Registry namespace and
organization-controlled GPG key/fingerprint for its required detached checksum
signature. A Cosign bundle supplements that signature; it is not a substitute for
Terraform Registry's signing requirements. Reserve/verify npm publishing rights
before using `@codetelemetry/pulumi-codetelemetry` publicly. Choose a Registry-backed
bridge configuration or portable plugin distribution for a standalone SDK.

Promotion must select an exact fully verified commit, use a stable version and
matching artifacts, and pass fresh final installation checks. Public visibility,
Registry onboarding, GPG signing and npm promotion are not performed by the
candidate workflow. See [the publication review](PUBLICATION_REVIEW.md).
