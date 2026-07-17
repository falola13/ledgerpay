DO $$
    BEGIN
        CREATE TYPE OutboxStatus AS ENUM ('pending','delivered');
        EXCEPTION
        WHEN duplicate_object THEN 
        NULL;
    END $$;

CREATE TABLE IF NOT EXISTS outbox_events (
    id TEXT PRIMARY KEY,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    status OutboxStatus NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);