# Tasks

- [x] Bound attempts, exponential backoff with jitter, context-aware wait and response size.
- [x] Retry only transient failures; preserve serialized request identity.
- [x] Add concurrency-safe open/half-open/closed breaker with stale-result exclusion.
- [x] Test HTTP status classes, injected timeouts, cancellation, bounded decoding and half-open concurrency.
- [x] All milestone gates passed on Go 1.27.1 darwin/arm64 (2026-10-18); downstream 86.4%, weighted non-cmd 76.23% (372/488), below final 80% requirement. Evidence: control logs/payments/card4d-gates.log. Archive this slice; PSP/lifecycle work remains open.
