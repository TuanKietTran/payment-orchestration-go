-- Apply after 001_init.sql. Intent and payment state are committed together.
CREATE TABLE payment_outbox (
    payment_id UUID PRIMARY KEY REFERENCES payments(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
