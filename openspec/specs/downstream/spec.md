### Requirement: Bounded transient retries

#### Scenario: Transient failure and duplicate delivery
- **GIVEN** a fraud score or ledger post with a stable replay identity
- **WHEN** transport fails or the dependency returns 408, 429 or 5xx
- **THEN** at most three identical serialized requests are attempted with two-second client timeouts
- **AND** exponential backoff with equal jitter is context-cancellable
- **AND** other HTTP errors and invalid successful JSON are not retried.

### Requirement: Circuit breaker

#### Scenario: Unavailable dependency
- **GIVEN** three exhausted logical operations
- **WHEN** another operation arrives before the five-second cooldown
- **THEN** it fails fast without a dependency request
- **AND** after cooldown exactly one half-open probe is admitted
- **AND** late results from earlier generations cannot close a newer open circuit.
