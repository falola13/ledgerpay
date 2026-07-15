DO $$
  BEGIN 
    CREATE TYPE DirectionType AS ENUM ('debit','credit');
    EXCEPTION
    WHEN duplicate_object THEN 
    NULL;
END $$; 

CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()   
);

CREATE TABLE IF NOT EXISTS wallets (
 id TEXT PRIMARY KEY,
 account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE  ,
 currency TEXT NOT NULL DEFAULT 'USD',
created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id TEXT PRIMARY KEY,
    transfer_id TEXT NOT NULL,
    wallet_id TEXT NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    direction DirectionType NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


INSERT INTO accounts (
    id,
    email,
    created_at,
    updated_at
)
VALUES (
    'acct_system',
    'system@mail.com',
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO wallets (
    id,
    account_id,
    currency,
    created_at,
    updated_at
)
VALUES (
    'wal_system',
    'acct_system',
    'USD',
    now(),
    now()
)
ON CONFLICT (id) DO NOTHING;