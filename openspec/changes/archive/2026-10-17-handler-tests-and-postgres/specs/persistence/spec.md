## ADDED Requirements

### Requirement: In-memory store by default
The system SHALL keep payments in process memory when `DATABASE_URL` is unset.

#### Scenario: Read follows create in the same process
- **GIVEN** `DATABASE_URL` is unset
- **WHEN** a client creates a payment and then reads it
- **THEN** the read returns that payment
- **AND** the data is gone when the process exits

### Requirement: Postgres store when configured
The system SHALL use Postgres when `DATABASE_URL` is set, and SHALL apply the same create, read, and idempotency rules there.

#### Scenario: Payment survives a new connection
- **GIVEN** `DATABASE_URL` points at a database where `migrations/001_init.sql` has been applied
- **WHEN** a client creates a payment
- **THEN** a later read through a new connection returns that payment, including amount, currency, status, and ledger transaction id when one was stored

#### Scenario: Duplicate key does not insert a second row
- **GIVEN** the Postgres store already holds a payment for an idempotency key
- **WHEN** a second create uses that key
- **THEN** the store rejects it with `idempotency key already used` and the original payment id

### Requirement: Status update on the store
The store SHALL update the status of a known payment and SHALL reject an unknown id. This port is not on the HTTP API.

#### Scenario: Mark succeeded
- **GIVEN** a stored payment is `PENDING`
- **WHEN** the store updates it to `SUCCEEDED`
- **THEN** a later read returns `SUCCEEDED`

#### Scenario: Unknown id
- **WHEN** the store updates a UUID that was never stored
- **THEN** the update fails with `payment not found`
