# Security policy

These integrations are unpublished and experimental. There is no supported
public release yet; use the latest reviewed commit for local evaluation. Build
with a supported, patched Go toolchain: the reviewed patch levels are 1.26.10
and 1.27.3. Go 1.26 is the language-version floor, not a recommendation to use
unpatched Go 1.26.0 or Go 1.27.0.

## Reporting a vulnerability

Do not post credentials, webhook secrets, customer telemetry, state files, saved
plans, or exploit details in public issues or pull requests.

Use the [private vulnerability reporting form](https://github.com/CodeTelemetry/codetelemetry-iac/security/advisories/new)
under **Security → Report a vulnerability**. Maintainers must enable and verify
this channel when making the repository public. If the private form is unavailable,
use an existing private maintainer contact rather than posting a public report.

Include the affected commit/version, tool versions, expected and observed
behavior, and a minimal reproduction using dummy credentials. Maintainers should
acknowledge private reports and coordinate any fix and disclosure with reporters.

## Credentials and state

The API remains responsible for authenticating Cognito ID tokens. The provider's
workspace fingerprint only prevents accidental switching between service origins
or token issuer/subject pairs. It does not verify a JWT signature or grant access.

New resource state carries a SHA-256 workspace fingerprint; refresh, update, and
destroy refuse mismatches before HTTP. Routine token renewal with the same issuer
and subject is supported. Legacy state without a fingerprint binds to the current
credentials on its first refresh: verify the original workspace credentials before
upgrading or importing old state.

Sensitive Terraform attributes are still present in plaintext in Terraform state
and saved plan files. Pulumi encrypts values marked secret, but backend access and
encryption keys still need protection. Logging with extra tool/debug tracing may
expose data outside the provider's redacted diagnostics; never attach raw traces
or state files to public reports.

## Release smoke account

Live artifact checks use a dedicated unprivileged Cognito account and an encrypted
`release-verification` environment password. Only main-branch jobs receive it.
A pinned SRP client obtains an ID token for each run; no AWS administration
credentials, static ID tokens or refresh tokens are stored in CI. The account
must have no customer-team memberships or ingest keys. Rotate its password and
GitHub secret together. Never log the auth helper's private token output.
