### Requirement: Bounded idempotent refund reservations

#### Scenario: Concurrent partial refunds
- **GIVEN** a settled payment
- **WHEN** refunds are reserved concurrently
- **THEN** each reservation is positive and pending plus completed amounts never exceed the payment amount
- **AND** identical key/payment/amount replays the original refund while changed payload conflicts.

### Requirement: Recoverable reverse postings

#### Scenario: Ambiguous reverse ledger response
- **GIVEN** a persisted pending refund
- **WHEN** its reverse ledger post succeeds but the response or local completion fails
- **THEN** recovery reuses the same refund UUID ledger key
- **AND** completion atomically stores the refund posting reference and completed total
- **AND** the payment becomes PARTIALLY_REFUNDED or REFUNDED only after ledger evidence.

### Requirement: Repeated partial refund transition

#### Scenario: Additional refund leaves a remaining balance
- **GIVEN** a partially refunded payment
- **WHEN** another positive refund completes and the cumulative total remains below the original amount
- **THEN** PARTIALLY_REFUNDED to PARTIALLY_REFUNDED is legal
- **AND** the positive amount and cumulative cap are checked independently of the status transition.
