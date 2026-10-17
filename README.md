# Payment Orchestration (CQRS)

Payment lifecycle service using CQRS.

## Commands
- CreatePayment
- CancelPayment
- RefundPayment

## Queries
- GetPayment
- ListPayments

## Run
```bash
go test ./...
go run ./cmd/server
```

`DATABASE_URL` selects the store. Unset uses the in-memory repository. When set, the server uses Postgres and expects `migrations/001_init.sql` to already be applied.
