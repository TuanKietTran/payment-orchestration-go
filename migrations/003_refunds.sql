ALTER TABLE payments ADD COLUMN refunded_amount NUMERIC(20,6) NOT NULL DEFAULT 0;

-- A PENDING row is the durable refund posting intent; a SETTLED row is evidence.
CREATE TABLE payment_refunds (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id),
    idempotency_key TEXT NOT NULL UNIQUE,
    amount NUMERIC(20,6) NOT NULL CHECK (amount > 0),
    currency TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('PENDING', 'SETTLED')),
    ledger_tx_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX payment_refunds_payment ON payment_refunds(payment_id);
CREATE INDEX payment_refunds_pending ON payment_refunds(id) WHERE status = 'PENDING';
