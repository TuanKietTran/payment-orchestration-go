# Reject unsupported money before formatting

The HTTP ledger adapter formats two fractional digits, so accepting arbitrary precision/currencies can silently change money. Payments and refunds will reject, never round, unsupported precision. Support USD/EUR/GBP only, two minor-unit digits, positive payment/refund amounts up to 999999999999.99. Bound raw decimal exponents to [-18,12] before any rescaling or comparison to avoid pathological allocation. Numerically redundant fractional zeroes within that bound remain accepted.

Apply the same pure validation at command and downstream adapter boundaries, and include cumulative refund values. Existing numeric JSON input continues to parse directly into decimal (no float). Add extreme-exponent fuzz/property cases. This deliberately tightens the published create contract; consumers using the existing USD/EUR two-decimal fixtures are unchanged. No currency conversion or money rounding is implemented.
