## ADDED Requirements

### Requirement: Atomic processing intent

#### Scenario: Allow versus review or deny
- **GIVEN** a pending payment and a fraud result
- **WHEN** the decision is committed
- **THEN** ALLOW atomically stores PROCESSING and a ledger posting intent
- **AND** REVIEW stores SCREENED and DENY/BLOCK stores FAILED without a posting intent
- **AND** a concurrent later decision cannot overwrite the committed result.

### Requirement: Recoverable ledger completion

#### Scenario: Response lost after ledger commit
- **GIVEN** a persisted posting intent and a ledger implementing key-based idempotency
- **WHEN** the ledger response or local completion is lost
- **THEN** the intent remains recoverable and the payment is not reported settled
- **AND** replay uses the original ledger key and transaction reference
- **AND** completion atomically stores SETTLED plus LedgerTxID and removes the intent.

#### Scenario: Startup recovery
- **GIVEN** Postgres with both migrations applied and pending intents
- **WHEN** a server starts with the same database and ledger
- **THEN** its bounded recovery worker dispatches pending intents without rescoring fraud
- **AND** memory-only server restarts have no durability guarantee.
