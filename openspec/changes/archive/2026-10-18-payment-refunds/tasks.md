# Tasks

- [x] Add domain refund model and atomic memory/Postgres reservation/completion/recovery.
- [x] Implement idempotent reverse-ledger refund command and startup worker recovery.
- [x] Publish callable refund HTTP contract with conflict/invalid/transient mappings.
- [x] Prove partial/full, concurrent cumulative caps, payload conflicts, ambiguous response and durable replay.
- [x] All milestone gates passed on Go 1.27.1 darwin/arm64 (2026-10-18). Domain 100%, weighted non-cmd 80.80% (589/729), native Postgres repository 83.1%. Evidence: control logs/payments/card4e-gates.log. Archive this slice; explicit PSP and reconciliation remain unfinished.
