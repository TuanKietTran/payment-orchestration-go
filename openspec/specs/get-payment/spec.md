# Get payment

## Purpose

Return one stored payment, or say it is missing.

## Requirements

### Requirement: Return a stored payment
The system SHALL return the payment stored for an id.

#### Scenario: Known id
- **GIVEN** a payment was created
- **WHEN** a client gets `/v1/payments/{id}` with that id
- **THEN** the response is HTTP 200
- **AND** the body includes the id, idempotency key, accounts, amount, currency, and status

### Requirement: Unknown payment
The system SHALL respond that a missing id was not found.

#### Scenario: Unknown id
- **WHEN** the id is a UUID that was never created
- **THEN** the response is HTTP 404 and the body contains `payment not found`

### Requirement: Invalid id
The system SHALL reject an id that is not a UUID.

#### Scenario: Not a UUID
- **WHEN** the path id is not a UUID
- **THEN** the response is HTTP 400 and the body contains `invalid id`
