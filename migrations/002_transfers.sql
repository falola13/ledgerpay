DO $$
    BEGIN 
        CREATE TYPE TransferStatus AS ENUM ('succeeded','failed');
        EXCEPTION
        WHEN duplicate_object THEN 
        NULL;
    END $$;
    

CREATE TABLE IF NOT EXISTS transfers (
    id TEXT PRIMARY KEY,
    wallet_id TEXT NOT NULL REFERENCES wallets(id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency TEXT NOT NULL DEFAULT 'USD',
    status TransferStatus NOT NULL ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);