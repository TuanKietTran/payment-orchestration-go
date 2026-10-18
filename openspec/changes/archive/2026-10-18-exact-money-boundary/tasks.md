# Tasks

- [x] Validate supported currency, minor units, maximum amount and exponent bounds in the pure domain.
- [x] Guard adapter formatting and refund reservations with the same exact boundary.
- [x] Test boundaries, 1000 minor-unit refund conservation cases and extreme exponent fuzz seeds.
- [x] Publish the deliberate supported-money contract tightening.
- [x] All gates pass (Go 1.27.1 darwin/arm64, 2026-10-18): domain 100%, weighted non-cmd 83.30% (788/946). FuzzParseMoneyBounds: 160995 executions/3s, PASS. Evidence: control logs/payments/card4g-{gates,fuzz}.log. Archive this slice.
