# Adversarial publication review — 2026-10-22

**Disposition: public package release is not cleared.** The code is an unpublished,
experimental integration. MPL-2.0 has been selected; the final distribution identity and release
validation are still required before a supported release. The fresh CodeTelemetry organization repository has been created;
public visibility and registry/npm publication have not been performed.

The pre-publication review examined reachable Git refs, tracked files, HTTP and credential handling, state transitions,
Terraform and Pulumi behavior, dependency advisories, and the CI trust boundary.
CI and independent review receipts must identify the final candidate commit;
this document is not a permanent security certification.

## Findings addressed in this hardening change

### P1: credential changes could silently rebind singleton resources

Both resources formerly used only the literal ID `workspace`. Changing service
origins or tenant credentials could refresh state from a different workspace and
then overwrite or clear its settings. A computed `workspace_binding` now hashes
service origin plus stable Cognito issuer/subject claims. Reads, updates, and
deletes reject mismatches before HTTP; token renewal preserves the fingerprint.
The service remains the authentication authority. Unverified claim decoding in
the client is only a misconfiguration guard, not an authorization decision.

Regression tests cover both resources and every guarded operation, legacy-state
backfill, stable token renewal, exact claim-name matching, and Pulumi attempts to change the tenant. Existing
state stays readable: a legacy state without a fingerprint binds once on first
refresh. **Verify original credentials before that upgrade**; old state cannot
prove an identity it never recorded. Intentional imports use current credentials.

### P1: reachable Go dependency advisories and a vulnerable build toolchain

The baseline Go vulnerability scan identified reachable advisories in HTTP/2,
gRPC, text handling, and certificate verification. The dependencies are updated
and CI now builds with patched Go 1.26.10 / 1.27.3. The local build minimum rises
to Go 1.26 because patched dependencies require it. The updated candidate passed
a new govulncheck scan with no reported vulnerabilities.

Examples of primary advisory evidence:
[GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809),
[GO-2026-6612](https://pkg.go.dev/vuln/GO-2026-6612), and
[GO-2026-6443](https://pkg.go.dev/vuln/GO-2026-6443).
Reachability is conservative static analysis, not proof of a remote exploit.

### P2: malformed origin URLs and API string normalization

Empty queries/fragments, encoded origin paths, and empty hostnames are rejected
rather than being concatenated into malformed API URLs. Accepted origins are reduced to scheme and host before use. Rule strings containing leading/trailing BOM whitespace are rejected
before persistence because the API trims that character and would otherwise
return a different value from Terraform's plan.

### P2: computed apply inputs bypassed plan-time validation

Terraform config validation can receive unknown values. After those values resolve,
invalid enrichment fractions could reach the API's default/clamp behavior, and
oversized or lossy rule JSON could be marshaled or written before a useful error.
The shared write boundary now revalidates resolved rules, bounds bytes before
marshaling, and rejects invalid/unknown enrichment settings before HTTP. Tests
assert no HTTP requests for these inputs, protecting both Terraform and Pulumi.

### P2: publication checks were absent from CI

A new `Publication security` job scans all fetched Git history with Gitleaks,
checks reachable Go vulnerabilities, and audits the locked npm graph. Tools are
pinned and errors fail the job. Credential/state ignore rules, security reporting
guidance, contributor instructions, and a self-contained API contract are added.
Ordinary PR checks execute with read-only repository permissions and no cloud
secrets; there is no privileged `pull_request_target` or artifact-execution path.

## Confidentiality and dependency evidence

- Gitleaks scanned all reachable Git history at the review baseline with zero
  findings. The final committed candidate must be scanned again by CI.
- npm audit reported zero known vulnerabilities, including when run from a clean
  directory containing only package metadata. The final CI repeats the audit.
- No production JWT, customer telemetry, state, plan, or private application source
  was found in tracked content. Credential examples are placeholders or dynamically
  generated dummy tokens used only by local HTTP fixtures.
- The private repository has historical pull-request refs and cached commits.
  Replacing branch history cannot erase GitHub-owned refs. Current docs are
  self-contained; the planned public CodeTelemetry repository must be created
  fresh from the squashed snapshot, without importing private refs, issues, or
  pull requests. Do not transfer this private repository for publication.
- Automated scanners can miss secrets and vulnerabilities. They supplement the
  manual inspection and lifecycle tests; they do not establish absence of risk.

## Remaining publication decisions and release blockers

1. **License selected:** the project uses MPL-2.0. Audit third-party
   redistribution notices alongside final binary/SDK
   artifacts when packaging them.
2. **Distribution identity:** the intended public owner is CodeTelemetry, while
   module/source addresses now use CodeTelemetry. The fresh repository name
   `terraform-provider-codetelemetry` satisfies the Registry naming pattern.
   Registry ownership and consumer installation still need verification. See the
   [official publishing requirements](https://developer.hashicorp.com/terraform/registry/providers/publishing).
3. **Consumer distribution:** the main candidate workflow builds signed, cross-platform
   artifacts and tests filesystem-mirror/npm-tarball installation. There are no
   proven Registry/npm publication paths. Tests use local
   binaries and generated SDKs. The generated npm SDK currently uses the local
   name `@codetelemetry/pulumi-codetelemetry`; publishing under that scope must not be assumed
   available to CodeTelemetry. Choose an owned distribution identity. The main workflow builds and verifies signed candidates. Its main-only live
   smoke requires dedicated environment configuration before release readiness.
4. **Service compatibility:** tests exercise HTTP fixtures, not a deployed service
   or changing private application main. The main-only CodeTelemetry `main` job must pass its authenticated smoke
   test against a dedicated test workspace; supported API versions still need
   to be defined before a supported release.
5. **Security reporting:** enable and verify GitHub private vulnerability reporting
   under the final public organization repository before announcing it.

## Known operational limits retained

Enrichment has no conditional-write API; its updates and reset-on-destroy remain
last-write-wins. Both resources intentionally have destructive destroy semantics
that are documented. ID tokens expire and there is no refresh/service-account
flow. Protect Terraform state and saved plans; sensitive attributes remain in
those files. Pulumi secret outputs are encrypted by its configured backend.

The source can be evaluated locally with these limits. This review does not
authorize changing repository visibility, transferring ownership, or publishing
artifacts. Complete the [public release plan](PUBLIC_RELEASE_PLAN.md) and obtain
an explicit publication decision first.

Organization setup limitation: GitHub rejects branch-protection configuration for
this private repository under its current plan. The main-only release environment
and read-only default workflow token are configured; required PR/check enforcement
and private vulnerability reporting must still be enabled and verified before
public launch. See [release setup](RELEASING.md).
