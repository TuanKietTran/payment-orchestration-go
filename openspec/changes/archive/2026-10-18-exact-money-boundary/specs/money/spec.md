## ADDED Requirements

### Requirement: Exact supported money boundary

#### Scenario: Reject silent rounding and unsupported currency
- **GIVEN** a create, fraud score, PSP operation, ledger post or refund request
- **WHEN** its amount is not exact to two minor-unit digits, exceeds 999999999999.99, or its currency is not USD/EUR/GBP
- **THEN** validation rejects it before downstream formatting or posting
- **AND** positive values with redundant fractional zeroes remain accepted within exponent bounds
- **AND** no foreign exchange conversion or monetary rounding occurs.

#### Scenario: Pathological decimal exponent
- **GIVEN** a parsed decimal with raw exponent outside [-18,12]
- **WHEN** validation runs
- **THEN** it rejects the value before rescaling, comparison or fixed decimal formatting.
