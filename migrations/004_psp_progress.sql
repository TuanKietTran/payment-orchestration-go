ALTER TABLE payments ADD COLUMN fraud_decision TEXT NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN authorization_id TEXT NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN capture_id TEXT NOT NULL DEFAULT '';

-- Previous PROCESSING rows already have an ALLOW posting intent. Replay the
-- synthetic PSP stages before cash dispatch; the existing ledger key is stable.
UPDATE payments SET status='SCREENED',fraud_decision='ALLOW' WHERE status='PROCESSING';
UPDATE payments SET fraud_decision='REVIEW' WHERE status='SCREENED' AND fraud_decision='';
UPDATE payments SET fraud_decision='DENY' WHERE status='FAILED' AND fraud_decision='';
