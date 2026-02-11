# Accounting Module Phase 2 — Reimbursement Workflow + WhatsApp

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the reimbursement workflow: CRUD, WhatsApp message parsing, item matching integration, batch posting with two-leg entries, and an admin page for review/posting.

**Architecture:** New `parser/` package for WhatsApp message parsing. New `reimbursement.go` and `whatsapp.go` handlers. Batch posting uses `pgxpool.Pool` transaction for atomicity. Two-leg entries per reimbursement item: expense leg (correct expense date) + payment leg (correct payment date). SvelteKit page with status tabs, item matching, and batch posting dialog.

**Tech Stack:** Go 1.22+ (Chi, sqlc, pgx/v5, shopspring/decimal), PostgreSQL 16, SvelteKit 2 (Svelte 5, Tailwind CSS 4).

**Design Doc:** `docs/plans/2026-02-11-accounting-module-design.md` (Phase 2 section)

**Depends on:** Phase 1 complete (all `acct_*` tables, master data CRUD, item matcher, purchase handler).

---

## Codebase Conventions Reference

Same as Phase 1 plan. Key additions for Phase 2:

### New Patterns in Phase 2
- **DB transactions for batch operations:** Use `pool.Begin(ctx)` → `database.New(tx)` → `tx.Commit(ctx)` with `defer tx.Rollback(ctx)`. Same pattern as `api/internal/handler/orders.go` order creation.
- **Two-leg entries:** Reimbursement batch posting creates 2 `acct_cash_transactions` per item: expense leg (date=expense_date, cash_account_id=NULL) + payment leg (date=payment_date, cash_account_id=selected).
- **Parser package:** Pure text parsing, no DB dependency. Lives in `api/internal/accounting/parser/`. Tested independently.

### Commands
```bash
cd api && go test ./internal/accounting/... -v      # All accounting tests
cd api && go test ./internal/accounting/parser/ -v   # Parser tests only
cd api && go test ./internal/accounting/handler/ -v  # Handler tests only
cd api && export PATH=$PATH:~/go/bin && sqlc generate
cd admin && pnpm dev
cd admin && pnpm build
```

---

## Task 1: sqlc Queries — Reimbursements + Account Lookup

**Files:**
- Create: `api/queries/acct_reimbursement_requests.sql`
- Modify: `api/queries/acct_accounts.sql` (add GetAcctAccountByCode)

**Step 1: Write reimbursement queries**

Create `api/queries/acct_reimbursement_requests.sql`:

```sql
-- name: ListAcctReimbursementRequests :many
SELECT * FROM acct_reimbursement_requests
WHERE
    (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')) AND
    (sqlc.narg('batch_id')::text IS NULL OR batch_id = sqlc.narg('batch_id')) AND
    (sqlc.narg('requester')::text IS NULL OR requester = sqlc.narg('requester'))
ORDER BY expense_date DESC, created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAcctReimbursementRequest :one
SELECT * FROM acct_reimbursement_requests WHERE id = $1;

-- name: CreateAcctReimbursementRequest :one
INSERT INTO acct_reimbursement_requests (
    batch_id, expense_date, item_id, description, qty, unit_price, amount,
    line_type, account_id, status, requester, receipt_link
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateAcctReimbursementRequest :one
UPDATE acct_reimbursement_requests
SET item_id = $2, description = $3, qty = $4, unit_price = $5, amount = $6,
    line_type = $7, account_id = $8, status = $9
WHERE id = $1
RETURNING *;

-- name: UpdateReimbursementPosted :exec
UPDATE acct_reimbursement_requests
SET batch_id = $2, status = 'Posted', posted_at = now()
WHERE id = $1;

-- name: CountReimbursementBatches :one
SELECT COALESCE(COUNT(DISTINCT batch_id), 0)::int AS batch_count
FROM acct_reimbursement_requests
WHERE batch_id IS NOT NULL;

-- name: CountReimbursementsByStatus :many
SELECT status,
    COUNT(*)::int AS count,
    COALESCE(SUM(amount), 0)::text AS total_amount
FROM acct_reimbursement_requests
GROUP BY status;
```

**Step 2: Add account lookup by code**

Append to `api/queries/acct_accounts.sql`:

```sql
-- name: GetAcctAccountByCode :one
SELECT * FROM acct_accounts WHERE account_code = $1 AND is_active = true;
```

**Step 3: Add prefix-specific purchase code query**

Append to `api/queries/acct_cash_transactions.sql`:

```sql
-- Prefix-specific transaction code sequence for purchases (PCS)
-- CRITICAL: The existing GetNextTransactionCode uses global MAX which breaks
-- when other prefixes (SLS/PYR/JNL) are introduced in Phase 4.
-- Always use this prefix-specific query for PCS code generation.
-- name: GetNextPurchaseCode :one
SELECT COALESCE(MAX(transaction_code), 'PCS000000')::text AS max_code
FROM acct_cash_transactions
WHERE transaction_code LIKE 'PCS%';
```

**Note:** Also update `api/internal/accounting/handler/purchase.go` to use `GetNextPurchaseCode` instead of `GetNextTransactionCode`. Update the `PurchaseStore` interface accordingly:

```go
type PurchaseStore interface {
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextPurchaseCode(ctx context.Context) (string, error) // Changed from GetNextTransactionCode
	UpdateAcctItemLastPrice(ctx context.Context, arg database.UpdateAcctItemLastPriceParams) error
}
```

And in `CreatePurchase`, change `h.store.GetNextTransactionCode(r.Context())` → `h.store.GetNextPurchaseCode(r.Context())`.

**Step 4: Regenerate sqlc**

Run: `cd api && export PATH=$PATH:~/go/bin && sqlc generate`

Expected: No errors. New query functions in `api/internal/database/`.

**Step 5: Commit**

```bash
git add api/queries/acct_reimbursement_requests.sql api/queries/acct_accounts.sql api/queries/acct_cash_transactions.sql api/internal/database/ api/internal/accounting/handler/purchase.go
git commit -m "feat(accounting): add sqlc queries for reimbursements, account lookup, and prefix-specific PCS code sequence"
```

---

## Task 2: WhatsApp Message Parser — Core Logic

**Files:**
- Create: `api/internal/accounting/parser/parser.go`
- Create: `api/internal/accounting/parser/parser_test.go`

**Step 1: Write parser tests**

Create `api/internal/accounting/parser/parser_test.go`:

```go
package parser

import (
	"testing"
	"time"
)

func TestParseDate_Indonesian(t *testing.T) {
	tests := []struct {
		input string
		month time.Month
		day   int
	}{
		{"20 jan", time.January, 20},
		{"5 feb", time.February, 5},
		{"15 mei", time.May, 15},
		{"1 agu", time.August, 1},
		{"3 ags", time.August, 3},
		{"10 okt", time.October, 10},
		{"25 des", time.December, 25},
		{"7 agustus", time.August, 7},
	}
	for _, tt := range tests {
		date, err := parseDate(tt.input)
		if err != nil {
			t.Errorf("parseDate(%q): unexpected error: %v", tt.input, err)
			continue
		}
		if date.Month() != tt.month || date.Day() != tt.day {
			t.Errorf("parseDate(%q) = %v, want month=%v day=%d", tt.input, date, tt.month, tt.day)
		}
	}
}

func TestParseDate_Invalid(t *testing.T) {
	invalids := []string{"cabe merah", "hello world", "", "32 jan", "0 feb"}
	for _, s := range invalids {
		_, err := parseDate(s)
		if err == nil {
			t.Errorf("parseDate(%q): expected error, got nil", s)
		}
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		token string
		price int64
		ok    bool
	}{
		{"500k", 500000, true},
		{"1.5jt", 1500000, true},
		{"72k", 72000, true},
		{"300rb", 300000, true},
		{"25000", 25000, true},
		{"2.5k", 2500, true},
		{"1jt", 1000000, true},
		{"cabe", 0, false},
		{"5kg", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		price, ok := parsePrice(tt.token)
		if ok != tt.ok {
			t.Errorf("parsePrice(%q): ok = %v, want %v", tt.token, ok, tt.ok)
			continue
		}
		if ok && price != tt.price {
			t.Errorf("parsePrice(%q) = %d, want %d", tt.token, price, tt.price)
		}
	}
}

func TestParseItemLine(t *testing.T) {
	tests := []struct {
		line string
		desc string
		qty  float64
		unit string
		price int64
	}{
		{"cabe merah tanjung 5kg 500k", "cabe merah tanjung", 5, "kg", 500000},
		{"beras sania 20kg 300k", "beras sania", 20, "kg", 300000},
		{"minyak 2L 72k", "minyak", 2, "L", 72000},
		{"sabun cuci 3pcs 25k", "sabun cuci", 3, "pcs", 25000},
		{"gula 10bks 150k", "gula", 10, "bks", 150000},
		{"jeruk 1iket 15k", "jeruk", 1, "iket", 15000},
	}
	for _, tt := range tests {
		item := parseItemLine(tt.line)
		if item.Description != tt.desc {
			t.Errorf("parseItemLine(%q).Description = %q, want %q", tt.line, item.Description, tt.desc)
		}
		if item.Quantity != tt.qty {
			t.Errorf("parseItemLine(%q).Quantity = %f, want %f", tt.line, item.Quantity, tt.qty)
		}
		if item.Unit != tt.unit {
			t.Errorf("parseItemLine(%q).Unit = %q, want %q", tt.line, item.Unit, tt.unit)
		}
		if item.TotalPrice != tt.price {
			t.Errorf("parseItemLine(%q).TotalPrice = %d, want %d", tt.line, item.TotalPrice, tt.price)
		}
	}
}

func TestParseMessage_FullMessage(t *testing.T) {
	msg := "20 jan\ncabe merah tanjung 5kg 500k\nberas sania 20kg 300k\nminyak 2L 72k"

	result, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage: unexpected error: %v", err)
	}

	if result.ExpenseDate.Month() != time.January || result.ExpenseDate.Day() != 20 {
		t.Errorf("ExpenseDate = %v, want Jan 20", result.ExpenseDate)
	}
	if len(result.Items) != 3 {
		t.Fatalf("Items count = %d, want 3", len(result.Items))
	}
	if result.Items[0].Description != "cabe merah tanjung" {
		t.Errorf("Items[0].Description = %q, want %q", result.Items[0].Description, "cabe merah tanjung")
	}
	if result.Items[0].TotalPrice != 500000 {
		t.Errorf("Items[0].TotalPrice = %d, want 500000", result.Items[0].TotalPrice)
	}
}

func TestParseMessage_EmptyLines(t *testing.T) {
	msg := "20 jan\n\ncabe 5kg 500k\n\nberas 20kg 300k\n"
	result, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage: unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(result.Items))
	}
}

func TestParseMessage_NoDate(t *testing.T) {
	msg := "cabe 5kg 500k\nberas 20kg 300k"
	_, err := ParseMessage(msg)
	if err == nil {
		t.Error("ParseMessage: expected error for missing date, got nil")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/parser/ -v`

Expected: FAIL — package doesn't exist yet.

**Step 3: Write parser implementation**

Create `api/internal/accounting/parser/parser.go`:

```go
package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ParseResult holds the parsed WhatsApp reimbursement message.
type ParseResult struct {
	ExpenseDate time.Time
	Items       []ParsedItem
}

// ParsedItem represents one line item from the message.
type ParsedItem struct {
	RawLine     string
	Description string  // remaining tokens after qty/unit/price extraction
	Quantity    float64
	Unit        string
	TotalPrice  int64 // in Rupiah (e.g., 500000)
}

// UnitPrice returns total_price / quantity.
func (p ParsedItem) UnitPrice() int64 {
	if p.Quantity <= 0 {
		return p.TotalPrice
	}
	return int64(float64(p.TotalPrice) / p.Quantity)
}

// ParseMessage parses a WhatsApp reimbursement message.
// First line must be a date (e.g., "20 jan").
// Subsequent lines are item entries (e.g., "cabe merah 5kg 500k").
func ParseMessage(msg string) (*ParseResult, error) {
	lines := strings.Split(strings.TrimSpace(msg), "\n")

	// Find date line (first non-empty line)
	var dateLine string
	var itemLines []string
	foundDate := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !foundDate {
			dateLine = line
			foundDate = true
		} else {
			itemLines = append(itemLines, line)
		}
	}

	if !foundDate {
		return nil, fmt.Errorf("empty message")
	}

	date, err := parseDate(dateLine)
	if err != nil {
		return nil, fmt.Errorf("first line must be a date (e.g., '20 jan'): %w", err)
	}

	if len(itemLines) == 0 {
		return nil, fmt.Errorf("no item lines found after date")
	}

	var items []ParsedItem
	for _, line := range itemLines {
		items = append(items, parseItemLine(line))
	}

	return &ParseResult{
		ExpenseDate: date,
		Items:       items,
	}, nil
}

// monthMap maps Indonesian month abbreviations/names to time.Month.
var monthMap = map[string]time.Month{
	"jan": time.January, "januari": time.January,
	"feb": time.February, "februari": time.February,
	"mar": time.March, "maret": time.March,
	"apr": time.April, "april": time.April,
	"mei": time.May,
	"jun": time.June, "juni": time.June,
	"jul": time.July, "juli": time.July,
	"agu": time.August, "ags": time.August, "agustus": time.August,
	"sep": time.September, "september": time.September,
	"okt": time.October, "oktober": time.October,
	"nov": time.November, "november": time.November,
	"des": time.December, "desember": time.December,
}

// parseDate parses "20 jan" into a time.Time (current year).
func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("expected 'DD month', got %q", s)
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, fmt.Errorf("invalid day: %q", parts[0])
	}

	month, ok := monthMap[parts[1]]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown month: %q", parts[1])
	}

	year := time.Now().Year()
	date := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	// Validate the date is real (e.g., not Feb 31)
	if date.Day() != day {
		return time.Time{}, fmt.Errorf("invalid date: %d %s", day, parts[1])
	}

	return date, nil
}

// parsePrice tries to parse a price token.
// Supports: "500k" → 500000, "1.5jt" → 1500000, "300rb" → 300000, "25000" → 25000.
func parsePrice(tok string) (int64, bool) {
	tok = strings.ToLower(strings.TrimSpace(tok))
	if tok == "" {
		return 0, false
	}

	// Try suffixed formats
	type suffix struct {
		s    string
		mult float64
	}
	suffixes := []suffix{
		{"jt", 1_000_000},
		{"rb", 1_000},
		{"k", 1_000},
	}

	for _, sf := range suffixes {
		if strings.HasSuffix(tok, sf.s) {
			numStr := tok[:len(tok)-len(sf.s)]
			if numStr == "" {
				return 0, false
			}
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil || val <= 0 {
				return 0, false
			}
			return int64(val * sf.mult), true
		}
	}

	// Try plain number (must be all digits or digits with dots/commas)
	// Remove thousand separators (dots in Indonesian: 500.000)
	cleaned := strings.ReplaceAll(tok, ".", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	val, err := strconv.ParseInt(cleaned, 10, 64)
	if err != nil || val <= 0 {
		return 0, false
	}
	return val, true
}

// parseItemLine parses a single item line like "cabe merah tanjung 5kg 500k".
// Extracts: description (remaining tokens), quantity, unit, total price.
func parseItemLine(line string) ParsedItem {
	line = strings.TrimSpace(line)
	tokens := strings.Fields(strings.ToLower(line))

	result := ParsedItem{
		RawLine:  line,
		Quantity: 1,
	}

	if len(tokens) == 0 {
		return result
	}

	// Extract price (scan from right)
	priceIdx := -1
	for i := len(tokens) - 1; i >= 0; i-- {
		price, ok := parsePrice(tokens[i])
		if ok {
			result.TotalPrice = price
			priceIdx = i
			break
		}
	}

	// Extract quantity+unit (scan remaining tokens from right, skip price token)
	qtyIdx := -1
	for i := len(tokens) - 1; i >= 0; i-- {
		if i == priceIdx {
			continue
		}
		q, u, ok := parseQtyUnit(tokens[i])
		if ok {
			result.Quantity = q
			result.Unit = u
			qtyIdx = i
			break
		}
	}

	// Remaining tokens = description
	var descTokens []string
	for i, tok := range tokens {
		if i == priceIdx || i == qtyIdx {
			continue
		}
		descTokens = append(descTokens, tok)
	}
	result.Description = strings.Join(descTokens, " ")

	return result
}

// parseQtyUnit tries to parse "5kg" into (5, "kg", true).
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

Run: `cd api && go test ./internal/accounting/parser/ -v`

Expected: All tests PASS.

**Step 5: Commit**

```bash
git add api/internal/accounting/parser/
git commit -m "feat(accounting): add WhatsApp message parser for reimbursements"
```

---

## Task 3: Reimbursement Handler — CRUD + List

**Files:**
- Create: `api/internal/accounting/handler/reimbursement.go`
- Create: `api/internal/accounting/handler/reimbursement_test.go`

**Step 1: Write reimbursement handler tests**

Create `api/internal/accounting/handler/reimbursement_test.go`:

Key test cases (follow `purchase_test.go` pattern — mock store with in-memory map):

1. `TestReimbursementList_Empty` — GET `/accounting/reimbursements` → 200, empty array
2. `TestReimbursementList_FilterByStatus` — GET `?status=Draft` → only Draft items
3. `TestReimbursementCreate_Valid` — POST with all fields → 201
4. `TestReimbursementCreate_MissingDescription` — POST → 400
5. `TestReimbursementUpdate_Valid` — PUT `/{id}` with new item_id and status=Ready → 200
6. `TestReimbursementUpdate_NotFound` — PUT unknown ID → 404

Mock store implements `ReimbursementStore` interface (defined in Step 3).

```go
package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

type mockReimbursementStore struct {
	items map[uuid.UUID]database.AcctReimbursementRequest
}

func newMockReimbursementStore() *mockReimbursementStore {
	return &mockReimbursementStore{items: make(map[uuid.UUID]database.AcctReimbursementRequest)}
}

func (m *mockReimbursementStore) ListAcctReimbursementRequests(ctx context.Context, arg database.ListAcctReimbursementRequestsParams) ([]database.AcctReimbursementRequest, error) {
	var result []database.AcctReimbursementRequest
	for _, r := range m.items {
		if arg.Status.Valid && r.Status != arg.Status.String {
			continue
		}
		result = append(result, r)
	}
	return result, nil
}

func (m *mockReimbursementStore) GetAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (database.AcctReimbursementRequest, error) {
	r, ok := m.items[id]
	if !ok {
		return database.AcctReimbursementRequest{}, pgx.ErrNoRows
	}
	return r, nil
}

func (m *mockReimbursementStore) CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r := database.AcctReimbursementRequest{
		ID:          uuid.New(),
		BatchID:     arg.BatchID,
		ExpenseDate: arg.ExpenseDate,
		ItemID:      arg.ItemID,
		Description: arg.Description,
		Qty:         arg.Qty,
		UnitPrice:   arg.UnitPrice,
		Amount:      arg.Amount,
		LineType:    arg.LineType,
		AccountID:   arg.AccountID,
		Status:      arg.Status,
		Requester:   arg.Requester,
		ReceiptLink: arg.ReceiptLink,
		CreatedAt:   time.Now(),
	}
	m.items[r.ID] = r
	return r, nil
}

func (m *mockReimbursementStore) UpdateAcctReimbursementRequest(ctx context.Context, arg database.UpdateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r, ok := m.items[arg.ID]
	if !ok {
		return database.AcctReimbursementRequest{}, pgx.ErrNoRows
	}
	r.ItemID = arg.ItemID
	r.Description = arg.Description
	r.Qty = arg.Qty
	r.UnitPrice = arg.UnitPrice
	r.Amount = arg.Amount
	r.LineType = arg.LineType
	r.AccountID = arg.AccountID
	r.Status = arg.Status
	m.items[r.ID] = r
	return r, nil
}

func (m *mockReimbursementStore) CountReimbursementsByStatus(ctx context.Context) ([]database.CountReimbursementsByStatusRow, error) {
	counts := map[string]int32{}
	for _, r := range m.items {
		counts[r.Status]++
	}
	var result []database.CountReimbursementsByStatusRow
	for status, count := range counts {
		result = append(result, database.CountReimbursementsByStatusRow{
			Status:      status,
			Count:       count,
			TotalAmount: "0.00",
		})
	}
	return result, nil
}

func setupReimbursementRouter(store handler.ReimbursementStore) *chi.Mux {
	h := handler.NewReimbursementHandler(store, nil) // nil pool for non-batch tests
	r := chi.NewRouter()
	r.Route("/accounting/reimbursements", h.RegisterRoutes)
	return r
}

func TestReimbursementList_Empty(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "GET", "/accounting/reimbursements?limit=50&offset=0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestReimbursementCreate_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements", map[string]interface{}{
		"expense_date": "2026-01-20",
		"description":  "Cabe merah tanjung 5kg",
		"qty":          "5",
		"unit_price":   "100000",
		"amount":       "500000",
		"line_type":    "INVENTORY",
		"account_id":   uuid.New().String(),
		"requester":    "Hamidah",
	})
	if rr.Code != http.StatusCreated {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
}

func TestReimbursementCreate_MissingDescription(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements", map[string]interface{}{
		"expense_date": "2026-01-20",
		"qty":          "5",
		"unit_price":   "100000",
		"amount":       "500000",
		"line_type":    "INVENTORY",
		"account_id":   uuid.New().String(),
		"requester":    "Hamidah",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestReimbursementUpdate_NotFound(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "PUT", "/accounting/reimbursements/"+uuid.New().String(), map[string]interface{}{
		"description": "Updated",
		"qty":         "5",
		"unit_price":  "100000",
		"amount":      "500000",
		"line_type":   "INVENTORY",
		"account_id":  uuid.New().String(),
		"status":      "Ready",
	})
	if rr.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusNotFound)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: FAIL — `ReimbursementStore` and `NewReimbursementHandler` not defined yet.

**Step 3: Write reimbursement handler**

Create `api/internal/accounting/handler/reimbursement.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// ReimbursementStore defines the database methods for reimbursement handlers.
type ReimbursementStore interface {
	ListAcctReimbursementRequests(ctx context.Context, arg database.ListAcctReimbursementRequestsParams) ([]database.AcctReimbursementRequest, error)
	GetAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (database.AcctReimbursementRequest, error)
	CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	UpdateAcctReimbursementRequest(ctx context.Context, arg database.UpdateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	CountReimbursementsByStatus(ctx context.Context) ([]database.CountReimbursementsByStatusRow, error)
}

// --- Handler ---

// ReimbursementHandler handles reimbursement endpoints.
type ReimbursementHandler struct {
	store ReimbursementStore
	pool  *pgxpool.Pool // for batch posting transaction
}

// NewReimbursementHandler creates a new ReimbursementHandler.
func NewReimbursementHandler(store ReimbursementStore, pool *pgxpool.Pool) *ReimbursementHandler {
	return &ReimbursementHandler{store: store, pool: pool}
}

// RegisterRoutes registers reimbursement endpoints.
func (h *ReimbursementHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Put("/{id}", h.Update)
	r.Get("/counts", h.StatusCounts)
	r.Post("/post-batch", h.PostBatch)
}

// --- Request / Response types ---

type createReimbursementRequest struct {
	ExpenseDate string  `json:"expense_date"` // "2026-01-20"
	ItemID      *string `json:"item_id"`      // optional UUID
	Description string  `json:"description"`
	Qty         string  `json:"qty"`
	UnitPrice   string  `json:"unit_price"`
	Amount      string  `json:"amount"`
	LineType    string  `json:"line_type"`
	AccountID   string  `json:"account_id"`
	Requester   string  `json:"requester"`
	ReceiptLink *string `json:"receipt_link"`
}

type updateReimbursementRequest struct {
	ItemID      *string `json:"item_id"`
	Description string  `json:"description"`
	Qty         string  `json:"qty"`
	UnitPrice   string  `json:"unit_price"`
	Amount      string  `json:"amount"`
	LineType    string  `json:"line_type"`
	AccountID   string  `json:"account_id"`
	Status      string  `json:"status"` // "Draft" or "Ready"
}

type reimbursementResponse struct {
	ID          uuid.UUID  `json:"id"`
	BatchID     *string    `json:"batch_id"`
	ExpenseDate string     `json:"expense_date"`
	ItemID      *string    `json:"item_id"`
	Description string     `json:"description"`
	Qty         string     `json:"qty"`
	UnitPrice   string     `json:"unit_price"`
	Amount      string     `json:"amount"`
	LineType    string     `json:"line_type"`
	AccountID   uuid.UUID  `json:"account_id"`
	Status      string     `json:"status"`
	Requester   string     `json:"requester"`
	ReceiptLink *string    `json:"receipt_link"`
	PostedAt    *time.Time `json:"posted_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toReimbursementResponse(r database.AcctReimbursementRequest) reimbursementResponse {
	resp := reimbursementResponse{
		ID:          r.ID,
		ExpenseDate: r.ExpenseDate.Time.Format("2006-01-02"),
		Description: r.Description,
		LineType:    r.LineType,
		AccountID:   r.AccountID,
		Status:      r.Status,
		Requester:   r.Requester,
		CreatedAt:   r.CreatedAt,
	}
	if r.BatchID.Valid {
		resp.BatchID = &r.BatchID.String
	}
	if r.ItemID.Valid {
		idStr := uuid.UUID(r.ItemID.Bytes).String()
		resp.ItemID = &idStr
	}
	if r.ReceiptLink.Valid {
		resp.ReceiptLink = &r.ReceiptLink.String
	}
	if r.PostedAt.Valid {
		resp.PostedAt = &r.PostedAt.Time
	}

	// Convert Numeric fields to string
	resp.Qty = numericToString(r.Qty)
	resp.UnitPrice = numericToString(r.UnitPrice)
	resp.Amount = numericToString(r.Amount)

	return resp
}

// numericToString converts pgtype.Numeric to string with 2 decimal places.
// CANONICAL: This is THE numeric-to-string helper for all accounting handlers.
// Phases 3 and 4 should reuse this function (same package, already accessible).
// Do NOT create duplicates like numericToDecimalString — use this one.
func numericToString(n pgtype.Numeric) string {
	if !n.Valid {
		return "0.00"
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return "0.00"
	}
	d, err := decimal.NewFromString(val.(string))
	if err != nil {
		return "0.00"
	}
	return d.StringFixed(2)
}

// --- Handlers ---

// List returns reimbursement requests filtered by query params.
func (h *ReimbursementHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	params := database.ListAcctReimbursementRequestsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		params.Status = pgtype.Text{String: s, Valid: true}
	}
	if s := r.URL.Query().Get("batch_id"); s != "" {
		params.BatchID = pgtype.Text{String: s, Valid: true}
	}
	if s := r.URL.Query().Get("requester"); s != "" {
		params.Requester = pgtype.Text{String: s, Valid: true}
	}

	items, err := h.store.ListAcctReimbursementRequests(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list reimbursements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]reimbursementResponse, len(items))
	for i, item := range items {
		resp[i] = toReimbursementResponse(item)
	}
	writeJSON(w, http.StatusOK, resp)
}

// Create adds a new reimbursement request.
func (h *ReimbursementHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createReimbursementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ExpenseDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expense_date is required"})
		return
	}
	if req.Description == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "description is required"})
		return
	}
	if req.Qty == "" || req.UnitPrice == "" || req.Amount == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "qty, unit_price, and amount are required"})
		return
	}
	if req.LineType == "" || req.AccountID == "" || req.Requester == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type, account_id, and requester are required"})
		return
	}

	// Parse date
	date, err := time.Parse("2006-01-02", req.ExpenseDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid expense_date format"})
		return
	}

	// Parse account_id
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse optional item_id
	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		id, err := uuid.Parse(*req.ItemID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
			return
		}
		itemID = uuidToPgUUID(id)
	}

	// Parse decimals
	var qtyPg, pricePg, amountPg pgtype.Numeric
	if err := qtyPg.Scan(req.Qty); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty format"})
		return
	}
	if err := pricePg.Scan(req.UnitPrice); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price format"})
		return
	}
	if err := amountPg.Scan(req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount format"})
		return
	}

	item, err := h.store.CreateAcctReimbursementRequest(r.Context(), database.CreateAcctReimbursementRequestParams{
		BatchID:     pgtype.Text{}, // null
		ExpenseDate: pgtype.Date{Time: date, Valid: true},
		ItemID:      itemID,
		Description: req.Description,
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    req.LineType,
		AccountID:   accountID,
		Status:      "Draft",
		Requester:   req.Requester,
		ReceiptLink: stringToPgText(req.ReceiptLink),
	})
	if err != nil {
		log.Printf("ERROR: create reimbursement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toReimbursementResponse(item))
}

// Update modifies a reimbursement request (owner review).
func (h *ReimbursementHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ID"})
		return
	}

	var req updateReimbursementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Description == "" || req.Qty == "" || req.UnitPrice == "" || req.Amount == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "description, qty, unit_price, and amount are required"})
		return
	}
	if req.LineType == "" || req.AccountID == "" || req.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type, account_id, and status are required"})
		return
	}

	validStatuses := map[string]bool{"Draft": true, "Ready": true}
	if !validStatuses[req.Status] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be Draft or Ready"})
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		pid, err := uuid.Parse(*req.ItemID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
			return
		}
		itemID = uuidToPgUUID(pid)
	}

	var qtyPg, pricePg, amountPg pgtype.Numeric
	if err := qtyPg.Scan(req.Qty); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty"})
		return
	}
	if err := pricePg.Scan(req.UnitPrice); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price"})
		return
	}
	if err := amountPg.Scan(req.Amount); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount"})
		return
	}

	item, err := h.store.UpdateAcctReimbursementRequest(r.Context(), database.UpdateAcctReimbursementRequestParams{
		ID:          id,
		ItemID:      itemID,
		Description: req.Description,
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    req.LineType,
		AccountID:   accountID,
		Status:      req.Status,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "reimbursement not found"})
			return
		}
		log.Printf("ERROR: update reimbursement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toReimbursementResponse(item))
}

// StatusCounts returns count of reimbursements per status.
func (h *ReimbursementHandler) StatusCounts(w http.ResponseWriter, r *http.Request) {
	counts, err := h.store.CountReimbursementsByStatus(r.Context())
	if err != nil {
		log.Printf("ERROR: count reimbursements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS (including existing purchase + master tests).

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/reimbursement.go api/internal/accounting/handler/reimbursement_test.go
git commit -m "feat(accounting): add reimbursement CRUD handler with list, create, update, status counts"
```

---

## Task 4: Reimbursement Batch Posting

**Files:**
- Modify: `api/internal/accounting/handler/reimbursement.go` (PostBatch method — already stubbed in RegisterRoutes)
- Modify: `api/internal/accounting/handler/reimbursement_test.go` (add batch posting tests)

**Step 1: Write batch posting tests**

Append to `reimbursement_test.go`. These tests need a mock that also implements the batch-posting DB operations. Since PostBatch uses `pool.Begin()` for a real transaction, the test mock approach is different — either:
- Test with integration test (real DB) — deferred to Task 8
- Test the validation logic (bad request cases) with unit tests

Add these unit tests for validation:

```go
func TestReimbursementPostBatch_EmptyIDs(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements/post-batch", map[string]interface{}{
		"ids":             []string{},
		"payment_date":    "2026-01-25",
		"cash_account_id": uuid.New().String(),
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestReimbursementPostBatch_MissingDate(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements/post-batch", map[string]interface{}{
		"ids":             []string{uuid.New().String()},
		"cash_account_id": uuid.New().String(),
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestReimbursementPostBatch_NilPool(t *testing.T) {
	store := newMockReimbursementStore()
	// pool is nil → should return 500
	router := setupReimbursementRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements/post-batch", map[string]interface{}{
		"ids":             []string{uuid.New().String()},
		"payment_date":    "2026-01-25",
		"cash_account_id": uuid.New().String(),
	})
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}
```

**Step 2: Write PostBatch handler**

Add to `reimbursement.go`:

```go
// --- Batch Posting ---

type postBatchRequest struct {
	IDs           []string `json:"ids"`            // reimbursement request UUIDs
	PaymentDate   string   `json:"payment_date"`   // "2026-01-25"
	CashAccountID string   `json:"cash_account_id"`
}

type postBatchResponse struct {
	BatchID             string `json:"batch_id"`
	ItemsPosted         int    `json:"items_posted"`
	TotalAmount         string `json:"total_amount"`
	TransactionsCreated int    `json:"transactions_created"`
}

// PostBatch posts a batch of Ready reimbursement requests.
// Creates two-leg cash_transaction entries per item:
//   - Expense leg: date=expense_date, account=item's account, cash_account=NULL
//   - Payment leg: date=payment_date, account=Reimb Payable (2101), cash_account=selected
func (h *ReimbursementHandler) PostBatch(w http.ResponseWriter, r *http.Request) {
	var req postBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids cannot be empty"})
		return
	}
	if req.PaymentDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payment_date is required"})
		return
	}
	if req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_id is required"})
		return
	}

	paymentDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment_date format"})
		return
	}

	cashAccountID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	var ids []uuid.UUID
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reimbursement ID: " + idStr})
			return
		}
		ids = append(ids, id)
	}

	if h.pool == nil {
		log.Printf("ERROR: pool is nil for batch posting")
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Begin DB transaction
	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		log.Printf("ERROR: begin tx: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	defer tx.Rollback(r.Context())

	qtx := database.New(tx)

	// Generate batch ID
	batchCount, err := qtx.CountReimbursementBatches(r.Context())
	if err != nil {
		log.Printf("ERROR: count batches: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	batchID := fmt.Sprintf("REIMB-%s-%03d", paymentDate.Format("20060102"), batchCount+1)

	// Get next PCS transaction code (prefix-specific to avoid collision with SLS/PYR/JNL)
	maxCode, err := qtx.GetNextPurchaseCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next purchase code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, _ := strconv.Atoi(maxCode[3:])
	nextNum++

	// Look up Reimbursement Payable account (2101)
	reimbPayableAcct, err := qtx.GetAcctAccountByCode(r.Context(), "2101")
	if err != nil {
		log.Printf("ERROR: lookup account 2101: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reimbursement payable account (2101) not found"})
		return
	}

	totalAmount := decimal.Zero
	txCount := 0

	for _, id := range ids {
		// Fetch the reimbursement request
		reimb, err := qtx.GetAcctReimbursementRequest(r.Context(), id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "reimbursement not found: " + id.String()})
				return
			}
			log.Printf("ERROR: get reimbursement %s: %v", id, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Must be Ready (not already Posted)
		if reimb.Status != "Ready" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("reimbursement %s has status %q, expected Ready", id, reimb.Status),
			})
			return
		}

		// Expense leg: DR expense/inventory account, date=expense_date
		expenseCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++
		_, err = qtx.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      expenseCode,
			TransactionDate:      reimb.ExpenseDate,
			ItemID:               reimb.ItemID,
			Description:          reimb.Description,
			Quantity:             reimb.Qty,
			UnitPrice:            reimb.UnitPrice,
			Amount:               reimb.Amount,
			LineType:             reimb.LineType,
			AccountID:            reimb.AccountID,
			CashAccountID:        pgtype.UUID{},            // NULL — no cash movement
			OutletID:             pgtype.UUID{},             // NULL
			ReimbursementBatchID: pgtype.Text{String: batchID, Valid: true},
		})
		if err != nil {
			log.Printf("ERROR: create expense leg: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		txCount++

		// Payment leg: DR Reimb Payable, CR Cash, date=payment_date
		paymentCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++
		_, err = qtx.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      paymentCode,
			TransactionDate:      pgtype.Date{Time: paymentDate, Valid: true},
			ItemID:               pgtype.UUID{},
			Description:          fmt.Sprintf("Reimburse: %s", reimb.Description),
			Quantity:             reimb.Qty,
			UnitPrice:            reimb.UnitPrice,
			Amount:               reimb.Amount,
			LineType:             "LIABILITY",
			AccountID:            reimbPayableAcct.ID,
			CashAccountID:        uuidToPgUUID(cashAccountID),
			OutletID:             pgtype.UUID{},
			ReimbursementBatchID: pgtype.Text{String: batchID, Valid: true},
		})
		if err != nil {
			log.Printf("ERROR: create payment leg: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		txCount++

		// Update reimbursement status to Posted
		if err := qtx.UpdateReimbursementPosted(r.Context(), database.UpdateReimbursementPostedParams{
			ID:      id,
			BatchID: pgtype.Text{String: batchID, Valid: true},
		}); err != nil {
			log.Printf("ERROR: update reimbursement posted: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Sum total
		amtVal, err := reimb.Amount.Value()
		if err == nil && amtVal != nil {
			d, _ := decimal.NewFromString(amtVal.(string))
			totalAmount = totalAmount.Add(d)
		}
	}

	// Commit transaction
	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("ERROR: commit batch posting: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, postBatchResponse{
		BatchID:             batchID,
		ItemsPosted:         len(ids),
		TotalAmount:         totalAmount.StringFixed(2),
		TransactionsCreated: txCount,
	})
}
```

**Step 3: Run tests**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/reimbursement.go api/internal/accounting/handler/reimbursement_test.go
git commit -m "feat(accounting): add reimbursement batch posting with two-leg entries"
```

---

## Task 5: WhatsApp Endpoint Handler

**Files:**
- Create: `api/internal/accounting/handler/whatsapp.go`
- Create: `api/internal/accounting/handler/whatsapp_test.go`

**Step 1: Write WhatsApp handler tests**

Create `api/internal/accounting/handler/whatsapp_test.go`:

```go
package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

type mockWhatsAppStore struct {
	items    []database.AcctItem
	accounts map[string]database.AcctAccount // by code
	created  []database.AcctReimbursementRequest
}

func newMockWhatsAppStore() *mockWhatsAppStore {
	inventoryAcct := database.AcctAccount{
		ID:          uuid.New(),
		AccountCode: "1200",
		AccountName: "Inventory",
		AccountType: "Asset",
		LineType:    "INVENTORY",
		IsActive:    true,
	}
	expenseAcct := database.AcctAccount{
		ID:          uuid.New(),
		AccountCode: "6000",
		AccountName: "General Expense",
		AccountType: "Expense",
		LineType:    "EXPENSE",
		IsActive:    true,
	}
	return &mockWhatsAppStore{
		accounts: map[string]database.AcctAccount{
			"1200": inventoryAcct,
			"6000": expenseAcct,
		},
	}
}

func (m *mockWhatsAppStore) ListAcctItems(ctx context.Context) ([]database.AcctItem, error) {
	return m.items, nil
}

func (m *mockWhatsAppStore) GetAcctAccountByCode(ctx context.Context, code string) (database.AcctAccount, error) {
	a, ok := m.accounts[code]
	if !ok {
		return database.AcctAccount{}, fmt.Errorf("account not found")
	}
	return a, nil
}

func (m *mockWhatsAppStore) CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r := database.AcctReimbursementRequest{
		ID:          uuid.New(),
		Description: arg.Description,
		Status:      arg.Status,
		Requester:   arg.Requester,
	}
	m.created = append(m.created, r)
	return r, nil
}

func setupWhatsAppRouter(store handler.WhatsAppStore) *chi.Mux {
	h := handler.NewWhatsAppHandler(store)
	r := chi.NewRouter()
	r.Post("/accounting/reimbursements/from-whatsapp", h.FromWhatsApp)
	return r
}

func TestFromWhatsApp_ValidMessage(t *testing.T) {
	store := newMockWhatsAppStore()

	// Add a matchable item
	var avgPrice pgtype.Numeric
	avgPrice.Scan("100000")
	store.items = []database.AcctItem{
		{
			ID:           uuid.New(),
			ItemCode:     "ITEM0012",
			ItemName:     "Cabe Merah Tanjung",
			Keywords:     "cabe,merah,tanjung",
			Unit:         "kg",
			IsInventory:  true,
			IsActive:     true,
			AveragePrice: avgPrice,
		},
	}

	router := setupWhatsAppRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"message_text": "20 jan\ncabe merah tanjung 5kg 500k",
		"chat_id":      "120363421848364675@g.us",
	})

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	if len(store.created) != 1 {
		t.Errorf("created count: got %d, want 1", len(store.created))
	}
}

func TestFromWhatsApp_MissingMessageText(t *testing.T) {
	store := newMockWhatsAppStore()
	router := setupWhatsAppRouter(store)
	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
```

**Step 2: Write WhatsApp handler**

Create `api/internal/accounting/handler/whatsapp.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/matcher"
	"github.com/kiwari-pos/api/internal/accounting/parser"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Store interface ---

// WhatsAppStore defines the database methods needed by the WhatsApp handler.
type WhatsAppStore interface {
	ListAcctItems(ctx context.Context) ([]database.AcctItem, error)
	GetAcctAccountByCode(ctx context.Context, code string) (database.AcctAccount, error)
	CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
}

// --- Handler ---

// WhatsAppHandler handles the WhatsApp reimbursement endpoint.
type WhatsAppHandler struct {
	store WhatsAppStore
}

// NewWhatsAppHandler creates a new WhatsAppHandler.
func NewWhatsAppHandler(store WhatsAppStore) *WhatsAppHandler {
	return &WhatsAppHandler{store: store}
}

// --- Request / Response ---

type whatsAppRequest struct {
	SenderPhone string `json:"sender_phone"`
	SenderName  string `json:"sender_name"`
	MessageText string `json:"message_text"`
	ChatID      string `json:"chat_id"`
}

type whatsAppResponse struct {
	ReplyMessage   string `json:"reply_message"`
	ItemsCreated   int    `json:"items_created"`
	ItemsMatched   int    `json:"items_matched"`
	ItemsAmbiguous int    `json:"items_ambiguous"`
	ItemsUnmatched int    `json:"items_unmatched"`
}

// FromWhatsApp parses a WhatsApp reimbursement message, matches items, and creates requests.
func (h *WhatsAppHandler) FromWhatsApp(w http.ResponseWriter, r *http.Request) {
	var req whatsAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.MessageText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message_text is required"})
		return
	}
	if req.SenderName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sender_name is required"})
		return
	}

	// Parse the message
	parsed, err := parser.ParseMessage(req.MessageText)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parse error: " + err.Error()})
		return
	}

	// Load all active items for matching
	dbItems, err := h.store.ListAcctItems(r.Context())
	if err != nil {
		log.Printf("ERROR: load items for matching: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Convert to matcher items
	var matcherItems []matcher.Item
	for _, di := range dbItems {
		matcherItems = append(matcherItems, matcher.Item{
			ID:       di.ID,
			Code:     di.ItemCode,
			Name:     di.ItemName,
			Keywords: di.Keywords,
			Unit:     di.Unit,
		})
	}
	m := matcher.New(matcherItems)

	// Look up default accounts
	inventoryAcct, err := h.store.GetAcctAccountByCode(r.Context(), "1200")
	if err != nil {
		log.Printf("ERROR: lookup inventory account 1200: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "inventory account (1200) not found"})
		return
	}
	expenseAcct, err := h.store.GetAcctAccountByCode(r.Context(), "6000")
	if err != nil {
		log.Printf("ERROR: lookup expense account 6000: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "expense account (6000) not found"})
		return
	}

	// Process each parsed item
	var matchedLines, ambiguousLines, unmatchedLines []string
	matchedCount, ambiguousCount, unmatchedCount := 0, 0, 0
	totalPrice := int64(0)

	for _, pi := range parsed.Items {
		result := m.Match(pi.Description)

		var itemID pgtype.UUID
		var lineType string
		var accountID uuid.UUID
		var displayLine string

		unitPrice := pi.UnitPrice()
		amount := pi.TotalPrice

		switch result.Status {
		case matcher.Matched:
			matchedCount++
			itemID = uuidToPgUUID(result.Item.ID)

			// Determine line_type from matched item's is_inventory
			dbItem := findDBItem(dbItems, result.Item.ID)
			if dbItem != nil && dbItem.IsInventory {
				lineType = "INVENTORY"
				accountID = inventoryAcct.ID
			} else {
				lineType = "EXPENSE"
				accountID = expenseAcct.ID
			}

			qtyStr := ""
			if pi.Quantity != 1 || pi.Unit != "" {
				qtyStr = fmt.Sprintf(" %.0f%s", pi.Quantity, pi.Unit)
			}
			displayLine = fmt.Sprintf("• %s - %s%s - Rp%s",
				result.Item.Code, result.Item.Name, qtyStr, formatCompactPrice(amount))
			matchedLines = append(matchedLines, displayLine)

		case matcher.Ambiguous:
			ambiguousCount++
			lineType = "EXPENSE"
			accountID = expenseAcct.ID

			var names []string
			for _, c := range result.Candidates {
				names = append(names, c.Name)
			}
			displayLine = fmt.Sprintf("• %s %.0f%s - Rp%s → [%s]",
				pi.Description, pi.Quantity, pi.Unit, formatCompactPrice(amount),
				strings.Join(names, " / "))
			ambiguousLines = append(ambiguousLines, displayLine)

		case matcher.Unmatched:
			unmatchedCount++
			lineType = "EXPENSE"
			accountID = expenseAcct.ID

			displayLine = fmt.Sprintf("• %s %.0f%s - Rp%s",
				pi.Description, pi.Quantity, pi.Unit, formatCompactPrice(amount))
			unmatchedLines = append(unmatchedLines, displayLine)
		}

		totalPrice += amount

		// Create reimbursement request
		var qtyPg, pricePg, amountPg pgtype.Numeric
		qtyPg.Scan(fmt.Sprintf("%.4f", pi.Quantity))
		pricePg.Scan(fmt.Sprintf("%d", unitPrice))
		amountPg.Scan(fmt.Sprintf("%d", amount))

		h.store.CreateAcctReimbursementRequest(r.Context(), database.CreateAcctReimbursementRequestParams{
			BatchID:     pgtype.Text{},
			ExpenseDate: pgtype.Date{Time: parsed.ExpenseDate, Valid: true},
			ItemID:      itemID,
			Description: pi.RawLine,
			Qty:         qtyPg,
			UnitPrice:   pricePg,
			Amount:      amountPg,
			LineType:    lineType,
			AccountID:   accountID,
			Status:      "Draft",
			Requester:   req.SenderName,
			ReceiptLink: pgtype.Text{},
		})
	}

	// Format reply message
	dateStr := parsed.ExpenseDate.Format("2 Jan")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("✅ Reimbursement Draft (%s):\n\n", dateStr))

	if len(matchedLines) > 0 {
		sb.WriteString(fmt.Sprintf("📦 Cocok (%d):\n", matchedCount))
		for _, line := range matchedLines {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}
	if len(ambiguousLines) > 0 {
		sb.WriteString(fmt.Sprintf("⚠️ Ambigu (%d):\n", ambiguousCount))
		for _, line := range ambiguousLines {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}
	if len(unmatchedLines) > 0 {
		sb.WriteString(fmt.Sprintf("❌ Tidak Cocok (%d):\n", unmatchedCount))
		for _, line := range unmatchedLines {
			sb.WriteString(line + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Total: Rp%s (%d item)\n", formatCompactPrice(totalPrice), len(parsed.Items)))
	sb.WriteString("Status: Draft\n")
	sb.WriteString(fmt.Sprintf("👤 %s", req.SenderName))

	writeJSON(w, http.StatusOK, whatsAppResponse{
		ReplyMessage:   sb.String(),
		ItemsCreated:   len(parsed.Items),
		ItemsMatched:   matchedCount,
		ItemsAmbiguous: ambiguousCount,
		ItemsUnmatched: unmatchedCount,
	})
}

// --- Helpers ---

// findDBItem finds a database.AcctItem by UUID.
func findDBItem(items []database.AcctItem, id uuid.UUID) *database.AcctItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// formatCompactPrice formats price in compact Indonesian style.
// 500000 → "500K", 1500000 → "1.5Jt", 72000 → "72K", 800 → "800"
func formatCompactPrice(amount int64) string {
	if amount >= 1_000_000 && amount%1_000_000 == 0 {
		return fmt.Sprintf("%dJt", amount/1_000_000)
	}
	if amount >= 1_000_000 {
		val := float64(amount) / 1_000_000
		return fmt.Sprintf("%.1fJt", val)
	}
	if amount >= 1_000 && amount%1_000 == 0 {
		return fmt.Sprintf("%dK", amount/1_000)
	}
	if amount >= 1_000 {
		val := float64(amount) / 1_000
		return fmt.Sprintf("%.1fK", val)
	}
	return fmt.Sprintf("%d", amount)
}
```

**Step 3: Run tests**

Run: `cd api && go test ./internal/accounting/handler/ -v`

Expected: All tests PASS.

**Step 4: Commit**

```bash
git add api/internal/accounting/handler/whatsapp.go api/internal/accounting/handler/whatsapp_test.go
git commit -m "feat(accounting): add WhatsApp reimbursement endpoint with message parsing and item matching"
```

---

## Task 6: Wire Phase 2 Routes + Admin Types + Sidebar

**Files:**
- Modify: `api/internal/router/router.go`
- Modify: `admin/src/lib/types/api.ts`
- Modify: `admin/src/lib/components/Sidebar.svelte`

**Step 1: Add routes to router.go**

In `api/internal/router/router.go`, inside the existing accounting `r.Group` block (after purchase routes), add:

```go
// Reimbursements + WhatsApp
whatsappHandler := accthandler.NewWhatsAppHandler(queries)
reimbursementHandler := accthandler.NewReimbursementHandler(queries, pool)
r.Route("/accounting/reimbursements", func(r chi.Router) {
	reimbursementHandler.RegisterRoutes(r)
	// WhatsApp endpoint registered INSIDE the route group.
	// Chi matches literal paths ("/from-whatsapp") before wildcards ("/{id}"),
	// so this won't conflict with PUT /{id}.
	r.Post("/from-whatsapp", whatsappHandler.FromWhatsApp)
})
```

**Note on WhatsApp endpoint auth:** This endpoint is inside the OWNER-only auth group. n8n (the automation that calls this) must use a JWT token from a user with OWNER role (e.g., a service account). If you prefer webhook-style auth (API key), add a separate middleware or move this endpoint outside the auth group with its own key validation. For now, n8n using the owner's JWT is the simplest approach.

**Step 2: Add types to api.ts**

Append to `admin/src/lib/types/api.ts`:

```typescript
export interface AcctReimbursementRequest {
	id: string;
	batch_id: string | null;
	expense_date: string;
	item_id: string | null;
	description: string;
	qty: string;
	unit_price: string;
	amount: string;
	line_type: string;
	account_id: string;
	status: 'Draft' | 'Ready' | 'Posted';
	requester: string;
	receipt_link: string | null;
	posted_at: string | null;
	created_at: string;
}

export interface ReimbursementStatusCount {
	status: string;
	count: number;
	total_amount: string;
}
```

**Step 3: Add sidebar item**

In `admin/src/lib/components/Sidebar.svelte`, add to the `keuanganItems` array:

```typescript
const keuanganItems: NavItem[] = [
    { label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
    { label: 'Reimburse', href: '/accounting/reimbursements', icon: '##', roles: ['OWNER'] },
    { label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] }
];
```

**Step 4: Verify compile**

Run: `cd api && go build ./...`

Expected: No errors.

**Step 5: Commit**

```bash
git add api/internal/router/router.go admin/src/lib/types/api.ts admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): wire reimbursement routes, add admin types, add sidebar item"
```

---

## Task 7: Admin Page — Reimbursement

**Files:**
- Create: `admin/src/routes/(app)/accounting/reimbursements/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/reimbursements/+page.svelte`

**Step 1: Write server load + actions**

Create `admin/src/routes/(app)/accounting/reimbursements/+page.server.ts`:

```typescript
import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type {
	AcctReimbursementRequest,
	AcctItem,
	AcctAccount,
	AcctCashAccount,
	ReimbursementStatusCount
} from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') redirect(302, '/');

	const accessToken = cookies.get('access_token')!;
	const status = url.searchParams.get('status') ?? '';

	const queryParams = new URLSearchParams({ limit: '200', offset: '0' });
	if (status) queryParams.set('status', status);

	const [reimbResult, countsResult, itemsResult, accountsResult, cashAccountsResult] =
		await Promise.all([
			apiRequest<AcctReimbursementRequest[]>(
				`/accounting/reimbursements?${queryParams}`,
				{ accessToken }
			),
			apiRequest<ReimbursementStatusCount[]>('/accounting/reimbursements/counts', {
				accessToken
			}),
			apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
			apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
			apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken })
		]);

	return {
		reimbursements: reimbResult.ok ? reimbResult.data : [],
		counts: countsResult.ok ? countsResult.data : [],
		items: itemsResult.ok ? itemsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		currentStatus: status
	};
};

export const actions: Actions = {
	update: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString();
		const dataStr = formData.get('update_data')?.toString() ?? '';

		let body;
		try {
			body = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest(`/accounting/reimbursements/${id}`, {
			method: 'PUT',
			body,
			accessToken
		});

		if (!result.ok) return fail(result.status || 400, { error: result.message });
		return { success: true };
	},

	postBatch: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const dataStr = formData.get('batch_data')?.toString() ?? '';

		let body;
		try {
			body = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest('/accounting/reimbursements/post-batch', {
			method: 'POST',
			body,
			accessToken
		});

		if (!result.ok) return fail(result.status || 400, { error: result.message });
		return { batchSuccess: true, batchResult: result.ok ? result.data : null };
	}
};
```

**Step 2: Write page component**

Create `admin/src/routes/(app)/accounting/reimbursements/+page.svelte`:

Key features (follow existing purchase page patterns):

- **Status tabs**: Semua / Draft (N) / Siap (N) / Posted — use `$state` for active tab, status counts from API
- **Reimbursement table**: Date, Description, Item, Qty, Price, Total, Requester, Status, Actions
- **Edit modal/inline**: For Draft items — item autocomplete (same as purchase page), qty/price fields, status change to Ready
- **Batch posting**: Checkboxes on Ready items, "Posting Batch" button → dialog with payment_date + cash_account picker
- **Formatting**: `formatRupiah` for amounts, `formatDate` for dates
- **Form handling**: `use:enhance` with JSON serialization in callback (same pattern as purchase page)

This page is large (~500-800 lines). The implementing engineer should follow the exact patterns from `purchases/+page.svelte` for:
- `$state` for form state and modal visibility
- `$derived` for computed values (filtered list, totals)
- `use:enhance` for SPA-like form submission
- Scoped `<style>` with CSS variables

**Step 3: Verify it renders**

Run: `cd admin && pnpm dev` — navigate to `/accounting/reimbursements`.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/reimbursements/
git commit -m "feat(accounting): add reimbursement admin page with review, matching, and batch posting"
```

---

## Task 8: Verify Phase 2 Build + Tests

**Step 1: Run Go tests**

Run: `cd api && go test ./... -v`

Expected: All tests pass (existing + new parser + reimbursement + whatsapp tests).

**Step 2: Run admin build**

Run: `cd admin && pnpm build`

Expected: No type errors. Build succeeds.

**Step 3: Verify API compiles**

Run: `cd api && go build ./cmd/server/`

Expected: Binary compiles clean.

**Step 4: Commit any fixes**

Fix any issues found, commit.

---

## Phase 2 Checklist

| # | Task | Delivers |
|---|------|----------|
| 1 | sqlc queries | Reimbursement CRUD + account lookup by code |
| 2 | WhatsApp parser | `parser.go` with Indonesian date, price shortcuts, qty extraction |
| 3 | Reimbursement handler | CRUD + list with filters + status counts |
| 4 | Batch posting | Two-leg entries in DB transaction, batch_id generation |
| 5 | WhatsApp handler | Message parsing → item matching → reimbursement creation → formatted reply |
| 6 | Router + types + sidebar | Wiring, TypeScript types, sidebar nav item |
| 7 | Admin page | Status tabs, edit/review, item matching, batch posting dialog |
| 8 | Verify build | Full test suite + build verification |

**NOT in Phase 2:** n8n workflow changes (done separately on the n8n side after API is deployed), reports, sales, payroll.
