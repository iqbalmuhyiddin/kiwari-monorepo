# Accounting Module Phase 1 — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the foundation for the accounting module: database tables, data migration, item matching engine, master data CRUD, and purchase entry — all powered by PostgreSQL + Go API + SvelteKit admin.

**Architecture:** New `acct_*` tables in the existing POS database. Go handlers under `api/internal/accounting/` with consumer-defines-interface pattern. SvelteKit pages under `admin/src/routes/(app)/accounting/`. Accounting endpoints are NOT outlet-scoped — they live at `/accounting/*` with OWNER-only access.

**Tech Stack:** Go 1.22+ (Chi, sqlc, pgx/v5), PostgreSQL 16, SvelteKit 2 (Svelte 5, Tailwind CSS 4), shopspring/decimal for money.

**Design Doc:** `docs/plans/2026-02-11-accounting-module-design.md`

---

## Codebase Conventions Reference

Before implementing any task, understand these existing patterns:

### Go API Patterns
- **Package location:** Handlers live in `api/internal/handler/`. New accounting code lives in `api/internal/accounting/` (separate from POS handlers).
- **Consumer-defines-interface:** Each handler file defines its own store interface (e.g., `CategoryStore`) satisfied by `*database.Queries`. See `api/internal/handler/categories.go:20-25`.
- **Handler struct:** `type FooHandler struct { store FooStore }` + `NewFooHandler(store FooStore)` constructor.
- **RegisterRoutes:** `func (h *FooHandler) RegisterRoutes(r chi.Router)` — registers all routes. See `api/internal/handler/categories.go:39-44`.
- **Request/Response types:** Defined in same file. Response types have explicit `json` tags. Money is `string` (decimal formatted). UUIDs are `uuid.UUID`. Nullable fields are pointers.
- **writeJSON helper:** `writeJSON(w, statusCode, payload)` — defined in `api/internal/handler/auth.go:226-232`.
- **Error pattern:** `pgx.ErrNoRows` → 404, `pgconn.PgError` code `23505` → 409, validation → 400, everything else → 500 with `log.Printf`.
- **pgtype.Text:** Used for nullable strings. `pgtype.Text{String: val, Valid: true}` for non-empty, zero value for null.
- **sqlc:** Queries in `api/queries/*.sql` with `-- name: FunctionName :one/:many/:exec` comments. Generated to `api/internal/database/`. Config in `api/sqlc.yaml`.
- **Migrations:** `api/migrations/NNNNNN_name.up.sql` / `.down.sql`. Next migration number: `000003`.
- **Router wiring:** In `api/internal/router/router.go`. Protected routes use `mw.Authenticate(cfg.JWTSecret)`. Owner-only routes use `mw.RequireRole("OWNER")`.
- **Tests:** Mock store in test file, `httptest.NewRecorder()`, `doRequest(t, router, method, path, body)` helper in `users_test.go`. Each handler has its own `_test.go` file in the same package.
- **decimal:** Import `github.com/shopspring/decimal`. Use `decimal.NewFromString(val)` to parse, `.StringFixed(2)` to format.

### SvelteKit Admin Patterns
- **Route structure:** `admin/src/routes/(app)/section/+page.svelte` + `+page.server.ts`. The `(app)` group has auth guard in layout.
- **Server load:** `+page.server.ts` calls `apiRequest<T>(path, { accessToken })` from `$lib/server/api`. Returns data to page.
- **API client:** `apiRequest<T>()` from `admin/src/lib/server/api.ts`. Returns `{ ok: true, data: T }` or `{ ok: false, status, message }`.
- **Auth:** `locals.user` from layout guard. `cookies.get('access_token')` for API calls. User has `{ id, outlet_id, full_name, email, role }`.
- **Types:** Defined in `admin/src/lib/types/api.ts`. All IDs are `string`. Money amounts are `string`.
- **Components:** In `admin/src/lib/components/`. Svelte 5 with `$props()`, `$state()`, `$derived()`.
- **Sidebar:** `admin/src/lib/components/Sidebar.svelte` — `navItems` array with `{ label, href, icon, roles? }`. Role-based visibility.
- **Styling:** Scoped `<style>` blocks. CSS variables: `--color-primary`, `--color-text-primary`, `--color-text-secondary`, `--color-border`, `--color-surface`, `--color-bg`, `--color-error`. Radii: `--radius-chip` (8px), `--radius-button` (10px), `--radius-card` (12px).
- **Formatting:** `formatRupiah()` from `$lib/utils/format`.

### Commands
```bash
# From api/ directory:
cd api && go test ./internal/accounting/... -v           # Run accounting tests
cd api && go test ./internal/handler/ -v                 # Run handler tests
export PATH=$PATH:~/go/bin && sqlc generate              # Regenerate sqlc (from api/)
export PATH=$PATH:~/go/bin && migrate -path migrations/ -database "$DATABASE_URL" up  # Run migrations

# From admin/ directory:
pnpm dev                                                  # Dev server on :5173
pnpm build                                                # Type-check + build
```

---

## Task 1: Database Migration — Create acct_* Tables

**Files:**
- Create: `api/migrations/000003_accounting_tables.up.sql`
- Create: `api/migrations/000003_accounting_tables.down.sql`

**Step 1: Write the up migration**

Create `api/migrations/000003_accounting_tables.up.sql`:

```sql
-- Accounting Module: master data + transaction tables
-- See docs/plans/2026-02-11-accounting-module-design.md

-- Chart of Accounts (~32 rows)
CREATE TABLE acct_accounts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_code  VARCHAR(10) UNIQUE NOT NULL,
    account_name  VARCHAR(100) NOT NULL,
    account_type  VARCHAR(20) NOT NULL,
    line_type     VARCHAR(20) NOT NULL,
    is_active     BOOLEAN DEFAULT true,
    created_at    TIMESTAMPTZ DEFAULT now()
);

ALTER TABLE acct_accounts ADD CONSTRAINT chk_acct_accounts_type
  CHECK (account_type IN ('Asset', 'Liability', 'Equity', 'Revenue', 'Expense'));

ALTER TABLE acct_accounts ADD CONSTRAINT chk_acct_accounts_line_type
  CHECK (line_type IN ('ASSET', 'INVENTORY', 'EXPENSE', 'SALES', 'COGS', 'LIABILITY', 'CAPITAL', 'DRAWING'));

-- Inventory Items (~88 rows)
CREATE TABLE acct_items (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_code      VARCHAR(20) UNIQUE NOT NULL,
    item_name      VARCHAR(100) NOT NULL,
    item_category  VARCHAR(30) NOT NULL,
    unit           VARCHAR(10) NOT NULL,
    is_inventory   BOOLEAN DEFAULT true,
    is_active      BOOLEAN DEFAULT true,
    average_price  DECIMAL(12,2),
    last_price     DECIMAL(12,2),
    for_hpp        DECIMAL(12,2),
    keywords       TEXT NOT NULL,
    created_at     TIMESTAMPTZ DEFAULT now()
);

ALTER TABLE acct_items ADD CONSTRAINT chk_acct_items_category
  CHECK (item_category IN ('Raw Material', 'Packaging', 'Consumable'));

-- Cash/Bank Accounts (~7 rows)
CREATE TABLE acct_cash_accounts (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cash_account_code   VARCHAR(20) UNIQUE NOT NULL,
    cash_account_name   VARCHAR(100) NOT NULL,
    bank_name           VARCHAR(50),
    ownership           VARCHAR(20) NOT NULL,
    is_active           BOOLEAN DEFAULT true,
    created_at          TIMESTAMPTZ DEFAULT now()
);

ALTER TABLE acct_cash_accounts ADD CONSTRAINT chk_acct_cash_accounts_ownership
  CHECK (ownership IN ('Business', 'Personal'));

-- Main Cash Journal (~38k rows)
CREATE TABLE acct_cash_transactions (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_code        VARCHAR(20) UNIQUE NOT NULL,
    transaction_date        DATE NOT NULL,
    item_id                 UUID REFERENCES acct_items(id),
    description             TEXT NOT NULL,
    quantity                DECIMAL(12,4) NOT NULL DEFAULT 1,
    unit_price              DECIMAL(12,2) NOT NULL,
    amount                  DECIMAL(12,2) NOT NULL,
    line_type               VARCHAR(20) NOT NULL,
    account_id              UUID NOT NULL REFERENCES acct_accounts(id),
    cash_account_id         UUID REFERENCES acct_cash_accounts(id),
    outlet_id               UUID REFERENCES outlets(id),
    reimbursement_batch_id  VARCHAR(30),
    created_at              TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_cash_tx_date ON acct_cash_transactions(transaction_date);
CREATE INDEX idx_cash_tx_line_type ON acct_cash_transactions(line_type);
CREATE INDEX idx_cash_tx_account ON acct_cash_transactions(account_id);
CREATE INDEX idx_cash_tx_cash_account ON acct_cash_transactions(cash_account_id);

-- Reimbursement Requests (~1k rows)
CREATE TABLE acct_reimbursement_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        VARCHAR(30),
    expense_date    DATE NOT NULL,
    item_id         UUID REFERENCES acct_items(id),
    description     TEXT NOT NULL,
    qty             DECIMAL(12,4) NOT NULL DEFAULT 1,
    unit_price      DECIMAL(12,2) NOT NULL,
    amount          DECIMAL(12,2) NOT NULL,
    line_type       VARCHAR(20) NOT NULL,
    account_id      UUID NOT NULL REFERENCES acct_accounts(id),
    status          VARCHAR(10) NOT NULL DEFAULT 'Draft',
    requester       VARCHAR(100) NOT NULL,
    receipt_link    TEXT,
    posted_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT now()
);

ALTER TABLE acct_reimbursement_requests ADD CONSTRAINT chk_acct_reimb_status
  CHECK (status IN ('Draft', 'Ready', 'Posted'));

CREATE INDEX idx_reimb_status ON acct_reimbursement_requests(status);
CREATE INDEX idx_reimb_batch ON acct_reimbursement_requests(batch_id);

-- Sales Daily Summary
CREATE TABLE acct_sales_daily_summaries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sales_date          DATE NOT NULL,
    channel             VARCHAR(30) NOT NULL,
    payment_method      VARCHAR(30) NOT NULL,
    gross_sales         DECIMAL(12,2) NOT NULL,
    discount_amount     DECIMAL(12,2) NOT NULL DEFAULT 0,
    net_sales           DECIMAL(12,2) NOT NULL,
    cash_account_id     UUID NOT NULL REFERENCES acct_cash_accounts(id),
    outlet_id           UUID REFERENCES outlets(id),
    source              VARCHAR(10) NOT NULL DEFAULT 'manual',
    created_at          TIMESTAMPTZ DEFAULT now(),
    UNIQUE(sales_date, channel, payment_method, outlet_id)
);

ALTER TABLE acct_sales_daily_summaries ADD CONSTRAINT chk_acct_sales_source
  CHECK (source IN ('pos', 'manual'));

-- Payroll Entries
CREATE TABLE acct_payroll_entries (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payroll_date        DATE NOT NULL,
    period_type         VARCHAR(10) NOT NULL,
    period_ref          VARCHAR(30),
    employee_name       VARCHAR(100) NOT NULL,
    gross_pay           DECIMAL(12,2) NOT NULL,
    payment_method      VARCHAR(20) NOT NULL,
    cash_account_id     UUID NOT NULL REFERENCES acct_cash_accounts(id),
    outlet_id           UUID REFERENCES outlets(id),
    posted_at           TIMESTAMPTZ,
    created_at          TIMESTAMPTZ DEFAULT now()
);

ALTER TABLE acct_payroll_entries ADD CONSTRAINT chk_acct_payroll_period
  CHECK (period_type IN ('Daily', 'Weekly', 'Monthly'));
```

**Step 2: Write the down migration**

Create `api/migrations/000003_accounting_tables.down.sql`:

```sql
DROP TABLE IF EXISTS acct_payroll_entries;
DROP TABLE IF EXISTS acct_sales_daily_summaries;
DROP TABLE IF EXISTS acct_reimbursement_requests;
DROP TABLE IF EXISTS acct_cash_transactions;
DROP TABLE IF EXISTS acct_cash_accounts;
DROP TABLE IF EXISTS acct_items;
DROP TABLE IF EXISTS acct_accounts;
```

**Step 3: Verify migration syntax**

Run: `cd api && export PATH=$PATH:~/go/bin && migrate -path migrations/ -database "$DATABASE_URL" up`

Expected: Migration applies successfully. If no local DB, verify SQL syntax by checking the file compiles with `psql`.

**Step 4: Regenerate sqlc**

Run: `cd api && export PATH=$PATH:~/go/bin && sqlc generate`

Expected: sqlc picks up new tables. New types appear in `api/internal/database/models.go`.

**Step 5: Commit**

```bash
git add api/migrations/000003_accounting_tables.up.sql api/migrations/000003_accounting_tables.down.sql
git commit -m "feat(accounting): add acct_* database tables for accounting module"
```

---

## Task 2: sqlc Queries — Master Data CRUD

**Files:**
- Create: `api/queries/acct_accounts.sql`
- Create: `api/queries/acct_items.sql`
- Create: `api/queries/acct_cash_accounts.sql`

**Step 1: Write acct_accounts queries**

Create `api/queries/acct_accounts.sql`:

```sql
-- name: ListAcctAccounts :many
SELECT * FROM acct_accounts WHERE is_active = true ORDER BY account_code;

-- name: GetAcctAccount :one
SELECT * FROM acct_accounts WHERE id = $1 AND is_active = true;

-- name: CreateAcctAccount :one
INSERT INTO acct_accounts (account_code, account_name, account_type, line_type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateAcctAccount :one
UPDATE acct_accounts
SET account_name = $2, account_type = $3, line_type = $4
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: SoftDeleteAcctAccount :one
UPDATE acct_accounts SET is_active = false WHERE id = $1 AND is_active = true RETURNING id;
```

**Step 2: Write acct_items queries**

Create `api/queries/acct_items.sql`:

```sql
-- name: ListAcctItems :many
SELECT * FROM acct_items WHERE is_active = true ORDER BY item_code;

-- name: GetAcctItem :one
SELECT * FROM acct_items WHERE id = $1 AND is_active = true;

-- name: CreateAcctItem :one
INSERT INTO acct_items (item_code, item_name, item_category, unit, is_inventory, average_price, last_price, for_hpp, keywords)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: UpdateAcctItem :one
UPDATE acct_items
SET item_name = $2, item_category = $3, unit = $4, is_inventory = $5,
    average_price = $6, last_price = $7, for_hpp = $8, keywords = $9
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: SoftDeleteAcctItem :one
UPDATE acct_items SET is_active = false WHERE id = $1 AND is_active = true RETURNING id;

-- name: UpdateAcctItemLastPrice :exec
UPDATE acct_items SET last_price = $2 WHERE id = $1;
```

**Step 3: Write acct_cash_accounts queries**

Create `api/queries/acct_cash_accounts.sql`:

```sql
-- name: ListAcctCashAccounts :many
SELECT * FROM acct_cash_accounts WHERE is_active = true ORDER BY cash_account_code;

-- name: GetAcctCashAccount :one
SELECT * FROM acct_cash_accounts WHERE id = $1 AND is_active = true;

-- name: CreateAcctCashAccount :one
INSERT INTO acct_cash_accounts (cash_account_code, cash_account_name, bank_name, ownership)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateAcctCashAccount :one
UPDATE acct_cash_accounts
SET cash_account_name = $2, bank_name = $3, ownership = $4
WHERE id = $1 AND is_active = true
RETURNING *;

-- name: SoftDeleteAcctCashAccount :one
UPDATE acct_cash_accounts SET is_active = false WHERE id = $1 AND is_active = true RETURNING id;
```

**Step 4: Regenerate sqlc**

Run: `cd api && export PATH=$PATH:~/go/bin && sqlc generate`

Expected: No errors. New query functions in `api/internal/database/`.

**Step 5: Commit**

```bash
git add api/queries/acct_accounts.sql api/queries/acct_items.sql api/queries/acct_cash_accounts.sql api/internal/database/
git commit -m "feat(accounting): add sqlc queries for master data CRUD"
```

---

## Task 3: sqlc Queries — Cash Transactions + Purchase Entry

**Files:**
- Create: `api/queries/acct_cash_transactions.sql`

**Step 1: Write cash transaction queries**

Create `api/queries/acct_cash_transactions.sql`:

```sql
-- name: ListAcctCashTransactions :many
SELECT * FROM acct_cash_transactions
WHERE
    ($1::date IS NULL OR transaction_date >= $1) AND
    ($2::date IS NULL OR transaction_date <= $2) AND
    ($3::text IS NULL OR line_type = $3) AND
    ($4::uuid IS NULL OR account_id = $4) AND
    ($5::uuid IS NULL OR cash_account_id = $5) AND
    ($6::uuid IS NULL OR outlet_id = $6)
ORDER BY transaction_date DESC, created_at DESC
LIMIT $7 OFFSET $8;

-- name: GetAcctCashTransaction :one
SELECT * FROM acct_cash_transactions WHERE id = $1;

-- name: CreateAcctCashTransaction :one
INSERT INTO acct_cash_transactions (
    transaction_code, transaction_date, item_id, description,
    quantity, unit_price, amount, line_type,
    account_id, cash_account_id, outlet_id, reimbursement_batch_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: GetNextTransactionCode :one
SELECT COALESCE(
    MAX(transaction_code),
    'PCS000000'
) FROM acct_cash_transactions;

-- name: GetLastItemPrice :one
SELECT unit_price FROM acct_cash_transactions
WHERE item_id = $1
ORDER BY transaction_date DESC, created_at DESC
LIMIT 1;
```

**Step 2: Regenerate sqlc**

Run: `cd api && export PATH=$PATH:~/go/bin && sqlc generate`

Expected: No errors. New query functions generated.

**Step 3: Commit**

```bash
git add api/queries/acct_cash_transactions.sql api/internal/database/
git commit -m "feat(accounting): add sqlc queries for cash transactions"
```

---

## Task 4: Item Matching Engine — Core Logic

**Files:**
- Create: `api/internal/accounting/matcher/matcher.go`
- Create: `api/internal/accounting/matcher/matcher_test.go`

**Step 1: Write matcher tests**

Create `api/internal/accounting/matcher/matcher_test.go`:

```go
package matcher

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Cabe Merah Tanjung", "cabe merah tanjung"},
		{"BERAS  Sania", "beras sania"},
		{"minyak,goreng", "minyak goreng"},
		{"test.item!", "test item"},
	}
	for _, tt := range tests {
		got := normalize(tt.input)
		if got != tt.want {
			t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("cabe merah tanjung 5kg 500k")
	if len(tokens) != 5 {
		t.Fatalf("tokenize: got %d tokens, want 5: %v", len(tokens), tokens)
	}
}

func TestExtractQuantity(t *testing.T) {
	tests := []struct {
		tokens []string
		qty    float64
		unit   string
		rest   []string
	}{
		{[]string{"cabe", "merah", "5kg"}, 5, "kg", []string{"cabe", "merah"}},
		{[]string{"minyak", "2L"}, 2, "L", []string{"minyak"}},
		{[]string{"beras", "20kg"}, 20, "kg", []string{"beras"}},
		{[]string{"tahu", "3pcs"}, 3, "pcs", []string{"tahu"}},
		{[]string{"gula", "10bks"}, 10, "bks", []string{"gula"}},
		{[]string{"jeruk", "1iket"}, 1, "iket", []string{"jeruk"}},
		// No quantity token
		{[]string{"cabe", "merah"}, 1, "", []string{"cabe", "merah"}},
	}
	for _, tt := range tests {
		qty, unit, rest := extractQuantity(tt.tokens)
		if qty != tt.qty {
			t.Errorf("extractQuantity(%v): qty = %f, want %f", tt.tokens, qty, tt.qty)
		}
		if unit != tt.unit {
			t.Errorf("extractQuantity(%v): unit = %q, want %q", tt.tokens, unit, tt.unit)
		}
		if len(rest) != len(tt.rest) {
			t.Errorf("extractQuantity(%v): rest = %v, want %v", tt.tokens, rest, tt.rest)
		}
	}
}

func TestMatchItems_SingleMatch(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.New(),
			Code:     "ITEM0012",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
		{
			ID:       uuid.New(),
			Code:     "ITEM0013",
			Name:     "Cabe Merah Kriting",
			Keywords: "cabe,merah,kriting",
			Unit:     "kg",
		},
	}
	m := New(items)

	result := m.Match("cabe merah tanjung")
	if result.Status != Matched {
		t.Fatalf("Match status: got %v, want Matched", result.Status)
	}
	if result.Item.Code != "ITEM0012" {
		t.Errorf("Match item code: got %v, want ITEM0012", result.Item.Code)
	}
}

func TestMatchItems_Ambiguous(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.New(),
			Code:     "ITEM0012",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
		{
			ID:       uuid.New(),
			Code:     "ITEM0013",
			Name:     "Cabe Merah Kriting",
			Keywords: "cabe,merah,kriting",
			Unit:     "kg",
		},
	}
	m := New(items)

	// "cabe merah" without variant → ambiguous between tanjung and kriting
	result := m.Match("cabe merah")
	if result.Status != Ambiguous {
		t.Fatalf("Match status: got %v, want Ambiguous", result.Status)
	}
	if len(result.Candidates) != 2 {
		t.Errorf("Candidates: got %d, want 2", len(result.Candidates))
	}
}

func TestMatchItems_Unmatched(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.New(),
			Code:     "ITEM0012",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
	}
	m := New(items)

	result := m.Match("unknown item xyz")
	if result.Status != Unmatched {
		t.Fatalf("Match status: got %v, want Unmatched", result.Status)
	}
}

func TestMatchItems_VariantFilter(t *testing.T) {
	items := []Item{
		{
			ID:       uuid.New(),
			Code:     "ITEM0001",
			Name:     "Cabe Hijau",
			Keywords: "cabe,hijau",
			Unit:     "kg",
		},
		{
			ID:       uuid.New(),
			Code:     "ITEM0002",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
	}
	m := New(items)

	// "cabe hijau" should match only ITEM0001 — hijau is a variant keyword
	result := m.Match("cabe hijau")
	if result.Status != Matched {
		t.Fatalf("Match status: got %v, want Matched", result.Status)
	}
	if result.Item.Code != "ITEM0001" {
		t.Errorf("Match item code: got %v, want ITEM0001", result.Item.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/matcher/ -v`

Expected: FAIL — package doesn't exist yet.

**Step 3: Write matcher implementation**

Create `api/internal/accounting/matcher/matcher.go`:

```go
package matcher

import (
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// MatchStatus represents the outcome of an item match attempt.
type MatchStatus int

const (
	Matched   MatchStatus = iota
	Ambiguous
	Unmatched
)

// Item represents a matchable inventory item.
type Item struct {
	ID       uuid.UUID
	Code     string
	Name     string
	Keywords string // CSV: "cabe,merah,tanjung"
	Unit     string
}

// MatchResult holds the outcome of matching a text description against items.
type MatchResult struct {
	Status     MatchStatus
	Item       *Item   // set when Status == Matched
	Candidates []Item  // set when Status == Ambiguous
}

// variantKeywords are color/variant keywords that get higher weight
// and trigger hard filtering (if input contains one, candidate MUST have it).
var variantKeywords = map[string]bool{
	"merah": true, "hijau": true, "kuning": true, "putih": true,
	"tanjung": true, "kriting": true, "keriting": true,
	"besar": true, "kecil": true, "sedang": true,
}

const variantWeight = 5
const regularWeight = 1

// Matcher scores text descriptions against a list of items.
type Matcher struct {
	items          []Item
	itemKeywordMap [][]string // pre-tokenized keywords per item
}

// New creates a Matcher from a list of items.
func New(items []Item) *Matcher {
	kwMap := make([][]string, len(items))
	for i, item := range items {
		kwMap[i] = strings.Split(strings.ToLower(item.Keywords), ",")
		for j := range kwMap[i] {
			kwMap[i][j] = strings.TrimSpace(kwMap[i][j])
		}
	}
	return &Matcher{items: items, itemKeywordMap: kwMap}
}

// Match scores a text description against all items and returns the result.
func (m *Matcher) Match(text string) MatchResult {
	normalized := normalize(text)
	tokens := tokenize(normalized)
	_, _, descTokens := extractQuantity(tokens)

	if len(descTokens) == 0 {
		return MatchResult{Status: Unmatched}
	}

	// Find which variant keywords are in the input
	inputVariants := map[string]bool{}
	for _, tok := range descTokens {
		if variantKeywords[tok] {
			inputVariants[tok] = true
		}
	}

	type scored struct {
		item  Item
		score int
	}

	var candidates []scored

	for i, item := range m.items {
		kws := m.itemKeywordMap[i]

		// Hard filter: if input has a variant keyword, candidate MUST have it
		skip := false
		for v := range inputVariants {
			found := false
			for _, kw := range kws {
				if kw == v {
					found = true
					break
				}
			}
			if !found {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Score: count keyword intersections
		score := 0
		for _, tok := range descTokens {
			for _, kw := range kws {
				if tok == kw {
					if variantKeywords[tok] {
						score += variantWeight
					} else {
						score += regularWeight
					}
					break
				}
			}
		}

		if score > 0 {
			candidates = append(candidates, scored{item: item, score: score})
		}
	}

	if len(candidates) == 0 {
		return MatchResult{Status: Unmatched}
	}

	// Find max score
	maxScore := 0
	for _, c := range candidates {
		if c.score > maxScore {
			maxScore = c.score
		}
	}

	// Collect top-scoring candidates
	var top []Item
	for _, c := range candidates {
		if c.score == maxScore {
			top = append(top, c.item)
		}
	}

	if len(top) == 1 {
		return MatchResult{Status: Matched, Item: &top[0]}
	}

	return MatchResult{Status: Ambiguous, Candidates: top}
}

// normalize lowercases and strips punctuation.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(' ')
		}
	}
	// Collapse multiple spaces
	return strings.Join(strings.Fields(b.String()), " ")
}

// tokenize splits a normalized string into tokens.
func tokenize(s string) []string {
	return strings.Fields(s)
}

// extractQuantity finds a quantity+unit token (e.g., "5kg", "2L", "3pcs")
// and returns the quantity, unit, and remaining tokens.
func extractQuantity(tokens []string) (qty float64, unit string, rest []string) {
	for i, tok := range tokens {
		q, u, ok := parseQtyUnit(tok)
		if ok {
			rest = make([]string, 0, len(tokens)-1)
			rest = append(rest, tokens[:i]...)
			rest = append(rest, tokens[i+1:]...)
			return q, u, rest
		}
	}
	return 1, "", tokens
}

// parseQtyUnit tries to parse "5kg" into (5, "kg", true).
func parseQtyUnit(tok string) (float64, string, bool) {
	// Find where digits end and letters begin
	i := 0
	hasDot := false
	for i < len(tok) {
		if tok[i] >= '0' && tok[i] <= '9' {
			i++
		} else if tok[i] == '.' && !hasDot {
			hasDot = true
			i++
		} else {
			break
		}
	}

	if i == 0 || i == len(tok) {
		return 0, "", false
	}

	numStr := tok[:i]
	unitStr := tok[i:]

	// Unit must be letters only
	for _, r := range unitStr {
		if !unicode.IsLetter(r) {
			return 0, "", false
		}
	}

	var qty float64
	n, err := strings.NewReader(numStr).Read(nil)
	_ = n
	// Simple float parse
	for _, r := range numStr {
		_ = r
	}
	// Use strconv
	import_needed := false
	_ = import_needed

	// Actually parse the number properly
	val := 0.0
	dotPos := -1
	for j, c := range numStr {
		if c == '.' {
			dotPos = j
			continue
		}
		digit := float64(c - '0')
		if dotPos >= 0 {
			divisor := 1.0
			for k := 0; k < j-dotPos; k++ {
				divisor *= 10
			}
			val += digit / divisor
		} else {
			val = val*10 + digit
		}
	}
	qty = val

	if qty <= 0 {
		return 0, "", false
	}

	return qty, unitStr, true
}
```

Wait — that `parseQtyUnit` is getting complicated. Let me simplify using `strconv`:

```go
package matcher

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// MatchStatus represents the outcome of an item match attempt.
type MatchStatus int

const (
	Matched   MatchStatus = iota
	Ambiguous
	Unmatched
)

func (s MatchStatus) String() string {
	switch s {
	case Matched:
		return "Matched"
	case Ambiguous:
		return "Ambiguous"
	case Unmatched:
		return "Unmatched"
	default:
		return "Unknown"
	}
}

// Item represents a matchable inventory item.
type Item struct {
	ID       uuid.UUID
	Code     string
	Name     string
	Keywords string // CSV: "cabe,merah,tanjung"
	Unit     string
}

// MatchResult holds the outcome of matching a text description against items.
type MatchResult struct {
	Status     MatchStatus
	Item       *Item
	Candidates []Item
}

// variantKeywords are color/variant keywords that get higher weight
// and trigger hard filtering.
var variantKeywords = map[string]bool{
	"merah": true, "hijau": true, "kuning": true, "putih": true,
	"tanjung": true, "kriting": true, "keriting": true,
	"besar": true, "kecil": true, "sedang": true,
}

const variantWeight = 5
const regularWeight = 1

// Matcher scores text descriptions against a list of items.
type Matcher struct {
	items          []Item
	itemKeywordMap [][]string
}

// New creates a Matcher from a list of items.
func New(items []Item) *Matcher {
	kwMap := make([][]string, len(items))
	for i, item := range items {
		kwMap[i] = strings.Split(strings.ToLower(item.Keywords), ",")
		for j := range kwMap[i] {
			kwMap[i][j] = strings.TrimSpace(kwMap[i][j])
		}
	}
	return &Matcher{items: items, itemKeywordMap: kwMap}
}

// Match scores a text description against all items and returns the result.
func (m *Matcher) Match(text string) MatchResult {
	normalized := normalize(text)
	tokens := tokenize(normalized)
	_, _, descTokens := extractQuantity(tokens)

	if len(descTokens) == 0 {
		return MatchResult{Status: Unmatched}
	}

	inputVariants := map[string]bool{}
	for _, tok := range descTokens {
		if variantKeywords[tok] {
			inputVariants[tok] = true
		}
	}

	type scored struct {
		item  Item
		score int
	}
	var candidates []scored

	for i, item := range m.items {
		kws := m.itemKeywordMap[i]

		skip := false
		for v := range inputVariants {
			found := false
			for _, kw := range kws {
				if kw == v {
					found = true
					break
				}
			}
			if !found {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		score := 0
		for _, tok := range descTokens {
			for _, kw := range kws {
				if tok == kw {
					if variantKeywords[tok] {
						score += variantWeight
					} else {
						score += regularWeight
					}
					break
				}
			}
		}

		if score > 0 {
			candidates = append(candidates, scored{item: item, score: score})
		}
	}

	if len(candidates) == 0 {
		return MatchResult{Status: Unmatched}
	}

	maxScore := 0
	for _, c := range candidates {
		if c.score > maxScore {
			maxScore = c.score
		}
	}

	var top []Item
	for _, c := range candidates {
		if c.score == maxScore {
			top = append(top, c.item)
		}
	}

	if len(top) == 1 {
		return MatchResult{Status: Matched, Item: &top[0]}
	}
	return MatchResult{Status: Ambiguous, Candidates: top}
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func tokenize(s string) []string {
	return strings.Fields(s)
}

func extractQuantity(tokens []string) (qty float64, unit string, rest []string) {
	for i, tok := range tokens {
		q, u, ok := parseQtyUnit(tok)
		if ok {
			rest = make([]string, 0, len(tokens)-1)
			rest = append(rest, tokens[:i]...)
			rest = append(rest, tokens[i+1:]...)
			return q, u, rest
		}
	}
	return 1, "", tokens
}

func parseQtyUnit(tok string) (float64, string, bool) {
	i := 0
	for i < len(tok) && (tok[i] >= '0' && tok[i] <= '9' || tok[i] == '.') {
		i++
	}
	if i == 0 || i == len(tok) {
		return 0, "", false
	}

	numStr := tok[:i]
	unitStr := tok[i:]

	for _, r := range unitStr {
		if !unicode.IsLetter(r) {
			return 0, "", false
		}
	}

	qty, err := strconv.ParseFloat(numStr, 64)
	if err != nil || qty <= 0 {
		return 0, "", false
	}

	return qty, unitStr, true
}
```

**Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/accounting/matcher/ -v`

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add api/internal/accounting/matcher/
git commit -m "feat(accounting): add item matching engine with keyword scoring"
```

---

## Task 5: Master Data Handlers — Accounts, Items, Cash Accounts

**Files:**
- Create: `api/internal/accounting/handler/master.go`
- Create: `api/internal/accounting/handler/master_test.go`

**Step 1: Write master handler tests**

Create `api/internal/accounting/handler/master_test.go`:

```go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock store for accounts ---

type mockAcctAccountStore struct {
	accounts map[uuid.UUID]database.AcctAccount
}

func newMockAcctAccountStore() *mockAcctAccountStore {
	return &mockAcctAccountStore{accounts: make(map[uuid.UUID]database.AcctAccount)}
}

func (m *mockAcctAccountStore) ListAcctAccounts(ctx context.Context) ([]database.AcctAccount, error) {
	var result []database.AcctAccount
	for _, a := range m.accounts {
		if a.IsActive.Bool {
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *mockAcctAccountStore) GetAcctAccount(ctx context.Context, id uuid.UUID) (database.AcctAccount, error) {
	a, ok := m.accounts[id]
	if !ok || !a.IsActive.Bool {
		return database.AcctAccount{}, pgx.ErrNoRows
	}
	return a, nil
}

func (m *mockAcctAccountStore) CreateAcctAccount(ctx context.Context, arg database.CreateAcctAccountParams) (database.AcctAccount, error) {
	a := database.AcctAccount{
		ID:          uuid.New(),
		AccountCode: arg.AccountCode,
		AccountName: arg.AccountName,
		AccountType: arg.AccountType,
		LineType:    arg.LineType,
		IsActive:    pgtype.Bool{Bool: true, Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.accounts[a.ID] = a
	return a, nil
}

func (m *mockAcctAccountStore) UpdateAcctAccount(ctx context.Context, arg database.UpdateAcctAccountParams) (database.AcctAccount, error) {
	a, ok := m.accounts[arg.ID]
	if !ok || !a.IsActive.Bool {
		return database.AcctAccount{}, pgx.ErrNoRows
	}
	a.AccountName = arg.AccountName
	a.AccountType = arg.AccountType
	a.LineType = arg.LineType
	m.accounts[a.ID] = a
	return a, nil
}

func (m *mockAcctAccountStore) SoftDeleteAcctAccount(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	a, ok := m.accounts[id]
	if !ok || !a.IsActive.Bool {
		return uuid.Nil, pgx.ErrNoRows
	}
	a.IsActive = pgtype.Bool{Bool: false, Valid: true}
	m.accounts[a.ID] = a
	return a.ID, nil
}

// --- Test helpers ---

func doRequest(t *testing.T, router http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

func decodeJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

// --- Account tests ---

func setupAccountRouter(store handler.AcctAccountStore) *chi.Mux {
	h := handler.NewMasterHandler(store, nil, nil)
	r := chi.NewRouter()
	r.Route("/accounting/master/accounts", h.RegisterAccountRoutes)
	return r
}

func TestAccountList_Empty(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/master/accounts", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestAccountCreate_Valid(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/master/accounts", map[string]interface{}{
		"account_code": "1000",
		"account_name": "Cash on Hand",
		"account_type": "Asset",
		"line_type":    "ASSET",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	resp := decodeJSON(t, rr)
	if resp["account_code"] != "1000" {
		t.Errorf("account_code: got %v, want 1000", resp["account_code"])
	}
}

func TestAccountCreate_MissingCode(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/master/accounts", map[string]interface{}{
		"account_name": "Cash",
		"account_type": "Asset",
		"line_type":    "ASSET",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestAccountDelete_NotFound(t *testing.T) {
	store := newMockAcctAccountStore()
	router := setupAccountRouter(store)
	rr := doRequest(t, router, "DELETE", "/accounting/master/accounts/"+uuid.New().String(), nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: FAIL — package doesn't exist yet.

**Step 3: Write master handler**

Create `api/internal/accounting/handler/master.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Store interfaces (consumer-defines-interface) ---

type AcctAccountStore interface {
	ListAcctAccounts(ctx context.Context) ([]database.AcctAccount, error)
	GetAcctAccount(ctx context.Context, id uuid.UUID) (database.AcctAccount, error)
	CreateAcctAccount(ctx context.Context, arg database.CreateAcctAccountParams) (database.AcctAccount, error)
	UpdateAcctAccount(ctx context.Context, arg database.UpdateAcctAccountParams) (database.AcctAccount, error)
	SoftDeleteAcctAccount(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

type AcctItemStore interface {
	ListAcctItems(ctx context.Context) ([]database.AcctItem, error)
	GetAcctItem(ctx context.Context, id uuid.UUID) (database.AcctItem, error)
	CreateAcctItem(ctx context.Context, arg database.CreateAcctItemParams) (database.AcctItem, error)
	UpdateAcctItem(ctx context.Context, arg database.UpdateAcctItemParams) (database.AcctItem, error)
	SoftDeleteAcctItem(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

type AcctCashAccountStore interface {
	ListAcctCashAccounts(ctx context.Context) ([]database.AcctCashAccount, error)
	GetAcctCashAccount(ctx context.Context, id uuid.UUID) (database.AcctCashAccount, error)
	CreateAcctCashAccount(ctx context.Context, arg database.CreateAcctCashAccountParams) (database.AcctCashAccount, error)
	UpdateAcctCashAccount(ctx context.Context, arg database.UpdateAcctCashAccountParams) (database.AcctCashAccount, error)
	SoftDeleteAcctCashAccount(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// MasterHandler handles master data CRUD for accounting.
type MasterHandler struct {
	acctStore     AcctAccountStore
	itemStore     AcctItemStore
	cashAcctStore AcctCashAccountStore
}

func NewMasterHandler(acctStore AcctAccountStore, itemStore AcctItemStore, cashAcctStore AcctCashAccountStore) *MasterHandler {
	return &MasterHandler{
		acctStore:     acctStore,
		itemStore:     itemStore,
		cashAcctStore: cashAcctStore,
	}
}

// --- Account routes ---

func (h *MasterHandler) RegisterAccountRoutes(r chi.Router) {
	r.Get("/", h.ListAccounts)
	r.Post("/", h.CreateAccount)
	r.Put("/{id}", h.UpdateAccount)
	r.Delete("/{id}", h.DeleteAccount)
}

// --- Account request/response types ---

type createAccountRequest struct {
	AccountCode string `json:"account_code"`
	AccountName string `json:"account_name"`
	AccountType string `json:"account_type"`
	LineType    string `json:"line_type"`
}

type updateAccountRequest struct {
	AccountName string `json:"account_name"`
	AccountType string `json:"account_type"`
	LineType    string `json:"line_type"`
}

type accountResponse struct {
	ID          uuid.UUID `json:"id"`
	AccountCode string    `json:"account_code"`
	AccountName string    `json:"account_name"`
	AccountType string    `json:"account_type"`
	LineType    string    `json:"line_type"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

func toAccountResponse(a database.AcctAccount) accountResponse {
	return accountResponse{
		ID:          a.ID,
		AccountCode: a.AccountCode,
		AccountName: a.AccountName,
		AccountType: a.AccountType,
		LineType:    a.LineType,
		IsActive:    a.IsActive.Bool,
		CreatedAt:   a.CreatedAt.Time,
	}
}

// --- Account handlers ---

func (h *MasterHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.acctStore.ListAcctAccounts(r.Context())
	if err != nil {
		log.Printf("ERROR: list acct accounts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *MasterHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.AccountCode == "" || req.AccountName == "" || req.AccountType == "" || req.LineType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_code, account_name, account_type, and line_type are required"})
		return
	}

	account, err := h.acctStore.CreateAcctAccount(r.Context(), database.CreateAcctAccountParams{
		AccountCode: req.AccountCode,
		AccountName: req.AccountName,
		AccountType: req.AccountType,
		LineType:    req.LineType,
	})
	if err != nil {
		log.Printf("ERROR: create acct account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusCreated, toAccountResponse(account))
}

func (h *MasterHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ID"})
		return
	}

	var req updateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.AccountName == "" || req.AccountType == "" || req.LineType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_name, account_type, and line_type are required"})
		return
	}

	account, err := h.acctStore.UpdateAcctAccount(r.Context(), database.UpdateAcctAccountParams{
		ID:          id,
		AccountName: req.AccountName,
		AccountType: req.AccountType,
		LineType:    req.LineType,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
		log.Printf("ERROR: update acct account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, toAccountResponse(account))
}

func (h *MasterHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ID"})
		return
	}

	_, err = h.acctStore.SoftDeleteAcctAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
		log.Printf("ERROR: delete acct account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Item routes (same pattern, added later in Task 5b) ---

func (h *MasterHandler) RegisterItemRoutes(r chi.Router) {
	r.Get("/", h.ListItems)
	r.Post("/", h.CreateItem)
	r.Put("/{id}", h.UpdateItem)
	r.Delete("/{id}", h.DeleteItem)
}

// --- Cash Account routes (same pattern, added later in Task 5c) ---

func (h *MasterHandler) RegisterCashAccountRoutes(r chi.Router) {
	r.Get("/", h.ListCashAccounts)
	r.Post("/", h.CreateCashAccount)
	r.Put("/{id}", h.UpdateCashAccount)
	r.Delete("/{id}", h.DeleteCashAccount)
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}
```

Note: Item and CashAccount handlers follow the exact same CRUD pattern as Account handlers. The implementing engineer should follow the `ListAccounts`/`CreateAccount`/`UpdateAccount`/`DeleteAccount` pattern for both. The request/response types and `to*Response` functions will map the sqlc-generated types. I'm omitting the full code for Items and CashAccounts here to avoid repeating 200+ lines of identical CRUD — they are structurally identical to the Account handlers.

**Step 4: Add item and cash account handler stubs**

Implement `ListItems`, `CreateItem`, `UpdateItem`, `DeleteItem`, `ListCashAccounts`, `CreateCashAccount`, `UpdateCashAccount`, `DeleteCashAccount` in the same file following the Account handler pattern. The request types map directly to the sqlc create/update param structs. Response types expose all columns except internal pgtype wrappers.

**Step 5: Run tests to verify they pass**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 6: Commit**

```bash
git add api/internal/accounting/handler/
git commit -m "feat(accounting): add master data CRUD handlers for accounts, items, cash accounts"
```

---

## Task 6: Purchase Entry Handler

**Files:**
- Create: `api/internal/accounting/handler/purchase.go`
- Create: `api/internal/accounting/handler/purchase_test.go`

**Step 1: Write purchase handler tests**

Create `api/internal/accounting/handler/purchase_test.go`. Key test cases:

1. **Create single purchase** — POST `/accounting/purchases` with one item line. Expect `201` with `acct_cash_transactions` row created.
2. **Create multi-line purchase** — POST with multiple items. Expect multiple transactions created, all with same date and cash_account.
3. **Missing date** — Expect `400`.
4. **Missing cash_account_id** — Expect `400`.
5. **Auto-fills transaction_code** — Verify sequential `PCS000001` codes.
6. **Updates item last_price** — After purchase, verify item's `last_price` is updated.

Key endpoint design:
```
POST /accounting/purchases
{
  "transaction_date": "2026-01-20",
  "cash_account_id": "uuid",
  "outlet_id": "uuid",          // optional
  "items": [
    {
      "item_id": "uuid",
      "description": "Cabe merah tanjung",
      "quantity": 5,
      "unit_price": "100000"
    }
  ]
}
```

**Step 2: Write purchase handler**

Create `api/internal/accounting/handler/purchase.go`:

The handler should:
1. Parse the request body
2. Validate required fields (date, cash_account_id, items non-empty)
3. Get the next transaction code by querying `GetNextTransactionCode`, parsing the numeric suffix, and incrementing
4. For each item:
   - Create `acct_cash_transactions` row with `line_type=INVENTORY`, `account_id` = the inventory account (1200)
   - Auto-increment transaction code
   - If `item_id` is set, update `acct_items.last_price` via `UpdateAcctItemLastPrice`
5. Return the created transactions

The store interface needs:
```go
type PurchaseStore interface {
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
	UpdateAcctItemLastPrice(ctx context.Context, arg database.UpdateAcctItemLastPriceParams) error
}
```

**Step 3: Run tests**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/purchase.go api/internal/accounting/handler/purchase_test.go
git commit -m "feat(accounting): add purchase entry handler"
```

---

## Task 7: Wire Accounting Routes into Router

**Files:**
- Modify: `api/internal/router/router.go`

**Step 1: Add accounting routes to router**

Add a new `r.Group` block for accounting routes, protected by auth + OWNER role:

```go
// Accounting routes (OWNER only, not outlet-scoped)
r.Group(func(r chi.Router) {
    r.Use(mw.RequireRole("OWNER"))

    // Master data
    masterHandler := accthandler.NewMasterHandler(queries, queries, queries)
    r.Route("/accounting/master/accounts", masterHandler.RegisterAccountRoutes)
    r.Route("/accounting/master/items", masterHandler.RegisterItemRoutes)
    r.Route("/accounting/master/cash-accounts", masterHandler.RegisterCashAccountRoutes)

    // Purchases
    purchaseHandler := accthandler.NewPurchaseHandler(queries)
    r.Route("/accounting/purchases", purchaseHandler.RegisterRoutes)
})
```

Add import: `accthandler "github.com/kiwari-pos/api/internal/accounting/handler"`

This block goes inside the existing `r.Group(func(r chi.Router) { r.Use(mw.Authenticate(cfg.JWTSecret)) ... })` block, after the owner-only reports routes.

**Step 2: Verify it compiles**

Run: `cd api && go build ./...`

Expected: No errors.

**Step 3: Commit**

```bash
git add api/internal/router/router.go
git commit -m "feat(accounting): wire accounting routes into main router"
```

---

## Task 8: Admin Types — Accounting API Types

**Files:**
- Modify: `admin/src/lib/types/api.ts`

**Step 1: Add accounting types to api.ts**

Append to the end of `admin/src/lib/types/api.ts`:

```typescript
// ── Accounting types ────────────────────

export interface AcctAccount {
	id: string;
	account_code: string;
	account_name: string;
	account_type: 'Asset' | 'Liability' | 'Equity' | 'Revenue' | 'Expense';
	line_type: string;
	is_active: boolean;
	created_at: string;
}

export interface AcctItem {
	id: string;
	item_code: string;
	item_name: string;
	item_category: 'Raw Material' | 'Packaging' | 'Consumable';
	unit: string;
	is_inventory: boolean;
	is_active: boolean;
	average_price: string | null;
	last_price: string | null;
	for_hpp: string | null;
	keywords: string;
	created_at: string;
}

export interface AcctCashAccount {
	id: string;
	cash_account_code: string;
	cash_account_name: string;
	bank_name: string | null;
	ownership: 'Business' | 'Personal';
	is_active: boolean;
	created_at: string;
}

export interface AcctCashTransaction {
	id: string;
	transaction_code: string;
	transaction_date: string;
	item_id: string | null;
	description: string;
	quantity: string;
	unit_price: string;
	amount: string;
	line_type: string;
	account_id: string;
	cash_account_id: string | null;
	outlet_id: string | null;
	reimbursement_batch_id: string | null;
	created_at: string;
}
```

**Step 2: Commit**

```bash
git add admin/src/lib/types/api.ts
git commit -m "feat(accounting): add accounting API types for admin frontend"
```

---

## Task 9: Sidebar — Add Keuangan Section

**Files:**
- Modify: `admin/src/lib/components/Sidebar.svelte`

**Step 1: Add Keuangan nav items**

In `admin/src/lib/components/Sidebar.svelte`, add the Keuangan section items to the `navItems` array. Add a separator concept (or just new items with OWNER role):

```typescript
const navItems: NavItem[] = [
    { label: 'Dashboard', href: '/', icon: '##' },
    { label: 'Menu', href: '/menu', icon: '##' },
    { label: 'Orders', href: '/orders', icon: '##' },
    { label: 'Customers', href: '/customers', icon: '##' },
    { label: 'Reports', href: '/reports', icon: '##', roles: ['OWNER', 'MANAGER'] },
    { label: 'Settings', href: '/settings', icon: '##', roles: ['OWNER', 'MANAGER'] },
];

const keuanganItems: NavItem[] = [
    { label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
    { label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] },
];
```

Update the template to render a "Keuangan" section header and the `keuanganItems` below the main nav items. Only show the section if the user is OWNER.

**Step 2: Verify it renders**

Run: `cd admin && pnpm dev` — check sidebar shows "Keuangan" section for OWNER role.

**Step 3: Commit**

```bash
git add admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): add Keuangan section to admin sidebar"
```

---

## Task 10: Admin Page — Master Data

**Files:**
- Create: `admin/src/routes/(app)/accounting/master/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/master/+page.svelte`

**Step 1: Write server load**

Create `admin/src/routes/(app)/accounting/master/+page.server.ts`:

```typescript
import { apiRequest } from '$lib/server/api';
import type { AcctAccount, AcctItem, AcctCashAccount } from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';
import { fail } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ cookies }) => {
    const accessToken = cookies.get('access_token')!;

    const [accountsResult, itemsResult, cashAccountsResult] = await Promise.all([
        apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
        apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
        apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
    ]);

    return {
        accounts: accountsResult.ok ? accountsResult.data : [],
        items: itemsResult.ok ? itemsResult.data : [],
        cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
    };
};

export const actions: Actions = {
    createAccount: async ({ request, cookies }) => {
        const accessToken = cookies.get('access_token')!;
        const formData = await request.formData();
        const body = {
            account_code: formData.get('account_code'),
            account_name: formData.get('account_name'),
            account_type: formData.get('account_type'),
            line_type: formData.get('line_type'),
        };
        const result = await apiRequest('/accounting/master/accounts', {
            method: 'POST', body, accessToken
        });
        if (!result.ok) return fail(result.status, { error: result.message });
        return { success: true };
    },
    deleteAccount: async ({ request, cookies }) => {
        const accessToken = cookies.get('access_token')!;
        const formData = await request.formData();
        const id = formData.get('id');
        const result = await apiRequest(`/accounting/master/accounts/${id}`, {
            method: 'DELETE', accessToken
        });
        if (!result.ok) return fail(result.status, { error: result.message });
        return { success: true };
    },
    // Same pattern for createItem, deleteItem, createCashAccount, deleteCashAccount
};
```

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/master/+page.svelte`:

Three-tab layout (Akun, Item, Kas) with simple tables + add/delete forms. Follow the existing admin page patterns: scoped styles, Svelte 5 `$props()`, CSS variables.

**Step 3: Verify it works**

Run: `cd admin && pnpm dev` — navigate to `/accounting/master`. Verify tabs render, add/delete works.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/
git commit -m "feat(accounting): add master data admin page with accounts, items, cash accounts tabs"
```

---

## Task 11: Admin Page — Purchase Entry

**Files:**
- Create: `admin/src/routes/(app)/accounting/purchases/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/purchases/+page.svelte`

**Step 1: Write server load**

Create `admin/src/routes/(app)/accounting/purchases/+page.server.ts`:

```typescript
import { apiRequest } from '$lib/server/api';
import type { AcctItem, AcctCashAccount, AcctCashTransaction } from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';
import { fail } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ cookies, url }) => {
    const accessToken = cookies.get('access_token')!;

    const [itemsResult, cashAccountsResult, recentTxResult] = await Promise.all([
        apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
        apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken }),
        apiRequest<AcctCashTransaction[]>('/accounting/purchases?limit=20', { accessToken }),
    ]);

    return {
        items: itemsResult.ok ? itemsResult.data : [],
        cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
        recentPurchases: recentTxResult.ok ? recentTxResult.data : [],
    };
};

export const actions: Actions = {
    create: async ({ request, cookies }) => {
        const accessToken = cookies.get('access_token')!;
        const formData = await request.formData();
        const body = JSON.parse(formData.get('purchase_data') as string);

        const result = await apiRequest('/accounting/purchases', {
            method: 'POST', body, accessToken
        });
        if (!result.ok) return fail(result.status, { error: result.message });
        return { success: true };
    },
};
```

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/purchases/+page.svelte`:

Purchase entry form with:
- Date picker
- Cash account selector (dropdown)
- Outlet selector (optional, dropdown)
- Multi-line item entry with keyword-based autocomplete
- Quantity + unit price fields per line
- Amount auto-calculated
- "Simpan" button submits all lines

Use `$state()` for form state, `$derived()` for computed totals. Item autocomplete filters `data.items` by keyword match client-side.

**Step 3: Verify it works**

Run: `cd admin && pnpm dev` — navigate to `/accounting/purchases`. Test form interaction.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/purchases/
git commit -m "feat(accounting): add purchase entry admin page with item autocomplete"
```

---

## Task 12: Verify Build + Run Full Test Suite

**Step 1: Run Go tests**

Run: `cd api && go test ./... -v`

Expected: All tests pass, including new accounting tests.

**Step 2: Run admin build**

Run: `cd admin && pnpm build`

Expected: No type errors. Build succeeds.

**Step 3: Verify API starts**

Run: `cd api && go build ./cmd/server/`

Expected: Binary compiles without errors.

**Step 4: Final commit if needed**

Fix any issues found, commit fixes.

---

## Phase 1 Checklist

| # | Task | Delivers |
|---|------|----------|
| 1 | DB migration | All `acct_*` tables |
| 2 | sqlc queries — master data | CRUD for accounts, items, cash accounts |
| 3 | sqlc queries — transactions | Cash transaction queries + purchase support |
| 4 | Item matching engine | `matcher.go` with scoring, variant filtering, tests |
| 5 | Master data handlers | CRUD endpoints for 3 master data tables |
| 6 | Purchase handler | Purchase entry with auto-sequencing + price update |
| 7 | Router wiring | Accounting routes in main router |
| 8 | Admin types | TypeScript types for accounting API |
| 9 | Sidebar | Keuangan section in admin navigation |
| 10 | Master data page | Admin UI for accounts, items, cash accounts |
| 11 | Purchase entry page | Admin UI for daily purchase recording |
| 12 | Verify build | Full test suite + build verification |

**NOT in Phase 1:** Data migration script (one-time import of ~38k GSheet rows), reimbursement workflow, reports, sales, payroll. These are Phases 2-4 per the design doc.

**Data migration note:** The migration script to import historical GSheet data is conceptually part of Phase 1 per the design doc, but it's a separate one-time task that depends on having GSheet export CSVs available. It should be a separate plan/task after Phase 1 tables are deployed.
