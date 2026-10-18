# Tasks

- [x] Add deterministic injectable synthetic PSP and bounded transient retry/breaker wrapper.
- [x] Persist fraud decision, authorization and capture references under legal state transitions.
- [x] Preserve outbox intents across ambiguous PSP responses and local progress failures.
- [x] Fail explicit declines before cash posting, remove intent atomically.
- [x] Publish auto-driving capture route; retain review parking and one capture ledger transfer.
- [x] Test PSP before/after-commit injection, timeout/cancellation, decline, stable replay, memory/Postgres progress and HTTP capture.
- [x] Full milestone gates pass (2026-10-18 Go 1.27.1 darwin/arm64). Domain 100%, weighted non-cmd 82.96% (774/933); native Postgres repository 84.2%. FuzzValidateAmountCurrency ran 3s / 151185 executions with two workers. Evidence: control logs/payments/card4f-{gates,fuzz}.log. Archive this slice; authorization journals, PSP refunds, fees, reconciliation and hardening remain unfinished.
