# Tasks

- [x] Add atomic decision/intent and completion/reference repository operations in memory and Postgres.
- [x] Dispatch stable ledger keys and recover bounded batches without rescoring committed decisions.
- [x] Run a cancellable startup/periodic recovery worker.
- [x] Test ambiguous downstream commit, failed completion, concurrency and shared repository behavior.
- [x] Verify native Postgres evidence and full milestone gates (2026-10-18 macOS arm64 Go 1.27.1): build/vet/race/tidy pass; repository 78.9%, weighted non-cmd 68.03% (below final 80% gate). Archive this slice; full production-payments change remains open.
