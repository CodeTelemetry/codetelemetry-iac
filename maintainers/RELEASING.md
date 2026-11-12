# Verified release candidates

The repository is `CodeTelemetry/terraform-provider-codetelemetry`. It was seeded
from a hardened source snapshot, not transferred or mirrored from the earlier
private repository. The license is MPL-2.0. Every `main` commit produces a
verified candidate; a version tag on a verified commit produces a Terraform
Registry release.

## Main builds

After matrix validation and publication security pass on the
exact `main` commit, the candidate pipeline:

- Builds Linux, macOS, and Windows archives for amd64 and arm64, with the project
  license and Go dependency notices.
- Records the repository, source revision, workflow run, toolchain and platform
  matrix in `BUILD.json`, and hashes all payloads in `SHA256SUMS`.
- Signs `SHA256SUMS` with Cosign using the GitHub Actions OIDC identity, then checks
  its certificate issuer, exact workflow identity, source SHA, and every payload
  checksum. Sigstore's public transparency service records signing metadata,
  including the repository/workflow identity and artifact digest; source and
  credentials are not uploaded there.
- Tests fresh Terraform filesystem-mirror installation,
  then stores a `signed-candidate-SHA-ATTEMPT` Actions artifact for 14 days.

The candidate version is `0.0.0-dev.RUN_NUMBER`. Candidate builds never create a
GitHub release or a Terraform Registry version. Candidate installation tests are
independent of the Registry; they do not establish Registry distribution.

## Repository controls

Only this repository's main workflow identity is accepted for new candidate
signatures. Candidates signed under the former `codetelemetry-iac` name use that
identity and must be verified with that revision's trusted verifier; do not
broaden the current identity policy to accept both.

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

Configure these values in the `release-verification` environment, using a
dedicated non-production principal/workspace with no customer settings. Store
the account and pool identifiers as environment **secrets**, not variables:
Actions prints each step's variables in the public run log, and only secrets
are masked. The endpoint and the dedicated-workspace flag stay variables.

| Name | Kind | Meaning |
| --- | --- | --- |
| `SMOKE_ENDPOINT` | variable | HTTPS service origin, without a path/query/fragment |
| `SMOKE_EXPECTED_ISSUER` | secret | Exact Cognito ID-token issuer |
| `SMOKE_EXPECTED_SUB` | secret | Exact subject of the isolated test principal |
| `SMOKE_DEDICATED_WORKSPACE` | variable | Literal `true`, confirming this is a disposable workspace |
| `SMOKE_USER_POOL_ID` | secret | Cognito pool matching the expected issuer |
| `SMOKE_CLIENT_ID` | secret | Existing dashboard SRP client ID accepted by the service |
| `SMOKE_USERNAME` | secret | Dedicated CI-only account username |

Also configure the environment secret `SMOKE_PASSWORD`. Set secrets through GitHub's secret UI
or an authorized operator process using private stdin. The account is dedicated
to provider CI, has no privileged Cognito groups, customer-team memberships, or
ingest keys, and starts with empty alarms and default enrichment. It is an
isolated test workspace on the live service; it is not a separate service deployment.

The main-only smoke job installs the pinned, audited SRP client without credentials.
Its final step signs in with Cognito's existing `USER_SRP_AUTH` flow, obtains a fresh
ID token for that run, and validates issuer, subject, audience, token use and expiry
before contacting the service. It does not change the pool/client's auth policy.
The auth subprocess has a 60-second timeout, bounded output, and memory-only token
storage. Sign-in failures and interactive MFA/password-reset challenges fail closed.

The password and returned token stay out of command arguments, `GITHUB_ENV`, logs,
state uploads, and artifacts. The short-lived token travels through a captured
private pipe; the password is removed before invoking Terraform.
No static ID-token secret or expiring refresh-token secret is required. Rotate the
Cognito password and the environment secret together if access must be replaced.
Do not enter a customer's or maintainer's password in this environment.

Never execute `scripts/auth/login.mjs` as a raw Actions step: it is an internal
helper whose stdout contains the token for its caller. The workflow invokes
`scripts/run-authenticated-smoke.mjs`, which captures that output privately and
runs the lifecycle. Ordinary PR tests use mocked auth and need no credentials.

The runner refuses nonempty alarms or non-default enrichment before writing. It
installs the built artifacts in temporary consumers, tests Terraform create,
update, read/no-drift, import, and destroy. Test rules are disabled. Terraform import checks save a private recovery checkpoint before removing a
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
https://github.com/CodeTelemetry/terraform-provider-codetelemetry/.github/workflows/codetelemetry-providers.yml@refs/heads/main
```

and issuer `https://token.actions.githubusercontent.com`. Do not disable certificate,
transparency-log, or checksum checks. Only the dedicated signing job has OIDC signing permission. Packaging and
consumer execution have read-only permissions and no live credential access.
Signing establishes origin and integrity;
it does not replace the security or authenticated service checks.

## Publish a Terraform Registry release

1. Choose a `main` commit whose candidate pipeline passed, including the
   authenticated smoke test and cleanup.
2. Tag it `vX.Y.Z` and push the tag. Tags are permanent release identifiers: do
   not move or reuse one.
3. The [release workflow](../.github/workflows/release.yml) refuses tags that are
   not stable versions, not on `main`, or whose commit has no successful main
   candidate run. It rebuilds the six platform archives with the stable version,
   writes the Registry manifest and `SHA256SUMS`, signs the checksums, verifies
   the signature against the committed public key, and publishes the GitHub
   release.
4. The Terraform Registry ingests the release. Prove a fresh `terraform init`
   and commit-ready lock file from the Registry before announcing it.

### Signing key

The Registry requires a GPG signature of the checksum file. The organization
release key is RSA 4096 with fingerprint
`01989D0333D09DFBA0D31AB3959DFE4A42E6C7EE`; its public half is
[release-signing-key.asc](../release-signing-key.asc) and is the key registered
with the `CodeTelemetry` Registry namespace. The private key and passphrase are
held in the organization's secrets manager and supplied to the release job as the
`GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` secrets of the `release` environment,
which only version tags may deploy to. The key never signs candidates; Cosign
does that with the workflow identity.

To rotate the key, add the new public key to the Registry namespace before the
first release it signs, replace the committed public key and both environment
secrets together, and keep the old key registered so existing releases still
verify.

### One-time Registry onboarding

Sign in to the Terraform Registry with a GitHub account that administers the
CodeTelemetry organization, add the public key to the `CodeTelemetry` namespace,
and publish this repository as a provider. See the
[release plan](PUBLIC_RELEASE_PLAN.md).
