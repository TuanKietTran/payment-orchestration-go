## ADDED Requirements
### Requirement: Payment lifecycle and contract honesty
The API contract SHALL distinguish implemented behavior from planned behavior. The implemented API SHALL create a PENDING payment and retrieve it by UUID; lifecycle processing, capture, refund, list and reconciliation SHALL NOT be represented as implemented until callable.
#### Scenario: Create pending payment
- **GIVEN** a valid request with idempotency_key, source_account, dest_account, amount and currency
- **WHEN** POST /v1/payments is called
- **THEN** the service returns a payment_id and persists a PENDING record
#### Scenario: Read payment
- **GIVEN** a known UUID
- **WHEN** GET /v1/payments/{id} is called
- **THEN** it returns the payment; malformed UUID is 400 and missing payment is 404
#### Scenario: Planned surface is explicitly unavailable
- **GIVEN** processing, capture, refunds, list or reconciliation are described in the contract
- **THEN** each is marked PLANNED and is not advertised as callable behavior
