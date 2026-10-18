# Durable partial and full refunds

Reserve a positive refund against a settled payment under the payment row lock. Include all pending and completed reservations in the cumulative cap so concurrent calls cannot refund more than the original amount. A globally unique refund idempotency key replays the original reservation for the same payment/amount; changed payload is a conflict.

The pending refund record is a durable reverse-posting intent. Dispatch uses a stable `refund:<refund UUID>` ledger key with source and destination reversed. Atomically store its ledger reference, increment the completed refund total and set PARTIALLY_REFUNDED or REFUNDED. Recovery scans pending records on startup and periodically, alongside payment intents. Apply migration 003 after 001 and 002. No separate PSP refund claim: the current PSP remains implicit until its dedicated card.
