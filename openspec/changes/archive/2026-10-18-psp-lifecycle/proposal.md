# Explicit simulated PSP authorization and capture

Replace the implicit-success PSP with an injectable synthetic adapter. A committed ALLOW decision stores SCREENED and an outbox intent. Authorize and capture use stable operation keys; persist AUTHORIZED/AuthorizationID and CAPTURED/CaptureID before ledger dispatch. Settlement still requires ledger evidence. REVIEW also remains SCREENED but has no intent and a distinct FraudDecision.

Authorization is a simulated hold, not a cash ledger transfer; only capture posts customer-to-merchant cash, preventing double counting. This card does not claim separate off-balance-sheet authorization journal entries, nonzero fees or PSP refunds. Existing PROCESSING rows migrate to SCREENED/ALLOW for replay. Apply migration 004 after the earlier migrations.

Transient PSP faults retry with the bounded retry/breaker policy; explicit declines fail before cash posting and remove the intent. Ambiguous capture responses remain recoverable through stable PSP keys. The simulator produces deterministic references across restarts and supports before/after-commit faults in tests; it is not a real PSP or durable external account store.
