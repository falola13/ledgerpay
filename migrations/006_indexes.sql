CREATE INDEX IF NOT EXISTS idx_ledger_entries ON ledger_entries (wallet_id);
CREATE INDEX IF NOT EXISTS idx_outbox_due ON outbox_events (next_retry_at) WHERE status = 'pending';
