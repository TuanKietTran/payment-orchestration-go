# Payment persistence design

- `repository.PaymentRepository` is the port: `Save`, `GetByID`, `UpdateStatus`.
- `NewInMemoryPayment` is the default. `NewPostgresPayment` is used when `DATABASE_URL` is set. The caller applies `migrations/001_init.sql`; the server does not migrate on startup.
- A new payment is always `PENDING`. `ledger_tx_id` is stored when the domain value is a UUID string, and returned as a string. An empty value stays null.
- Idempotency is `payments.idempotency_key`. A conflict returns `idempotency key already used: <existing uuid>`.
- `UpdateStatus` sets `status` and `updated_at`. It is covered by tests and is not mounted on Chi.
- HTTP create and get tests use the in-memory port. `TestPostgresPayment` runs create, idempotency, status, and a missing id on embedded Postgres 16, and skips when the test process is root.
