# LedgerPay

[![CI](https://github.com/falola13/ledgerpay/actions/workflows/ci.yml/badge.svg)](https://github.com/falola13/ledgerpay/actions/workflows/ci.yml)

Educational Go + Postgres payments API: double-entry ledger, overdraft-safe charges, idempotent retries, transactional outbox, and signed webhooks with retries / dead letters. **Not production money** — demos and interview vocabulary, not real card rails.

## Architecture

```text
Client
  │  POST /v1/charges (+ Idempotency-Key)
  ▼
API server ──► Postgres
                 ├─ transfers + ledger_entries (source of truth)
                 ├─ idempotency_keys
                 └─ outbox_events (pending → delivered | dead)
                        │
                        ▼
                   Worker ──► signed HTTP POST ──► merchant webhook
                                              (HMAC-SHA256)
```

Compose runs: `postgres`, `migrate`, `server`, `worker`, `testreceiver`.

## Quick start

```powershell
git clone https://github.com/falola13/ledgerpay.git
cd ledgerpay
docker compose up --build
```

```powershell
# health
curl.exe -s http://localhost:8080/v1/ready

# create account (+ wallet in response)
curl.exe -s -X POST http://localhost:8080/v1/accounts -H "Content-Type: application/json" -d "{\"email\":\"you@example.com\"}"

# fund + charge (replace WALLET)
curl.exe -s -X POST http://localhost:8080/v1/wallets/WALLET/fund -H "Content-Type: application/json" -d "{\"amount_cents\":1000}"
curl.exe -s -i -X POST http://localhost:8080/v1/charges -H "Content-Type: application/json" -H "Idempotency-Key: demo-1" -d "{\"wallet_id\":\"WALLET\",\"amount_cents\":500,\"currency\":\"USD\"}"
```

## Demo: idempotent charges

Same key twice → one transfer; second response has `Idempotent-Replay: true`.

```text
{"balance_cents":0,"status":"succeeded","transfer_id":"2515c02f-3a6f-41ad-895d-016b1fe20a3d"}
# second call — same body + Idempotent-Replay: true
{"balance_cents":0,"status":"succeeded","transfer_id":"2515c02f-3a6f-41ad-895d-016b1fe20a3d"}
transfers_for_key = 1
```

## Demo: overdraft & concurrency

Insufficient funds → `402`; failed attempt writes no ledger/transfer rows. Concurrent last-cent charges: one `201`, one `402`, final balance `0` (`SELECT … FOR UPDATE` on the wallet row).

## Demo: webhook retry → dead letter

- Happy path: worker POSTs; testreceiver logs `Webhook Signature OK`; outbox `delivered`.
- Forgery: fake `X-LedgerPay-Signature` → `401` / `signature INVALID`.
- Death: stop `testreceiver` → attempts / `next_retry_at` climb → `dead` → `GET /v1/admin/dead-letters` → manual requeue to `pending` delivers after receiver is back.

## Design decisions

1. **Ledger vs mutable balance** — append-only credit/debit legs; balance is derived. Auditable history; no silent `UPDATE balance`.
2. **Idempotency-Key + stored response** vs amount+time dedupe — one intent, one transfer; retries replay the saved response.
3. **Transactional outbox** vs notify-after-commit — event row commits with the charge so a crash cannot lose the notification.
4. **Modular monolith** vs microservices — one deployable API + worker sharing Postgres until a scaling axis forces a split.

## Performance

**Indexes** (`migrations/006_indexes.sql`):

- `idx_ledger_entries (wallet_id)` — balance SUM on every charge  
- `idx_outbox_due (next_retry_at) WHERE status = 'pending'` — partial index for the worker claim  

**EXPLAIN** balance SUM for one wallet on a ~200k-row table (noise on another wallet):

```text
BEFORE: Parallel Seq Scan on ledger_entries … Execution Time: ~15.7 ms
AFTER:  Index Scan using idx_ledger_entries … Execution Time: ~0.08 ms
```

**Load snapshot** (PowerShell parallel jobs — k6 not installed):

```text
~40 workers × 25 charges = 1000 requests
ok=1000 err=0  p95 ≈ 132 ms  wall ≈ 22 s
(laptop, Docker Desktop, single wallet — FOR UPDATE serializes same-wallet charges)
```

## Observability

- `X-Request-Id` + structured request logs  
- `GET /v1/health` (liveness) vs `GET /v1/ready` (Postgres)  
- `GET /v1/metrics` — request counter/histogram; gauges for outbox pending + dead letters  

## What this is not

No real cards/Stripe, no multi-currency FX, dev-grade webhook secrets, single-region Postgres. Auth on admin dead-letters is a natural next step.

## What I'd add next

1. Authentication on admin endpoints  
2. Deploy + managed Postgres  
3. OpenTelemetry traces across API → worker → webhook  
