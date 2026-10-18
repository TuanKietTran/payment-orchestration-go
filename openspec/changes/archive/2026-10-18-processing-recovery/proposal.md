# Durable processing intent and posting evidence

The existing process command can lose its ledger reference if the downstream commit succeeds but the local status update fails. Introduce an atomic ALLOW decision plus outbox intent, and atomically persist SETTLED plus ledger reference while removing the intent. Replay always uses `payment:<payment idempotency key>`; downstream idempotency is required.

The worker scans at startup and every second, with batches bounded to 100. Manual process retries also dispatch pending intents. Memory supports the same repository contract but is not restart durable. Apply both SQL migrations before running with DATABASE_URL. This card does not implement explicit PSP authorization/capture, fees, refunds, reconciliation or downstream retry/breaker policies.
