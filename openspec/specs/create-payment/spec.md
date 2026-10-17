# Create payment

## Purpose

Store a pending payment once for an idempotency key.

## Requirements

### Requirement: Create a pending payment
The system SHALL store a payment in `PENDING` and return its id.

#### Scenario: First create
- **GIVEN** key `pay-1` has not been used
- **WHEN** a client posts source `acct-src`, destination `acct-dst`, amount 19.99, currency `USD`, and that key
- **THEN** the response is HTTP 200 and contains `payment_id`
- **AND** a later read shows those fields, amount 19.99, currency `USD`, and status `PENDING`

### Requirement: Idempotency key is single use
The system SHALL reject a second create that reuses an idempotency key.

#### Scenario: Same key twice
- **GIVEN** a create with key `same-pay` returned payment id P
- **WHEN** the client posts again with `same-pay`
- **THEN** the response is HTTP 400
- **AND** the body contains `idempotency key already used` and P

### Requirement: Reject invalid JSON
The system SHALL reject a body that is not JSON.

#### Scenario: Not JSON
- **WHEN** the body is not JSON
- **THEN** the response is HTTP 400 and the body contains `invalid json`
