# Service API contract used by the providers

The Terraform and Pulumi integrations use an existing CodeTelemetry service.
They do not deploy it. Configure the service origin and a current Cognito ID token;
all requests use `Authorization: Bearer <ID token>`. Ingest keys and Cognito access
tokens are not accepted. The service verifies authentication and determines the
workspace from the principal; the provider cannot select a tenant in the body.

| Resource | Read | Write | Destroy |
| --- | --- | --- | --- |
| Alarm rule list | `GET /api/alert-rules` | `PUT /api/alert-rules` with `{rules, rev}` | PUT an empty list with the saved revision |
| Enrichment settings | `GET /api/enrichment-config` | `PUT /api/enrichment-config` with all four settings | PUT service defaults |

## Alarm rules

Reads and successful writes return a JSON object containing `rules` (an array)
and `rev` (a nonnegative integer). Other response metadata is ignored. PUT is a
whole-list replacement with the caller's saved revision; a stale revision returns
HTTP 409. There are at most 60 rules and 300,000 bytes of rule JSON. Creation in
Terraform or Pulumi refuses an existing nonempty list until it is imported.

Each rule has a stable `id`, `name`, `enabled`, `type`, `params`, and `channels`.
The provider requires explicit values for defaulted fields: `cooldownMinutes`,
`severity`, `evaluateEveryMinutes`, `notifyOnRecovery`, and `origin`.
TQL rules additionally use a `tql` object with `query`, `comparator`, `threshold`,
and optional `column`/`minMatched`. `scope` can narrow the condition. See the
[complete local example](../terraform-provider-codetelemetry/examples/rules.json).
The service owns query compilation, privacy gates, delivery validation, and
per-field limits; malformed or unsupported rules are rejected before persistence.
The provider additionally rejects inputs the API would silently trim or strip.

## Enrichment settings

Reads and successful writes return `mintFloor`, `mergeThreshold`,
`consolidationCadence`, `budgetPct`, and server-owned `configRev`.
Only the first four fields are sent on write. Fractions range from 0 to 1;
cadence is `nightly`, `weekly`, or `manual`. Defaults are 0.6, 0.86, `nightly`,
and 0.25 respectively. Enrichment writes currently use last-write-wins semantics;
`configRev` invalidates service caches and is not a conditional-write token.
Avoid concurrent UI and infrastructure writers for these settings.

## Failure and transport behavior

A 401, 403, 404, or 503 is an error, not an instruction to remove a resource from
state. Writes have no automatic application-level retries because a timeout may
follow an already committed write. HTTP requests have a 30-second timeout, a
350,000-byte request bound, and a 2 MiB response bound. Redirects are refused.
HTTPS is required except for loopback development.

Destroy intentionally clears alarms or restores defaults. Use one state owner
per workspace and review destructive plans. See [SECURITY.md](../SECURITY.md) for
workspace binding, legacy-state migration, credential handling, and reporting.

These are the contracts exercised by the local HTTP fixtures. The automated
suites do not assert compatibility with every deployed service version; a live
service smoke remains necessary before declaring a supported public release.
