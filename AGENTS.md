# Repository Agent Rules

## Code Review Rules

- Review evidence is valid only for the exact current head SHA. A review on an older commit never clears a newer head.
- Preserve behavioral parity across sample, local-development, and production paths unless the task explicitly changes that contract.
- Bound rows, bytes, memory, and wall-clock work in collectors, schedulers, and persisted state. Scheduled work must remain fair when one tenant, repository, or provider is unusually large.
- Treat persisted files and schemas as compatibility boundaries: old data must remain readable, and new state must have explicit size bounds.
- For broad changes, state the architecture, affected subsystems, invariants, risks, and validation before implementation. Prefer focused commits for storage/schema, core behavior, infrastructure/API, and UI concerns.
- Run the repository validation and a local `codex review --base <base>` preflight before the final GitHub review gate. Batch actionable fixes, then rerun the local review on the repaired head.
- Keep the pull request draft while requesting the single final-head GitHub Codex review. Mark it ready only after that serverless review is clean, so `ready_for_review` deliberately admits the reviewed head to scarce self-hosted CI capacity.
- GitHub Codex is the independent final-head gate. Do not use repeated GitHub `@codex review` comments as the iterative debugger, and do not merge with unresolved current threads or failing required checks.
