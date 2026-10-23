# Contributing

The project is currently an unpublished integration. Its planned public home is
a fresh repository in the CodeTelemetry organization. Contributions are provided
under the [Mozilla Public License 2.0](LICENSE). Third-party code must retain its
required license notices.

Read [AGENTS.md](AGENTS.md) and run the full repository validation:

```bash
bash scripts/validate.sh
```

See [the validation guide](docs/VALIDATION.md) for prerequisites, individual suites,
and their limits. Add a regression test for changes to lifecycle behavior, state
compatibility, authentication handling, or bounded work. Run both integrations
when changing the shared Go provider. Test with dummy tokens and local servers;
ordinary pull-request CI must not require live credentials or private source.

Keep pull requests focused and describe the observable problem, resulting
behavior, compatibility impact, and validation performed. Preserve old readable
state and document any explicit migration. Match review evidence to the current
head and satisfy required checks before merging.

Do not include customer data, saved state/plans, personal credentials, private
application source, or raw debug traces. Report vulnerabilities according to
[SECURITY.md](SECURITY.md), not in public issues.
