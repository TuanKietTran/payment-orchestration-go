CREATE TABLE payments (
    id UUID PRIMARY KEY,
    idempotency_key TEXT UNIQUE NOT NULL,
    source_account TEXT NOT NULL,
    dest_account TEXT NOT NULL,
    amount NUMERIC(20,6) NOT NULL,
    currency TEXT NOT NULL,
    status TEXT NOT NULL,
    ledger_tx_id UUID,
    failure_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_idempotency ON payments(idempotency_key);
