# Bounded downstream resilience

Retry identical serialized requests only for transport failures, HTTP 408/429 and 5xx. Three attempts maximum, each with a two-second HTTP client timeout. Wait 25ms then 50ms (exponential ceiling 250ms) with equal jitter in [half delay, delay], interruptible by context cancellation. Existing fraud event and ledger idempotency keys stay unchanged across attempts.

Each configured downstream adapter owns a circuit breaker: three exhausted logical operations open it for five seconds. Exactly one half-open probe is admitted; success closes it, failure reopens it, cancellation releases it without declaring the dependency healthy. Generations discard late results admitted before opening. Other 4xx and invalid successful JSON are not retried. Responses are bounded to 1MiB. All state is process-local; the durable posting intent remains the recovery source.

No new dependencies. Deterministic injected clocks/waits plus HTTP fixtures prove retry classification, exact payload identity, breaker exclusion and cancellation without synchronization sleeps. PSP remains a separate unfinished lifecycle card.
