# Accounting Module Design

**Date:** 2026-02-11
**Status:** Approved
**Scope:** Migrate Google Sheets accounting system to Kiwari Admin (SvelteKit + Go API + PostgreSQL)

## Background

Kiwari uses a Google Sheets workbook ("DapurBundavNext") as its accounting system. It tracks purchases, sales, reimbursements, and auto-generates P&L and Cash Flow reports via formulas. An Apps Script handles reimbursement batch posting, and an n8n workflow parses WhatsApp messages from employees into reimbursement requests.

**Problems with the current system:**
- App Script posting is fragile and hard to debug
- n8n workflow has 13 nodes with complex JS logic for parsing and item matching — untestable
- GSheet formulas for reports can break silently
- No proper access control (anyone with sheet access sees everything)
- Manual data entry in a spreadsheet is error-prone at scale

**Goal:** Move all accounting functionality to the Kiwari Admin web UI, with data in PostgreSQL and business logic in the Go API.

## Current GSheet Structure

| Sheet | Rows | Purpose |
|-------|------|---------|
| MASTER_ACCOUNT | 32 | Chart of Accounts (Asset, Liability, Equity, Revenue, Expense) |
| MASTER_ITEM | 88 | Inventory items with keyword-based matching and pricing |
| MASTER_CASH_ACCOUNT | 7 | Physical cash wallets and bank accounts |
| CASH_TRANSACTION | ~38k | Main cash journal — all money movements flow here |
| REIMBURSEMENT_REQUEST | ~1k | Employee expense claims (Draft → Ready → Posted) |
| SALES_DAILY_SUMMARY | ~200 | Daily revenue by channel and payment method |
| PROFIT_AND_LOSS | formula | Monthly P&L (Revenue - COGS - Expenses) |
| CASH_FLOW_STATEMENT | formula | Monthly cash in/out by cash account |
| PAYROLL | empty | Planned but unused |
| MATERIAL_USAGE | empty | Planned but unused |
| INVENTORY_LEDGER | empty | Planned but unused |

## Architecture

### High-Level

```
┌─────────────────────────────────────────────────┐
│                  SvelteKit Admin                 │
│  /accounting/*  (new pages alongside POS pages)  │
└────────────────────┬────────────────────────────┘
                     │ server-side fetch (same pattern as POS)
                     ▼
┌─────────────────────────────────────────────────┐
│               Go API (new endpoints)             │
│  /accounting/transactions                        │
│  /accounting/purchases                           │
│  /accounting/reimbursements                      │
│  /accounting/sales-summaries                     │
│  /accounting/payroll                             │
│  /accounting/reports/pnl                         │
│  /accounting/reports/cashflow                    │
│  /accounting/items                               │
│  /accounting/reimbursements/from-whatsapp        │
└────────────────────┬────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────┐
│            PostgreSQL (same DB as POS)            │
│  New tables: acct_*                               │
│  + migration of all GSheet historical data        │
└─────────────────────────────────────────────────┘
```

### Key Architectural Decisions

1. **Same database as POS.** New `acct_*` tables alongside existing POS tables. Single source of truth, transactions can reference POS data (outlet_id FK).

2. **Outlet is a dimension, not a scope.** Unlike POS where data is hard-scoped by outlet_id, accounting operates at the business level. Transactions have an optional outlet_id for filtering/reporting, but endpoints live at `/accounting/*` not `/outlets/{oid}/accounting/*`. This is standard "cost center" accounting for small multi-outlet businesses.

3. **Reports are computed, not stored.** P&L and Cash Flow are SQL aggregations over `acct_cash_transactions`. No report tables needed.

4. **Item matching lives in Go.** Deterministic, unit-testable. Replaces the fragile n8n JS code nodes.

5. **n8n stays as thin relay.** WAHA → n8n (filter + forward) → Go API. All parsing, matching, and formatting logic moves to Go. n8n shrinks from 13 nodes to 5.

6. **Sales flow is hybrid.** POS orders auto-aggregate into daily summaries. Non-POS channels (GoFood, ShopeeFood) entered manually.

7. **Full historical migration in Phase 1.** All ~38k transactions imported upfront so reports work from day one.

### Scale Trigger (future)

At current volume (~50-100 transactions/day), PostgreSQL handles report queries on 38k+ rows with proper indexes. When data exceeds ~500k rows and report queries slow beyond 2s:

- **Step 1:** Add `acct_monthly_summaries` materialized view (pre-computed monthly totals). ~1 hour task.
- **Step 2:** Partition `acct_cash_transactions` by month. Zero code change.

Not built now — YAGNI.

## Data Model

### Master Data

```sql
-- Chart of Accounts (from MASTER_ACCOUNT, ~32 rows)
CREATE TABLE acct_accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_code  VARCHAR(10) UNIQUE NOT NULL,  -- "1000", "1200", "6000"
    account_name  VARCHAR(100) NOT NULL,         -- "Cash on Hand"
    account_type  VARCHAR(20) NOT NULL,          -- Asset|Liability|Equity|Revenue|Expense
    line_type     VARCHAR(20) NOT NULL,          -- ASSET|INVENTORY|EXPENSE|SALES|COGS|LIABILITY|CAPITAL|DRAWING
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMPTZ DEFAULT now()
);

-- Inventory Items (from MASTER_ITEM, ~88 rows)
CREATE TABLE acct_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_code      VARCHAR(20) UNIQUE NOT NULL,  -- "ITEM0001"
    item_name      VARCHAR(100) NOT NULL,
    item_category  VARCHAR(30) NOT NULL,          -- Raw Material|Packaging|Consumable
    unit           VARCHAR(10) NOT NULL,           -- kg|pcs|pack|liter|box
    is_inventory   BOOLEAN DEFAULT true,
    is_active      BOOLEAN DEFAULT true,
    average_price  DECIMAL(12,2),
    last_price     DECIMAL(12,2),
    for_hpp        DECIMAL(12,2),                  -- for COGS calculation
    keywords       TEXT NOT NULL,                   -- CSV: "cabe,merah,tanjung,chili"
    created_at     TIMESTAMPTZ DEFAULT now()
);

-- Cash/Bank Accounts (from MASTER_CASH_ACCOUNT, ~7 rows)
CREATE TABLE acct_cash_accounts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cash_account_code   VARCHAR(20) UNIQUE NOT NULL,  -- "CASH001", "BANK001"
    cash_account_name   VARCHAR(100) NOT NULL,
    bank_name           VARCHAR(50),
    ownership           VARCHAR(20) NOT NULL,          -- Business|Personal
    is_active           BOOLEAN DEFAULT true,
    created_at          TIMESTAMPTZ DEFAULT now()
);
```

**Design notes:**
- UUID as PK (consistent with POS tables), human-readable codes as unique business keys
- Display strings (`"1200 - Inventory - Raw Materials"`) computed in queries: `account_code || ' - ' || account_name`. Not stored.
- Keywords stay as CSV text — Go code tokenizes for matching. No junction table needed at 88 items.

### Transaction Tables

```sql
-- Main Cash Journal (from CASH_TRANSACTION, ~38k rows)
CREATE TABLE acct_cash_transactions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_code        VARCHAR(20) UNIQUE NOT NULL,  -- "PCS000001" (auto-sequenced)
    transaction_date        DATE NOT NULL,
    item_id                 UUID REFERENCES acct_items(id),       -- nullable
    description             TEXT NOT NULL,
    quantity                DECIMAL(12,4) NOT NULL DEFAULT 1,
    unit_price              DECIMAL(12,2) NOT NULL,
    amount                  DECIMAL(12,2) NOT NULL,               -- qty * unit_price
    line_type               VARCHAR(20) NOT NULL,                  -- denormalized from account
    account_id              UUID NOT NULL REFERENCES acct_accounts(id),
    cash_account_id         UUID REFERENCES acct_cash_accounts(id), -- nullable (reimbursement expense legs)
    outlet_id               UUID REFERENCES outlets(id),            -- nullable (business-level txns)
    reimbursement_batch_id  VARCHAR(30),                            -- links to reimbursement batch
    created_at              TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_cash_tx_date ON acct_cash_transactions(transaction_date);
CREATE INDEX idx_cash_tx_line_type ON acct_cash_transactions(line_type);
CREATE INDEX idx_cash_tx_account ON acct_cash_transactions(account_id);
CREATE INDEX idx_cash_tx_cash_account ON acct_cash_transactions(cash_account_id);

-- Reimbursement Requests (from REIMBURSEMENT_REQUEST, ~1k rows)
CREATE TABLE acct_reimbursement_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        VARCHAR(30),                                  -- assigned when batched
    expense_date    DATE NOT NULL,
    item_id         UUID REFERENCES acct_items(id),               -- nullable (unmatched items)
    description     TEXT NOT NULL,
    qty             DECIMAL(12,4) NOT NULL DEFAULT 1,
    unit_price      DECIMAL(12,2) NOT NULL,
    amount          DECIMAL(12,2) NOT NULL,
    line_type       VARCHAR(20) NOT NULL,                          -- INVENTORY|EXPENSE
    account_id      UUID NOT NULL REFERENCES acct_accounts(id),
    status          VARCHAR(10) NOT NULL DEFAULT 'Draft',          -- Draft|Ready|Posted
    requester       VARCHAR(100) NOT NULL,
    receipt_link    TEXT,
    posted_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

-- Sales Daily Summary (from SALES_DAILY_SUMMARY)
CREATE TABLE acct_sales_daily_summaries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sales_date          DATE NOT NULL,
    channel             VARCHAR(30) NOT NULL,     -- Take Away|Dine In|GoFood|ShopeeFood|Catering
    payment_method      VARCHAR(30) NOT NULL,     -- Cash|QRIS|Transfer|Gopay|Multipayment
    gross_sales         DECIMAL(12,2) NOT NULL,
    discount_amount     DECIMAL(12,2) NOT NULL DEFAULT 0,
    net_sales           DECIMAL(12,2) NOT NULL,
    cash_account_id     UUID NOT NULL REFERENCES acct_cash_accounts(id),
    outlet_id           UUID REFERENCES outlets(id),
    source              VARCHAR(10) NOT NULL DEFAULT 'manual',  -- 'pos'|'manual'
    created_at          TIMESTAMPTZ DEFAULT now(),
    UNIQUE(sales_date, channel, payment_method, outlet_id)
);

-- Payroll Entries (from PAYROLL sheet, currently empty)
CREATE TABLE acct_payroll_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payroll_date        DATE NOT NULL,
    period_type         VARCHAR(10) NOT NULL,     -- Daily|Weekly|Monthly
    period_ref          VARCHAR(30),              -- "2025-W03" or "2025-01"
    employee_name       VARCHAR(100) NOT NULL,
    gross_pay           DECIMAL(12,2) NOT NULL,
    payment_method      VARCHAR(20) NOT NULL,
    cash_account_id     UUID NOT NULL REFERENCES acct_cash_accounts(id),
    outlet_id           UUID REFERENCES outlets(id),
    posted_at           TIMESTAMPTZ,              -- when posted to cash_transactions
    created_at          TIMESTAMPTZ DEFAULT now()
);
```

### Reports (no tables — SQL queries)

```sql
-- P&L: revenue minus expenses, grouped by month
SELECT
    date_trunc('month', transaction_date) AS period,
    SUM(CASE WHEN line_type = 'SALES' THEN amount END) AS net_sales,
    SUM(CASE WHEN line_type = 'COGS' THEN amount END) AS cogs,
    SUM(CASE WHEN line_type = 'EXPENSE' THEN amount END) AS expenses
FROM acct_cash_transactions
GROUP BY 1 ORDER BY 1;

-- Cash Flow: cash in vs cash out by cash account by month
SELECT
    date_trunc('month', transaction_date) AS period,
    cash_account_id,
    SUM(CASE WHEN line_type IN ('SALES','CAPITAL') THEN amount ELSE 0 END) AS cash_in,
    SUM(CASE WHEN line_type IN ('INVENTORY','EXPENSE','COGS','DRAWING') THEN amount ELSE 0 END) AS cash_out
FROM acct_cash_transactions
WHERE cash_account_id IS NOT NULL
GROUP BY 1, 2 ORDER BY 1;
```

### What was dropped vs GSheet

| GSheet column | Decision | Why |
|---|---|---|
| `account_display`, `cash_account_display`, `item_display` | Computed in API/query | Derived from code + name |
| `month` column | `date_trunc('month', transaction_date)` | Derived from date |
| `account_code` on transactions | Replaced by `account_id` FK | Proper relational model |
| `line_type_backup` | Dropped | Migration artifact |
| `MASTER_ITEM_FIXED` sheet | Dropped | Backup/migration sheet |
| `MATERIAL_USAGE` sheet | Dropped | Empty, unused |
| `INVENTORY_LEDGER` sheet | Dropped | Empty, unused |

## Features & Pages

### Navigation

New "Keuangan" section in admin sidebar:

```
Dashboard
Menu
Orders
Customers
Reports (existing POS sales reports)
──────────
Keuangan
  ├─ Ringkasan      /accounting              (overview dashboard)
  ├─ Pembelian       /accounting/purchases    (purchase entry)
  ├─ Penjualan       /accounting/sales        (sales summary)
  ├─ Reimburse       /accounting/reimbursements (reimbursement workflow)
  ├─ Gaji            /accounting/payroll      (payroll)
  ├─ Jurnal          /accounting/transactions (full ledger)
  ├─ Laporan         /accounting/reports      (P&L + Cash Flow)
  └─ Master Data     /accounting/master       (accounts, items, cash accounts)
```

### Access Control

- **OWNER:** Full access to all accounting pages
- **Employees:** Reimbursement submission only (via WhatsApp or limited admin access)
- **MANAGER/CASHIER/KITCHEN:** No accounting access

### Page Details

#### 1. Ringkasan — `/accounting`

Overview dashboard. Read-only.

- Cash balance cards (current balance per cash account)
- This month P&L mini-summary (revenue, COGS, gross profit, expenses, net profit)
- Pending reimbursements badge (count + total amount of Draft + Ready)
- Recent 10 transactions

#### 2. Pembelian — `/accounting/purchases`

Daily purchase recording with item matching. Replaces manual CASH_TRANSACTION entry.

- Date picker, cash account selector, outlet selector
- Multi-line item entry with keyword-based item matching (autocomplete)
- Price auto-fill from fallback chain: average_price → last_price → last transaction price
- User can override price
- "Simpan" creates `acct_cash_transactions` rows (line_type=INVENTORY, account=1200)

#### 3. Penjualan — `/accounting/sales`

Daily sales summary. Hybrid: auto from POS + manual for non-POS channels.

- Monthly list view with source indicator (POS vs manual)
- POS rows: auto-aggregated from completed orders, read-only
- Manual rows: editable, for GoFood/ShopeeFood/catering
- Sales → cash_transaction posting (DR Cash, CR Sales Revenue)

#### 4. Reimburse — `/accounting/reimbursements`

Reimbursement workflow. Replaces App Script + GSheet manual review.

- List view grouped by batch, filterable by status and requester
- **Draft:** Items may be unmatched/ambiguous. Owner reviews and matches.
- **Ready:** All items matched and priced. Ready to post.
- **Post Batch:** Dialog to pick payment date + cash account. Creates two-leg entries:
  - Expense leg: DR Expense/Inventory account, CR Reimbursement Payable (2101)
  - Payment leg: DR Reimbursement Payable (2101), CR Cash account
- Idempotent: cannot post same batch twice

#### 5. Gaji — `/accounting/payroll`

Payroll entry form. No complex payroll calculation.

- Date, period type (Daily/Weekly/Monthly), period reference, outlet, cash account
- Multi-employee entry (name + gross pay + payment method)
- Creates `acct_cash_transactions` (DR Payroll Expense 6090, CR Cash)

#### 6. Jurnal — `/accounting/transactions`

Full ledger view. The source of truth.

- Filterable: date range, line_type, account, cash_account, outlet, search text
- Every entry from purchases, sales, reimbursements, payroll appears here
- Supports manual entry for one-off transactions (bank transfers, owner drawings, equipment)
- Auto-generated entries are read-only

#### 7. Laporan — `/accounting/reports`

Financial reports. Two tabs.

**Tab 1: Laba Rugi (P&L)**
- Monthly columns (matches GSheet P&L layout)
- Rows: Net Sales, COGS, Gross Profit, Operating Expenses (itemized by account), Net Profit, margins %
- Optional outlet filter, date range
- CSV export

**Tab 2: Arus Kas (Cash Flow)**
- Monthly columns (matches GSheet Cash Flow layout)
- Cash In by source account, Cash Out by source account, Net Cash Flow
- Broken down by cash account
- Optional outlet filter, date range
- CSV export

#### 8. Master Data — `/accounting/master`

CRUD for reference data. Three sub-tabs: Akun (accounts), Item (items), Kas (cash accounts).

- Simple table + add/edit forms
- Items page includes keywords field for the matching engine
- Rarely changed after initial setup

## Item Matching Engine

Lives in Go: `api/internal/accounting/matcher/`

### Pipeline

```
Raw text input (e.g., "cabe merah tanjung 5kg")
  → Normalize: lowercase, strip punctuation
  → Tokenize: ["cabe", "merah", "tanjung", "5", "kg"]
  → Extract quantity + unit: qty=5, unit=kg
  → Match remaining tokens against acct_items.keywords
  → Score each candidate: keyword intersection count
      - Color/variant keywords (merah/hijau, tanjung/kriting): weight=5
      - Regular keywords: weight=1
  → Hard filter: if input has color/variant, candidate MUST have it
  → Return: matched | ambiguous | unmatched
```

### Match Results

| Result | Condition | Behavior |
|--------|-----------|----------|
| **Matched** | Single high-score candidate | Auto-fill item_id, line_type, account, price |
| **Ambiguous** | Tied top-score candidates differing by color/variant user didn't specify | Flag for owner to pick |
| **Unmatched** | No candidates score above 0 | Flag for owner to manually assign |

### Price Resolution (fallback chain)

1. `acct_items.average_price` (if set)
2. `acct_items.last_price` (if set)
3. Most recent `acct_cash_transactions.unit_price` for this item_id
4. Manual entry by user

## WhatsApp Integration

### Current Flow (n8n, 13 nodes)

```
WAHA Trigger → IF: Is Target Group → IF: Is Command
  → Parse Reimbursement Message (JS) → IF: Has Items → Deduplicate Messages
    → Get MASTER_ITEM (GSheet) → Match Items to Master (JS)
      → Split Items → Append to REIMBURSEMENT_REQUEST (GSheet)
        → Aggregate Results → Format Confirmation (JS) → Send WhatsApp Reply
```

### New Flow (n8n, 5 nodes)

```
WAHA Trigger
  → IF: Is Target Group
    → IF: Is Command
      → Deduplicate Messages
        → HTTP Request: POST /accounting/reimbursements/from-whatsapp
            body: { sender_phone, sender_name, message_text, chat_id }
          → Send WhatsApp Reply (message from API response body)
```

All parsing, matching, and formatting moves to Go. n8n just does: filter → forward → reply.

### Message Parser (Go)

Lives in: `api/internal/accounting/parser/`

Input format (from WhatsApp):
```
20 jan
cabe merah tanjung 5kg 500k
beras sania 20kg 300k
minyak 2L 72k
```

Rules:
1. First line with just a date → `expense_date` (parse Indonesian: "20 jan" → 2026-01-20)
2. Each subsequent line → one item: `{description, qty, unit, total_price}`
3. Price shortcuts: `500k` → 500,000 / `1.5jt` → 1,500,000
4. Unit extraction: `5kg`, `2L`, `3pcs`, `1iket`, `10bks`
5. Remaining tokens → item description (fed to matching engine)

### API Endpoint

```
POST /accounting/reimbursements/from-whatsapp

Request:
{
  "sender_phone": "+628123456789",
  "sender_name": "Hamidah",
  "message_text": "20 jan\ncabe merah tanjung 5kg 500k\nberas 20kg 300k",
  "chat_id": "120363421848364675@g.us"
}

Response:
{
  "reply_message": "✅ Reimbursement Draft (20 Jan):\n\n📦 Cocok (2):\n• ITEM0012 - Cabe Merah Tanjung 5kg - Rp500K\n• ITEM0011 - Beras 20kg - Rp300K\n\nTotal: Rp800K (2 item)\nStatus: Draft\n👤 Hamidah",
  "items_created": 2,
  "items_matched": 2,
  "items_ambiguous": 0,
  "items_unmatched": 0
}
```

## Phasing

Each phase delivers a usable feature. GSheet sheets are retired incrementally.

### Phase 1: Foundation + Full Migration + Purchase Entry

**Build:**
- DB migrations: all `acct_*` tables
- Full data migration script:
  - MASTER_ACCOUNT → acct_accounts (32 rows)
  - MASTER_ITEM → acct_items (88 rows)
  - MASTER_CASH_ACCOUNT → acct_cash_accounts (7 rows)
  - CASH_TRANSACTION → acct_cash_transactions (~38k rows)
  - REIMBURSEMENT_REQUEST → acct_reimbursement_requests (~1k rows)
  - SALES_DAILY_SUMMARY → acct_sales_daily_summaries (~200 rows)
- Reconciliation check: verify P&L totals match GSheet
- Go CRUD endpoints for master data
- Item matching engine (`internal/accounting/matcher/`)
- Purchase entry endpoints
- Admin pages: Pembelian + Master Data

**Switch point:** All historical data in PostgreSQL. Stop entering purchases in GSheet. Master data managed via admin UI.

**GSheet sheets retired:** MASTER_ACCOUNT, MASTER_ITEM, MASTER_CASH_ACCOUNT (master data)

### Phase 2: Reimbursement Workflow + WhatsApp

**Build:**
- Go reimbursement endpoints (CRUD + batch posting)
- Go WhatsApp message parser (`internal/accounting/parser/`)
- `POST /accounting/reimbursements/from-whatsapp` endpoint
- Admin page: Reimburse (list, review, match, batch post)
- Slim down n8n workflow (13 nodes → 5 nodes)

**Switch point:** n8n points to Go API instead of GSheet. Reimbursement review and posting in admin UI.

**GSheet sheets retired:** REIMBURSEMENT_REQUEST

### Phase 3: Reports

**Build:**
- P&L report endpoint (SQL aggregation)
- Cash Flow report endpoint
- Admin page: Laporan (P&L + Cash Flow tabs, CSV export)
- Ringkasan dashboard

**Switch point:** Financial reports come from admin UI, not GSheet formulas.

**GSheet sheets retired:** PROFIT_AND_LOSS, CASH_FLOW_STATEMENT

### Phase 4: Sales + Payroll + Ledger

**Build:**
- Auto-aggregate POS orders into daily summaries endpoint
- Manual sales entry endpoint
- Sales → cash_transaction posting logic
- Payroll entry endpoint + posting
- Full ledger view with filters + manual entry
- Admin pages: Penjualan + Gaji + Jurnal

**Switch point:** GSheet fully retired. Becomes read-only archive.

**GSheet sheets retired:** SALES_DAILY_SUMMARY, PAYROLL, CASH_TRANSACTION (all remaining)

### Phase Summary

| Phase | Delivers | GSheet Retired |
|-------|----------|----------------|
| P1 | Migration + purchases + item matching + master data | Master data sheets |
| P2 | Reimbursement workflow + WhatsApp integration | REIMBURSEMENT_REQUEST |
| P3 | P&L + Cash Flow reports + dashboard | Report sheets |
| P4 | Sales + payroll + full ledger | All remaining sheets |

## Go Package Structure

```
api/internal/accounting/
├── matcher/           # Item matching engine
│   ├── matcher.go     # Score, match, resolve functions
│   └── matcher_test.go
├── parser/            # WhatsApp message parser
│   ├── parser.go      # Date, item line, price/qty extraction
│   └── parser_test.go
├── handler/           # HTTP handlers
│   ├── transaction.go
│   ├── purchase.go
│   ├── reimbursement.go
│   ├── sales.go
│   ├── payroll.go
│   ├── report.go
│   ├── master.go
│   └── whatsapp.go
└── store/             # Database interfaces (consumer-defines-interface)
```

Follows existing Kiwari patterns: consumer-defines-interface for stores, `RegisterRoutes(r chi.Router)`, mock tests with httptest.

## Migration Script

One-time script to import GSheet data. Can be a Go CLI tool or Python script.

**Key mapping challenges:**
- `account_display` strings ("1200 - Inventory - Raw Materials") → lookup `acct_accounts.id` by `account_code` extracted from the display string
- `cash_account_display` → same pattern, lookup by code prefix
- `item_display` → lookup `acct_items.id` by `item_code`
- `outlet` text ("Rumah", "Kedai Gerlong") → lookup `outlets.id` by name
- `transaction_code` preserved as-is (PCS000001 sequence continues)

**Reconciliation:** After migration, run P&L query and compare totals against GSheet PROFIT_AND_LOSS sheet per month. All numbers must match.
