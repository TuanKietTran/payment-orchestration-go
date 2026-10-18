### Requirement: Atomic processing intent

#### Scenario: Allow versus review or deny
- **GIVEN** a pending payment and a fraud result
- **WHEN** the decision is committed
- **THEN** ALLOW atomically stores SCREENED, FraudDecision=ALLOW and a posting intent
- **AND** REVIEW stores SCREENED/FraudDecision=REVIEW and DENY/BLOCK stores FAILED without a posting intent
- **AND** a concurrent later decision cannot overwrite the committed result.

### Requirement: Recoverable ledger completion

#### Scenario: Response lost after ledger commit
- **GIVEN** a persisted posting intent and a ledger implementing key-based idempotency
- **WHEN** the ledger response or local completion is lost
- **THEN** the intent remains recoverable and the payment is not reported settled
- **AND** replay uses the original ledger key and transaction reference
- **AND** completion atomically stores SETTLED plus LedgerTxID and removes the intent.

#### Scenario: Startup recovery
- **GIVEN** Postgres with migrations 001 through 004 applied and pending intents
- **WHEN** a server starts with the same database and ledger
- **THEN** its bounded recovery worker dispatches pending intents without rescoring fraud
- **AND** memory-only server restarts have no durability guarantee.

### Requirement: Recoverable synthetic PSP progress

#### Scenario: Authorization and capture
- **GIVEN** an allowed screened payment with a persisted intent
- **WHEN** the synthetic PSP authorizes then captures it
- **THEN** each stable operation key returns a deterministic UUID reference
- **AND** AUTHORIZED/AuthorizationID then CAPTURED/CaptureID are persisted through legal transitions
- **AND** only capture dispatches the cash ledger transfer; authorization is not counted as another cash movement.

#### Scenario: Ambiguous PSP result
- **GIVEN** a response lost after a PSP operation or failed local progress persistence
- **WHEN** recovery runs
- **THEN** the original PSP key is replayed without rescoring fraud
- **AND** the payment remains non-settled until ledger evidence is persisted.

#### Scenario: Explicit PSP decline
- **GIVEN** a pre-capture payment and a PSP decline with no capture effect
- **WHEN** the decline is committed
- **THEN** the payment becomes FAILED and its intent is removed atomically
- **AND** no cash ledger post is made.
