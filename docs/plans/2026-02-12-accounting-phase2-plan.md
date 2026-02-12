# Accounting Phase 2: Reimbursement Workflow + WhatsApp Integration

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build reimbursement CRUD, batch posting, WhatsApp message parsing, and admin UI — replacing the GSheet REIMBURSEMENT_REQUEST sheet and the fragile n8n+Apps Script workflow.

**Architecture:** Reimbursement requests flow in via WhatsApp (parsed by Go) or manual admin entry. Owner reviews/matches items, assigns batches, and posts them — creating `acct_cash_transactions` entries. The n8n workflow shrinks from 13 nodes to 5 (thin relay to Go API).

**Tech Stack:** Go (Chi, sqlc, pgx/v5, shopspring/decimal), SvelteKit 2 (Svelte 5, Tailwind CSS 4), PostgreSQL 16.

---

## Conventions Reference

All patterns are established in Phase 1. Key files for reference:

| Pattern | Reference File |
|---------|---------------|
| sqlc query with optional filters | `api/queries/acct_cash_transactions.sql` (sqlc.narg pattern) |
| Handler + store interface | `api/internal/accounting/handler/purchase.go` (PurchaseStore) |
| Mock store + httptest | `api/internal/accounting/handler/purchase_test.go` |
| pgtype.Numeric ↔ string | `api/internal/accounting/handler/master.go:numericToStringPtr()` |
| Transaction code generation | `api/internal/accounting/handler/purchase.go:150-159` |
| Item matching engine | `api/internal/accounting/matcher/matcher.go` |
| Admin page server load | `admin/src/routes/(app)/accounting/purchases/+page.server.ts` |
| Admin page with forms | `admin/src/routes/(app)/accounting/purchases/+page.svelte` |
| Router wiring | `api/internal/router/router.go:70-83` |
| TypeScript API types | `admin/src/lib/types/api.ts:290-342` |
| Sidebar nav items | `admin/src/lib/components/Sidebar.svelte:24-27` |

**Key conventions:**
- Consumer-defines-interface: each handler file defines its own store interface
- Money: `shopspring/decimal` for math, `string` in JSON, `pgtype.Numeric` in DB
- Nullable: `pgtype.Text`, `pgtype.UUID`, `pgtype.Numeric` for nullable DB fields
- Errors: 400 validation, 404 `pgx.ErrNoRows`, 409 pgconn `23505`, 500 internal
- Tests: `handler_test` package, mock stores, `httptest`, `chi.NewRouter()`
- Admin: `+page.server.ts` load + actions, `use:enhance`, Svelte 5 `$state()`/`$derived()`

---

## Task Overview

| # | Task | Files | Tests |
|---|------|-------|-------|
| 1 | sqlc Queries — Reimbursement Requests | 1 query file, regenerate | — |
| 2 | WhatsApp Message Parser | 2 Go files | 10+ tests |
| 3 | Reimbursement CRUD Handler | 2 Go files | 10+ tests |
| 4 | Batch Posting Handler | extend task 3 files | 4+ tests |
| 5 | WhatsApp Endpoint Handler | 2 Go files | 5+ tests |
| 6 | Wire Routes | 1 Go file | — |
| 7 | Admin TypeScript Types | 1 TS file | — |
| 8 | Sidebar Update | 1 Svelte file | — |
| 9 | Admin Reimbursement Page | 2 files (server + page) | — |
| 10 | Build Verification | — | all tests |

---

## Task 1: sqlc Queries — Reimbursement Requests

**Files:**
- Create: `api/queries/acct_reimbursement_requests.sql`
- Regenerate: `api/internal/database/` (run `sqlc generate`)

**Step 1: Write query file**

Create `api/queries/acct_reimbursement_requests.sql`:

```sql
-- name: ListAcctReimbursementRequests :many
SELECT * FROM acct_reimbursement_requests
WHERE
    (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')) AND
    (sqlc.narg('requester')::text IS NULL OR requester = sqlc.narg('requester')) AND
    (sqlc.narg('batch_id')::text IS NULL OR batch_id = sqlc.narg('batch_id')) AND
    (sqlc.narg('start_date')::date IS NULL OR expense_date >= sqlc.narg('start_date')) AND
    (sqlc.narg('end_date')::date IS NULL OR expense_date <= sqlc.narg('end_date'))
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetAcctReimbursementRequest :one
SELECT * FROM acct_reimbursement_requests WHERE id = $1;

-- name: CreateAcctReimbursementRequest :one
INSERT INTO acct_reimbursement_requests (
    expense_date, item_id, description, qty, unit_price, amount,
    line_type, account_id, status, requester, receipt_link
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: UpdateAcctReimbursementRequest :one
UPDATE acct_reimbursement_requests
SET item_id = $2, description = $3, qty = $4, unit_price = $5,
    amount = $6, line_type = $7, account_id = $8, status = $9
WHERE id = $1 AND status != 'Posted'
RETURNING *;

-- name: DeleteAcctReimbursementRequest :one
DELETE FROM acct_reimbursement_requests
WHERE id = $1 AND status = 'Draft'
RETURNING id;

-- name: AssignReimbursementBatch :exec
UPDATE acct_reimbursement_requests
SET batch_id = $1, status = 'Ready'
WHERE id = $2 AND status = 'Draft';

-- name: ListReimbursementsByBatch :many
SELECT * FROM acct_reimbursement_requests
WHERE batch_id = $1
ORDER BY created_at;

-- name: PostReimbursementBatch :exec
UPDATE acct_reimbursement_requests
SET status = 'Posted', posted_at = now()
WHERE batch_id = $1 AND status = 'Ready';

-- name: CheckBatchPosted :one
SELECT EXISTS(
    SELECT 1 FROM acct_reimbursement_requests
    WHERE batch_id = $1 AND status = 'Posted'
)::boolean AS is_posted;

-- name: GetNextBatchCode :one
SELECT COALESCE(MAX(batch_id), 'RMB000')::text AS max_code
FROM acct_reimbursement_requests
WHERE batch_id IS NOT NULL;
```

**Step 2: Run sqlc generate**

```bash
cd api && export PATH=$PATH:~/go/bin && sqlc generate
```

Expected: generates new functions in `api/internal/database/acct_reimbursement_requests.sql.go`. Verify the file exists and contains `ListAcctReimbursementRequests`, `CreateAcctReimbursementRequest`, etc.

**Step 3: Verify compilation**

```bash
cd api && go build ./...
```

Expected: compiles without errors.

**Step 4: Commit**

```bash
git add api/queries/acct_reimbursement_requests.sql api/internal/database/
git commit -m "feat(accounting): add sqlc queries for reimbursement requests"
```

---

## Task 2: WhatsApp Message Parser

**Files:**
- Create: `api/internal/accounting/parser/parser.go`
- Create: `api/internal/accounting/parser/parser_test.go`

The parser converts raw WhatsApp message text into structured data. It handles Indonesian date parsing and price shortcuts (500k → 500000, 1.5jt → 1500000).

**Step 1: Write the failing tests**

Create `api/internal/accounting/parser/parser_test.go`:

```go
package parser

import (
	"testing"
	"time"
)

func TestParseDateLine(t *testing.T) {
	tests := []struct {
		input string
		month time.Month
		day   int
		ok    bool
	}{
		{"20 jan", time.January, 20, true},
		{"5 feb", time.February, 5, true},
		{"15 mei", time.May, 15, true},
		{"1 agu", time.August, 1, true},
		{"31 des", time.December, 31, true},
		{"10 oktober", time.October, 10, true},
		{"not a date", 0, 0, false},
		{"", 0, 0, false},
		{"cabe merah 5kg", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			date, ok := parseDateLine(tt.input)
			if ok != tt.ok {
				t.Fatalf("parseDateLine(%q): got ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if !ok {
				return
			}
			if date.Month() != tt.month || date.Day() != tt.day {
				t.Errorf("parseDateLine(%q): got %v-%v, want %v-%v",
					tt.input, date.Month(), date.Day(), tt.month, tt.day)
			}
		})
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		input string
		price float64
		ok    bool
	}{
		{"500k", 500000, true},
		{"72k", 72000, true},
		{"1.5jt", 1500000, true},
		{"2jt", 2000000, true},
		{"300rb", 300000, true},
		{"25.5k", 25500, true},
		{"cabe", 0, false},
		{"5kg", 0, false},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			price, ok := parsePrice(tt.input)
			if ok != tt.ok {
				t.Fatalf("parsePrice(%q): got ok=%v, want %v", tt.input, ok, tt.ok)
			}
			if ok && price != tt.price {
				t.Errorf("parsePrice(%q): got %v, want %v", tt.input, price, tt.price)
			}
		})
	}
}

func TestParseItemLine(t *testing.T) {
	tests := []struct {
		input       string
		description string
		qty         float64
		unit        string
		totalPrice  float64
	}{
		{"cabe merah tanjung 5kg 500k", "cabe merah tanjung", 5, "kg", 500000},
		{"beras sania 20kg 300k", "beras sania", 20, "kg", 300000},
		{"minyak 2L 72k", "minyak", 2, "l", 72000},
		{"gas 12kg 200k", "gas", 12, "kg", 200000},
		{"bensin 100k", "bensin", 1, "", 100000},
		{"tisu 3pack 45k", "tisu", 3, "pack", 45000},
		{"garam 10bks 50k", "garam", 10, "bks", 50000},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			item, err := parseItemLine(tt.input)
			if err != nil {
				t.Fatalf("parseItemLine(%q): %v", tt.input, err)
			}
			if item.Description != tt.description {
				t.Errorf("description: got %q, want %q", item.Description, tt.description)
			}
			if item.Qty != tt.qty {
				t.Errorf("qty: got %v, want %v", item.Qty, tt.qty)
			}
			if item.Unit != tt.unit {
				t.Errorf("unit: got %q, want %q", item.Unit, tt.unit)
			}
			if item.TotalPrice != tt.totalPrice {
				t.Errorf("totalPrice: got %v, want %v", item.TotalPrice, tt.totalPrice)
			}
		})
	}
}

func TestParseMessage(t *testing.T) {
	msg := "20 jan\ncabe merah tanjung 5kg 500k\nberas sania 20kg 300k\nminyak 2L 72k"

	result, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	if result.ExpenseDate.Month() != time.January || result.ExpenseDate.Day() != 20 {
		t.Errorf("date: got %v, want Jan 20", result.ExpenseDate)
	}
	if len(result.Items) != 3 {
		t.Fatalf("items: got %d, want 3", len(result.Items))
	}
	if result.Items[0].Description != "cabe merah tanjung" {
		t.Errorf("item 0 desc: got %q", result.Items[0].Description)
	}
	if result.Items[0].TotalPrice != 500000 {
		t.Errorf("item 0 price: got %v", result.Items[0].TotalPrice)
	}
	if result.Items[2].Description != "minyak" {
		t.Errorf("item 2 desc: got %q", result.Items[2].Description)
	}
}

func TestParseMessage_NoDate(t *testing.T) {
	msg := "cabe merah 5kg 500k\nberas 20kg 300k"

	_, err := ParseMessage(msg)
	if err == nil {
		t.Fatal("expected error for missing date")
	}
}

func TestParseMessage_EmptyLines(t *testing.T) {
	msg := "20 jan\n\ncabe merah 5kg 500k\n\nberas 20kg 300k\n"

	result, err := ParseMessage(msg)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items: got %d, want 2", len(result.Items))
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/accounting/parser/ -v
```

Expected: compilation error — `parser` package doesn't exist yet.

**Step 3: Implement the parser**

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

// ParsedMessage is the result of parsing a WhatsApp reimbursement message.
type ParsedMessage struct {
	ExpenseDate time.Time
	Items       []ParsedItem
}

// ParsedItem is a single item parsed from a WhatsApp message line.
type ParsedItem struct {
	RawText     string
	Description string
	Qty         float64
	Unit        string
	TotalPrice  float64
}

var indonesianMonths = map[string]time.Month{
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

// Known quantity units (NOT price suffixes).
var qtyUnits = map[string]bool{
	"kg": true, "g": true, "l": true, "ml": true,
	"pcs": true, "bks": true, "pack": true, "box": true,
	"ikat": true, "iket": true, "lbr": true, "btl": true,
	"ltr": true, "buah": true, "bh": true, "lembar": true,
	"sdm": true, "sdt": true, "ekor": true, "btr": true,
}

// ParseMessage parses a WhatsApp reimbursement message into structured data.
// First non-empty line must be a date (e.g. "20 jan"). Subsequent lines are items.
func ParseMessage(text string) (*ParsedMessage, error) {
	lines := strings.Split(text, "\n")

	var expenseDate time.Time
	var dateFound bool
	var items []ParsedItem

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !dateFound {
			date, ok := parseDateLine(line)
			if ok {
				expenseDate = date
				dateFound = true
				continue
			}
			return nil, fmt.Errorf("first line must be a date, got: %q", line)
		}

		item, err := parseItemLine(line)
		if err != nil {
			continue // skip unparseable lines
		}
		items = append(items, *item)
	}

	if !dateFound {
		return nil, fmt.Errorf("no date found in message")
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no items found in message")
	}

	return &ParsedMessage{
		ExpenseDate: expenseDate,
		Items:       items,
	}, nil
}

// parseDateLine tries to parse a line as an Indonesian date (e.g. "20 jan").
func parseDateLine(line string) (time.Time, bool) {
	line = strings.TrimSpace(strings.ToLower(line))
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return time.Time{}, false
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}

	month, ok := indonesianMonths[parts[1]]
	if !ok {
		return time.Time{}, false
	}

	year := time.Now().Year()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), true
}

// parseItemLine parses a single item line (e.g. "cabe merah 5kg 500k").
func parseItemLine(line string) (*ParsedItem, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	tokens := strings.Fields(strings.ToLower(line))

	var totalPrice float64
	var qty float64 = 1
	var unit string
	var descTokens []string
	var priceFound, qtyFound bool

	for _, tok := range tokens {
		if p, ok := parsePrice(tok); ok && !priceFound {
			totalPrice = p
			priceFound = true
		} else if q, u, ok := parseQtyUnitToken(tok); ok && !qtyFound {
			qty = q
			unit = u
			qtyFound = true
		} else {
			descTokens = append(descTokens, tok)
		}
	}

	return &ParsedItem{
		RawText:     line,
		Description: strings.Join(descTokens, " "),
		Qty:         qty,
		Unit:        unit,
		TotalPrice:  totalPrice,
	}, nil
}

// parsePrice parses price shortcuts: "500k" → 500000, "1.5jt" → 1500000, "300rb" → 300000.
func parsePrice(tok string) (float64, bool) {
	tok = strings.ToLower(tok)

	type suffix struct {
		s string
		m float64
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
				continue
			}
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				continue
			}
			return num * sf.m, true
		}
	}

	return 0, false
}

// parseQtyUnitToken parses "5kg" → (5, "kg", true). Only matches known units.
func parseQtyUnitToken(tok string) (float64, string, bool) {
	if tok == "" {
		return 0, "", false
	}

	// Find boundary between digits and letters
	digitEnd := 0
	for i, r := range tok {
		if unicode.IsDigit(r) || r == '.' {
			digitEnd = i + 1
		} else {
			break
		}
	}

	if digitEnd == 0 || digitEnd == len(tok) {
		return 0, "", false
	}

	numPart := tok[:digitEnd]
	unitPart := tok[digitEnd:]

	// Must be a known quantity unit
	if !qtyUnits[unitPart] {
		return 0, "", false
	}

	qty, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, "", false
	}

	return qty, unitPart, true
}
```

**Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/accounting/parser/ -v
```

Expected: all tests pass.

**Step 5: Commit**

```bash
git add api/internal/accounting/parser/
git commit -m "feat(accounting): add WhatsApp message parser with Indonesian date and price shortcuts"
```

---

## Task 3: Reimbursement CRUD Handler

**Files:**
- Create: `api/internal/accounting/handler/reimbursement.go`
- Create: `api/internal/accounting/handler/reimbursement_test.go`

**Step 1: Write the failing tests**

Create `api/internal/accounting/handler/reimbursement_test.go`:

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

// --- Mock Reimbursement Store ---

type mockReimbursementStore struct {
	requests   map[uuid.UUID]database.AcctReimbursementRequest
	nextBatch  string
	nextTxCode string
	txns       []database.AcctCashTransaction
}

func newMockReimbursementStore() *mockReimbursementStore {
	return &mockReimbursementStore{
		requests:   make(map[uuid.UUID]database.AcctReimbursementRequest),
		nextBatch:  "RMB000",
		nextTxCode: "PCS000000",
		txns:       []database.AcctCashTransaction{},
	}
}

func (m *mockReimbursementStore) ListAcctReimbursementRequests(ctx context.Context, arg database.ListAcctReimbursementRequestsParams) ([]database.AcctReimbursementRequest, error) {
	var result []database.AcctReimbursementRequest
	for _, r := range m.requests {
		result = append(result, r)
	}
	return result, nil
}

func (m *mockReimbursementStore) GetAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (database.AcctReimbursementRequest, error) {
	r, ok := m.requests[id]
	if !ok {
		return database.AcctReimbursementRequest{}, pgx.ErrNoRows
	}
	return r, nil
}

func (m *mockReimbursementStore) CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r := database.AcctReimbursementRequest{
		ID:          uuid.New(),
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
	m.requests[r.ID] = r
	return r, nil
}

func (m *mockReimbursementStore) UpdateAcctReimbursementRequest(ctx context.Context, arg database.UpdateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r, ok := m.requests[arg.ID]
	if !ok || r.Status == "Posted" {
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
	m.requests[r.ID] = r
	return r, nil
}

func (m *mockReimbursementStore) DeleteAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	r, ok := m.requests[id]
	if !ok || r.Status != "Draft" {
		return uuid.UUID{}, pgx.ErrNoRows
	}
	delete(m.requests, id)
	return id, nil
}

func (m *mockReimbursementStore) AssignReimbursementBatch(ctx context.Context, arg database.AssignReimbursementBatchParams) error {
	r, ok := m.requests[arg.ID]
	if !ok || r.Status != "Draft" {
		return nil // exec returns no error on 0 rows
	}
	r.BatchID = pgtype.Text{String: arg.BatchID.String, Valid: true}
	r.Status = "Ready"
	m.requests[r.ID] = r
	return nil
}

func (m *mockReimbursementStore) ListReimbursementsByBatch(ctx context.Context, batchID pgtype.Text) ([]database.AcctReimbursementRequest, error) {
	var result []database.AcctReimbursementRequest
	for _, r := range m.requests {
		if r.BatchID.Valid && r.BatchID.String == batchID.String {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *mockReimbursementStore) PostReimbursementBatch(ctx context.Context, batchID pgtype.Text) error {
	for id, r := range m.requests {
		if r.BatchID.Valid && r.BatchID.String == batchID.String && r.Status == "Ready" {
			r.Status = "Posted"
			r.PostedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
			m.requests[id] = r
		}
	}
	return nil
}

func (m *mockReimbursementStore) CheckBatchPosted(ctx context.Context, batchID pgtype.Text) (bool, error) {
	for _, r := range m.requests {
		if r.BatchID.Valid && r.BatchID.String == batchID.String && r.Status == "Posted" {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockReimbursementStore) GetNextBatchCode(ctx context.Context) (string, error) {
	return m.nextBatch, nil
}

func (m *mockReimbursementStore) CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
	tx := database.AcctCashTransaction{
		ID:              uuid.New(),
		TransactionCode: arg.TransactionCode,
		TransactionDate: arg.TransactionDate,
		Description:     arg.Description,
		LineType:        arg.LineType,
		AccountID:       arg.AccountID,
		CreatedAt:       time.Now(),
	}
	m.txns = append(m.txns, tx)
	m.nextTxCode = arg.TransactionCode
	return tx, nil
}

func (m *mockReimbursementStore) GetNextTransactionCode(ctx context.Context) (string, error) {
	return m.nextTxCode, nil
}

// --- Router helper ---

func setupReimbursementRouter(store handler.ReimbursementStore) *chi.Mux {
	h := handler.NewReimbursementHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/reimbursements", h.RegisterRoutes)
	return r
}

// --- Tests ---

func TestReimbursementList_Empty(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/reimbursements/?limit=20&offset=0", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestReimbursementCreate_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	accountID := uuid.New()
	body := map[string]interface{}{
		"expense_date": "2026-01-20",
		"description":  "Cabe merah tanjung 5kg",
		"qty":          "5.00",
		"unit_price":   "100000.00",
		"amount":       "500000.00",
		"line_type":    "INVENTORY",
		"account_id":   accountID.String(),
		"status":       "Draft",
		"requester":    "Hamidah",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeJSON(t, rr)
	if resp["requester"] != "Hamidah" {
		t.Errorf("expected requester Hamidah, got %v", resp["requester"])
	}
	if resp["status"] != "Draft" {
		t.Errorf("expected status Draft, got %v", resp["status"])
	}
}

func TestReimbursementCreate_MissingFields(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	body := map[string]interface{}{
		"description": "Test",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestReimbursementUpdate_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	// Seed a Draft request
	id := uuid.New()
	accountID := uuid.New()
	store.requests[id] = database.AcctReimbursementRequest{
		ID:        id,
		Status:    "Draft",
		AccountID: accountID,
		LineType:  "EXPENSE",
		CreatedAt: time.Now(),
	}

	router := setupReimbursementRouter(store)

	newAccountID := uuid.New()
	body := map[string]interface{}{
		"description": "Updated description",
		"qty":         "3.00",
		"unit_price":  "50000.00",
		"amount":      "150000.00",
		"line_type":   "INVENTORY",
		"account_id":  newAccountID.String(),
		"status":      "Ready",
	}

	rr := doRequest(t, router, "PUT", "/accounting/reimbursements/"+id.String(), body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeJSON(t, rr)
	if resp["status"] != "Ready" {
		t.Errorf("expected status Ready, got %v", resp["status"])
	}
}

func TestReimbursementDelete_DraftOnly(t *testing.T) {
	store := newMockReimbursementStore()
	draftID := uuid.New()
	readyID := uuid.New()
	accountID := uuid.New()

	store.requests[draftID] = database.AcctReimbursementRequest{
		ID: draftID, Status: "Draft", AccountID: accountID, CreatedAt: time.Now(),
	}
	store.requests[readyID] = database.AcctReimbursementRequest{
		ID: readyID, Status: "Ready", AccountID: accountID, CreatedAt: time.Now(),
	}

	router := setupReimbursementRouter(store)

	// Delete Draft — should succeed
	rr := doRequest(t, router, "DELETE", "/accounting/reimbursements/"+draftID.String(), nil)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}

	// Delete Ready — should fail
	rr = doRequest(t, router, "DELETE", "/accounting/reimbursements/"+readyID.String(), nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestReimbursementGet_NotFound(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	rr := doRequest(t, router, "GET", "/accounting/reimbursements/"+uuid.New().String(), nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestReimbursement
```

Expected: compilation error — `ReimbursementStore`, `NewReimbursementHandler` not defined.

**Step 3: Implement the handler**

Create `api/internal/accounting/handler/reimbursement.go`:

```go
package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// ReimbursementStore defines the database methods needed by reimbursement handlers.
type ReimbursementStore interface {
	ListAcctReimbursementRequests(ctx context.Context, arg database.ListAcctReimbursementRequestsParams) ([]database.AcctReimbursementRequest, error)
	GetAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (database.AcctReimbursementRequest, error)
	CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	UpdateAcctReimbursementRequest(ctx context.Context, arg database.UpdateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	DeleteAcctReimbursementRequest(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	AssignReimbursementBatch(ctx context.Context, arg database.AssignReimbursementBatchParams) error
	ListReimbursementsByBatch(ctx context.Context, batchID pgtype.Text) ([]database.AcctReimbursementRequest, error)
	PostReimbursementBatch(ctx context.Context, batchID pgtype.Text) error
	CheckBatchPosted(ctx context.Context, batchID pgtype.Text) (bool, error)
	GetNextBatchCode(ctx context.Context) (string, error)
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- Handler ---

// ReimbursementHandler handles reimbursement request endpoints.
type ReimbursementHandler struct {
	store ReimbursementStore
}

// NewReimbursementHandler creates a new ReimbursementHandler.
func NewReimbursementHandler(store ReimbursementStore) *ReimbursementHandler {
	return &ReimbursementHandler{store: store}
}

// RegisterRoutes registers reimbursement endpoints.
func (h *ReimbursementHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListReimbursements)
	r.Post("/", h.CreateReimbursement)
	r.Get("/{id}", h.GetReimbursement)
	r.Put("/{id}", h.UpdateReimbursement)
	r.Delete("/{id}", h.DeleteReimbursement)
	r.Post("/batch", h.AssignBatch)
	r.Post("/batch/post", h.PostBatch)
}

// --- Request / Response types ---

type createReimbursementRequest struct {
	ExpenseDate string  `json:"expense_date"` // "2026-01-20"
	ItemID      *string `json:"item_id"`      // optional UUID
	Description string  `json:"description"`
	Qty         string  `json:"qty"`        // decimal string
	UnitPrice   string  `json:"unit_price"` // decimal string
	Amount      string  `json:"amount"`     // decimal string
	LineType    string  `json:"line_type"`  // INVENTORY|EXPENSE
	AccountID   string  `json:"account_id"` // UUID
	Status      string  `json:"status"`     // Draft|Ready
	Requester   string  `json:"requester"`
	ReceiptLink *string `json:"receipt_link"` // optional URL
}

type updateReimbursementRequest struct {
	ItemID      *string `json:"item_id"`
	Description string  `json:"description"`
	Qty         string  `json:"qty"`
	UnitPrice   string  `json:"unit_price"`
	Amount      string  `json:"amount"`
	LineType    string  `json:"line_type"`
	AccountID   string  `json:"account_id"`
	Status      string  `json:"status"`
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
	AccountID   string     `json:"account_id"`
	Status      string     `json:"status"`
	Requester   string     `json:"requester"`
	ReceiptLink *string    `json:"receipt_link"`
	PostedAt    *time.Time `json:"posted_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

type assignBatchRequest struct {
	IDs []string `json:"ids"` // list of reimbursement UUIDs
}

type assignBatchResponse struct {
	BatchID  string `json:"batch_id"`
	Assigned int    `json:"assigned"`
}

type postBatchRequest struct {
	BatchID       string `json:"batch_id"`
	PaymentDate   string `json:"payment_date"`   // "2026-01-25"
	CashAccountID string `json:"cash_account_id"` // UUID
}

type postBatchResponse struct {
	BatchID      string                `json:"batch_id"`
	Posted       int                   `json:"posted"`
	Transactions []transactionResponse `json:"transactions"`
}

// --- Response converter ---

func toReimbursementResponse(r database.AcctReimbursementRequest) reimbursementResponse {
	resp := reimbursementResponse{
		ID:          r.ID,
		ExpenseDate: r.ExpenseDate.Time.Format("2006-01-02"),
		Description: r.Description,
		LineType:    r.LineType,
		AccountID:   r.AccountID.String(),
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

	resp.Qty = numericToString(r.Qty)
	resp.UnitPrice = numericToString(r.UnitPrice)
	resp.Amount = numericToString(r.Amount)

	return resp
}

func numericToString(n pgtype.Numeric) string {
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

func (h *ReimbursementHandler) ListReimbursements(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	params := database.ListAcctReimbursementRequestsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if s := r.URL.Query().Get("status"); s != "" {
		params.Status = pgtype.Text{String: s, Valid: true}
	}
	if s := r.URL.Query().Get("requester"); s != "" {
		params.Requester = pgtype.Text{String: s, Valid: true}
	}
	if s := r.URL.Query().Get("batch_id"); s != "" {
		params.BatchID = pgtype.Text{String: s, Valid: true}
	}
	if s := r.URL.Query().Get("start_date"); s != "" {
		if d, err := time.Parse("2006-01-02", s); err == nil {
			params.StartDate = pgtype.Date{Time: d, Valid: true}
		}
	}
	if s := r.URL.Query().Get("end_date"); s != "" {
		if d, err := time.Parse("2006-01-02", s); err == nil {
			params.EndDate = pgtype.Date{Time: d, Valid: true}
		}
	}

	requests, err := h.store.ListAcctReimbursementRequests(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list reimbursements: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	result := make([]reimbursementResponse, len(requests))
	for i, req := range requests {
		result[i] = toReimbursementResponse(req)
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *ReimbursementHandler) GetReimbursement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	req, err := h.store.GetAcctReimbursementRequest(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		log.Printf("ERROR: get reimbursement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toReimbursementResponse(req))
}

func (h *ReimbursementHandler) CreateReimbursement(w http.ResponseWriter, r *http.Request) {
	var req createReimbursementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ExpenseDate == "" || req.Description == "" || req.Qty == "" ||
		req.UnitPrice == "" || req.Amount == "" || req.LineType == "" ||
		req.AccountID == "" || req.Requester == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	// Validate status
	if req.Status == "" {
		req.Status = "Draft"
	}
	validStatuses := map[string]bool{"Draft": true, "Ready": true}
	if !validStatuses[req.Status] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be Draft or Ready"})
		return
	}

	// Validate line_type
	validLineTypes := map[string]bool{"INVENTORY": true, "EXPENSE": true}
	if !validLineTypes[req.LineType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type must be INVENTORY or EXPENSE"})
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

	// Parse decimals
	qty, err := decimal.NewFromString(req.Qty)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty"})
		return
	}
	unitPrice, err := decimal.NewFromString(req.UnitPrice)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price"})
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount"})
		return
	}

	// Convert to pgtype
	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan(qty.StringFixed(4))
	pricePg.Scan(unitPrice.StringFixed(2))
	amountPg.Scan(amount.StringFixed(2))

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

	created, err := h.store.CreateAcctReimbursementRequest(r.Context(), database.CreateAcctReimbursementRequestParams{
		ExpenseDate: pgtype.Date{Time: date, Valid: true},
		ItemID:      itemID,
		Description: req.Description,
		Qty:         qtyPg,
		UnitPrice:   pricePg,
		Amount:      amountPg,
		LineType:    req.LineType,
		AccountID:   accountID,
		Status:      req.Status,
		Requester:   req.Requester,
		ReceiptLink: stringToPgText(req.ReceiptLink),
	})
	if err != nil {
		log.Printf("ERROR: create reimbursement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toReimbursementResponse(created))
}

func (h *ReimbursementHandler) UpdateReimbursement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req updateReimbursementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Description == "" || req.Qty == "" || req.UnitPrice == "" ||
		req.Amount == "" || req.LineType == "" || req.AccountID == "" || req.Status == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
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

	qty, err := decimal.NewFromString(req.Qty)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid qty"})
		return
	}
	unitPrice, err := decimal.NewFromString(req.UnitPrice)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price"})
		return
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount"})
		return
	}

	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan(qty.StringFixed(4))
	pricePg.Scan(unitPrice.StringFixed(2))
	amountPg.Scan(amount.StringFixed(2))

	var itemID pgtype.UUID
	if req.ItemID != nil && *req.ItemID != "" {
		parsed, err := uuid.Parse(*req.ItemID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
			return
		}
		itemID = uuidToPgUUID(parsed)
	}

	updated, err := h.store.UpdateAcctReimbursementRequest(r.Context(), database.UpdateAcctReimbursementRequestParams{
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
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found or already posted"})
			return
		}
		log.Printf("ERROR: update reimbursement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toReimbursementResponse(updated))
}

func (h *ReimbursementHandler) DeleteReimbursement(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	_, err = h.store.DeleteAcctReimbursementRequest(r.Context(), id)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found or not Draft"})
			return
		}
		log.Printf("ERROR: delete reimbursement: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ReimbursementHandler) AssignBatch(w http.ResponseWriter, r *http.Request) {
	var req assignBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids cannot be empty"})
		return
	}

	// Generate batch code
	maxCode, err := h.store.GetNextBatchCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next batch code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	var nextNum int
	if len(maxCode) > 3 {
		numStr := maxCode[3:]
		nextNum, _ = strconv.Atoi(numStr)
	}
	nextNum++
	batchID := fmt.Sprintf("RMB%03d", nextNum)

	batchText := pgtype.Text{String: batchID, Valid: true}

	assigned := 0
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		if err := h.store.AssignReimbursementBatch(r.Context(), database.AssignReimbursementBatchParams{
			BatchID: batchText,
			ID:      id,
		}); err != nil {
			log.Printf("ERROR: assign batch %s to %s: %v", batchID, idStr, err)
			continue
		}
		assigned++
	}

	writeJSON(w, http.StatusOK, assignBatchResponse{
		BatchID:  batchID,
		Assigned: assigned,
	})
}

func (h *ReimbursementHandler) PostBatch(w http.ResponseWriter, r *http.Request) {
	var req postBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.BatchID == "" || req.PaymentDate == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "batch_id, payment_date, and cash_account_id are required"})
		return
	}

	batchText := pgtype.Text{String: req.BatchID, Valid: true}

	// Check idempotency
	posted, err := h.store.CheckBatchPosted(r.Context(), batchText)
	if err != nil {
		log.Printf("ERROR: check batch posted: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if posted {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "batch already posted"})
		return
	}

	// Parse payment date
	paymentDate, err := time.Parse("2006-01-02", req.PaymentDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payment_date"})
		return
	}
	pgDate := pgtype.Date{Time: paymentDate, Valid: true}

	// Parse cash account ID
	cashAccountID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	// Get items in batch
	items, err := h.store.ListReimbursementsByBatch(r.Context(), batchText)
	if err != nil {
		log.Printf("ERROR: list batch items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	readyItems := make([]database.AcctReimbursementRequest, 0)
	for _, item := range items {
		if item.Status == "Ready" {
			readyItems = append(readyItems, item)
		}
	}

	if len(readyItems) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no Ready items in batch"})
		return
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next tx code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	numStr := maxCode[3:]
	nextNum, _ := strconv.Atoi(numStr)
	nextNum++

	// Create cash transactions for each item
	var transactions []transactionResponse
	for _, item := range readyItems {
		txCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		tx, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      txCode,
			TransactionDate:      pgDate,
			ItemID:               item.ItemID,
			Description:          item.Description,
			Quantity:             item.Qty,
			UnitPrice:            item.UnitPrice,
			Amount:               item.Amount,
			LineType:             item.LineType,
			AccountID:            item.AccountID,
			CashAccountID:        uuidToPgUUID(cashAccountID),
			OutletID:             pgtype.UUID{},
			ReimbursementBatchID: pgtype.Text{String: req.BatchID, Valid: true},
		})
		if err != nil {
			log.Printf("ERROR: create cash tx for batch %s: %v", req.BatchID, err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		var itemIDPtr *string
		if item.ItemID.Valid {
			idStr := uuid.UUID(item.ItemID.Bytes).String()
			itemIDPtr = &idStr
		}

		transactions = append(transactions, transactionResponse{
			ID:              tx.ID,
			TransactionCode: tx.TransactionCode,
			TransactionDate: req.PaymentDate,
			Description:     tx.Description,
			Quantity:        numericToString(item.Qty),
			UnitPrice:       numericToString(item.UnitPrice),
			Amount:          numericToString(item.Amount),
			LineType:        tx.LineType,
			ItemID:          itemIDPtr,
			CreatedAt:       tx.CreatedAt,
		})
	}

	// Mark batch as posted
	if err := h.store.PostReimbursementBatch(r.Context(), batchText); err != nil {
		log.Printf("ERROR: post batch %s: %v", req.BatchID, err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, postBatchResponse{
		BatchID:      req.BatchID,
		Posted:       len(transactions),
		Transactions: transactions,
	})
}
```

Note: This handler uses `fmt.Sprintf` — add `"fmt"` to imports. The `numericToString` helper converts `pgtype.Numeric` to `string` using the same `Value()` + `decimal` pattern from `master.go`.

**Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestReimbursement
```

Expected: all reimbursement tests pass.

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/reimbursement.go api/internal/accounting/handler/reimbursement_test.go
git commit -m "feat(accounting): add reimbursement CRUD and batch posting handlers"
```

---

## Task 4: Batch Posting Tests

Add tests for batch assign and batch post operations to `reimbursement_test.go`.

**Step 1: Add batch tests**

Append to `api/internal/accounting/handler/reimbursement_test.go`:

```go
func TestBatchAssign_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	accountID := uuid.New()

	// Seed Draft items
	id1 := uuid.New()
	id2 := uuid.New()
	store.requests[id1] = database.AcctReimbursementRequest{
		ID: id1, Status: "Draft", AccountID: accountID, CreatedAt: time.Now(),
	}
	store.requests[id2] = database.AcctReimbursementRequest{
		ID: id2, Status: "Draft", AccountID: accountID, CreatedAt: time.Now(),
	}

	router := setupReimbursementRouter(store)

	body := map[string]interface{}{
		"ids": []string{id1.String(), id2.String()},
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeJSON(t, rr)
	if resp["assigned"] != float64(2) {
		t.Errorf("expected assigned=2, got %v", resp["assigned"])
	}
	if resp["batch_id"] == nil || resp["batch_id"] == "" {
		t.Error("expected batch_id to be set")
	}

	// Verify items are now Ready
	if store.requests[id1].Status != "Ready" {
		t.Error("expected id1 to be Ready")
	}
}

func TestBatchPost_Valid(t *testing.T) {
	store := newMockReimbursementStore()
	accountID := uuid.New()

	// Seed Ready items with batch
	id1 := uuid.New()
	var qtyPg, pricePg, amountPg pgtype.Numeric
	qtyPg.Scan("5.0000")
	pricePg.Scan("100000.00")
	amountPg.Scan("500000.00")

	store.requests[id1] = database.AcctReimbursementRequest{
		ID:        id1,
		Status:    "Ready",
		BatchID:   pgtype.Text{String: "RMB001", Valid: true},
		AccountID: accountID,
		LineType:  "INVENTORY",
		Qty:       qtyPg,
		UnitPrice: pricePg,
		Amount:    amountPg,
		CreatedAt: time.Now(),
	}

	router := setupReimbursementRouter(store)

	cashAccountID := uuid.New()
	body := map[string]interface{}{
		"batch_id":        "RMB001",
		"payment_date":    "2026-01-25",
		"cash_account_id": cashAccountID.String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", body)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeJSON(t, rr)
	if resp["posted"] != float64(1) {
		t.Errorf("expected posted=1, got %v", resp["posted"])
	}

	// Verify item is now Posted
	if store.requests[id1].Status != "Posted" {
		t.Error("expected item to be Posted")
	}

	// Verify cash transaction was created
	if len(store.txns) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(store.txns))
	}
}

func TestBatchPost_AlreadyPosted(t *testing.T) {
	store := newMockReimbursementStore()
	accountID := uuid.New()

	// Seed a Posted item
	id1 := uuid.New()
	store.requests[id1] = database.AcctReimbursementRequest{
		ID:        id1,
		Status:    "Posted",
		BatchID:   pgtype.Text{String: "RMB001", Valid: true},
		AccountID: accountID,
		PostedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		CreatedAt: time.Now(),
	}

	router := setupReimbursementRouter(store)

	body := map[string]interface{}{
		"batch_id":        "RMB001",
		"payment_date":    "2026-01-25",
		"cash_account_id": uuid.New().String(),
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch/post", body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestBatchAssign_EmptyIDs(t *testing.T) {
	store := newMockReimbursementStore()
	router := setupReimbursementRouter(store)

	body := map[string]interface{}{
		"ids": []string{},
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/batch", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}
```

**Step 2: Run all reimbursement tests**

```bash
cd api && go test ./internal/accounting/handler/ -v -run "TestReimbursement|TestBatch"
```

Expected: all tests pass.

**Step 3: Commit**

```bash
git add api/internal/accounting/handler/reimbursement_test.go
git commit -m "test(accounting): add batch assign and post tests for reimbursements"
```

---

## Task 5: WhatsApp Endpoint Handler

**Files:**
- Create: `api/internal/accounting/handler/whatsapp.go`
- Create: `api/internal/accounting/handler/whatsapp_test.go`

**Step 1: Write the failing tests**

Create `api/internal/accounting/handler/whatsapp_test.go`:

```go
package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/accounting/matcher"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock WhatsApp Store ---

type mockWhatsAppStore struct {
	requests map[uuid.UUID]database.AcctReimbursementRequest
}

func newMockWhatsAppStore() *mockWhatsAppStore {
	return &mockWhatsAppStore{
		requests: make(map[uuid.UUID]database.AcctReimbursementRequest),
	}
}

func (m *mockWhatsAppStore) CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
	r := database.AcctReimbursementRequest{
		ID:          uuid.New(),
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
		CreatedAt:   time.Now(),
	}
	m.requests[r.ID] = r
	return r, nil
}

func (m *mockWhatsAppStore) ListAcctItems(ctx context.Context) ([]database.AcctItem, error) {
	return []database.AcctItem{}, nil
}

// --- Router helper ---

func setupWhatsAppRouter(store handler.WhatsAppStore, items []matcher.Item, defaultAccountID uuid.UUID) *chi.Mux {
	h := handler.NewWhatsAppHandler(store, items, defaultAccountID)
	r := chi.NewRouter()
	r.Route("/accounting/reimbursements", func(r chi.Router) {
		r.Post("/from-whatsapp", h.FromWhatsApp)
	})
	return r
}

// --- Tests ---

func TestFromWhatsApp_Valid(t *testing.T) {
	store := newMockWhatsAppStore()

	// Set up matcher items
	itemID := uuid.New()
	items := []matcher.Item{
		{ID: itemID, Code: "ITEM0012", Name: "Cabe Merah Tanjung", Keywords: "cabe,merah,tanjung", Unit: "kg"},
	}

	accountID := uuid.New()
	router := setupWhatsAppRouter(store, items, accountID)

	body := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"message_text": "20 jan\ncabe merah tanjung 5kg 500k",
		"chat_id":      "120363421848364675@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeJSON(t, rr)
	if resp["items_created"] != float64(1) {
		t.Errorf("expected items_created=1, got %v", resp["items_created"])
	}
	if resp["items_matched"] != float64(1) {
		t.Errorf("expected items_matched=1, got %v", resp["items_matched"])
	}
	if resp["reply_message"] == nil || resp["reply_message"] == "" {
		t.Error("expected reply_message to be set")
	}

	// Verify reimbursement was created
	if len(store.requests) != 1 {
		t.Fatalf("expected 1 request created, got %d", len(store.requests))
	}
}

func TestFromWhatsApp_InvalidMessage(t *testing.T) {
	store := newMockWhatsAppStore()
	accountID := uuid.New()
	router := setupWhatsAppRouter(store, nil, accountID)

	body := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Test",
		"message_text": "this is not a reimbursement",
		"chat_id":      "test@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestFromWhatsApp_MissingFields(t *testing.T) {
	store := newMockWhatsAppStore()
	accountID := uuid.New()
	router := setupWhatsAppRouter(store, nil, accountID)

	body := map[string]interface{}{
		"sender_name": "Test",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestFromWhatsApp_UnmatchedItem(t *testing.T) {
	store := newMockWhatsAppStore()
	accountID := uuid.New()
	// Empty items list — nothing to match
	router := setupWhatsAppRouter(store, []matcher.Item{}, accountID)

	body := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"message_text": "20 jan\nxyz unknown item 5kg 500k",
		"chat_id":      "test@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	resp := decodeJSON(t, rr)
	if resp["items_unmatched"] != float64(1) {
		t.Errorf("expected items_unmatched=1, got %v", resp["items_unmatched"])
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestFromWhatsApp
```

Expected: compilation error — `WhatsAppStore`, `NewWhatsAppHandler` not defined.

**Step 3: Implement the handler**

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
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// WhatsAppStore defines the database methods needed by the WhatsApp handler.
type WhatsAppStore interface {
	CreateAcctReimbursementRequest(ctx context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error)
	ListAcctItems(ctx context.Context) ([]database.AcctItem, error)
}

// --- Handler ---

// WhatsAppHandler handles the WhatsApp reimbursement webhook.
type WhatsAppHandler struct {
	store            WhatsAppStore
	matcher          *matcher.Matcher
	defaultAccountID uuid.UUID // default expense account for unmatched items
}

// NewWhatsAppHandler creates a new WhatsAppHandler.
// items: pre-loaded matcher items. defaultAccountID: fallback account for unmatched items.
func NewWhatsAppHandler(store WhatsAppStore, items []matcher.Item, defaultAccountID uuid.UUID) *WhatsAppHandler {
	var m *matcher.Matcher
	if len(items) > 0 {
		m = matcher.New(items)
	}
	return &WhatsAppHandler{
		store:            store,
		matcher:          m,
		defaultAccountID: defaultAccountID,
	}
}

// --- Request / Response types ---

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

// --- Handler ---

func (h *WhatsAppHandler) FromWhatsApp(w http.ResponseWriter, r *http.Request) {
	var req whatsAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.SenderName == "" || req.MessageText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sender_name and message_text are required"})
		return
	}

	// Parse the message
	parsed, err := parser.ParseMessage(req.MessageText)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":         "failed to parse message",
			"reply_message": fmt.Sprintf("Format salah: %s", err.Error()),
		})
		return
	}

	expenseDate := pgtype.Date{Time: parsed.ExpenseDate, Valid: true}

	var (
		matchedItems   []string
		ambiguousItems []string
		unmatchedItems []string
		totalAmount    decimal.Decimal
	)

	for _, item := range parsed.Items {
		// Match item
		var matchResult matcher.MatchResult
		if h.matcher != nil {
			matchResult = h.matcher.Match(item.Description)
		} else {
			matchResult = matcher.MatchResult{Status: matcher.Unmatched}
		}

		// Calculate unit price from total price
		totalPrice := decimal.NewFromFloat(item.TotalPrice)
		qty := decimal.NewFromFloat(item.Qty)
		if qty.IsZero() {
			qty = decimal.NewFromInt(1)
		}
		unitPrice := totalPrice.Div(qty)

		// Build DB params
		var qtyPg, pricePg, amountPg pgtype.Numeric
		qtyPg.Scan(qty.StringFixed(4))
		pricePg.Scan(unitPrice.StringFixed(2))
		amountPg.Scan(totalPrice.StringFixed(2))

		var itemID pgtype.UUID
		lineType := "EXPENSE"
		accountID := h.defaultAccountID

		switch matchResult.Status {
		case matcher.Matched:
			itemID = uuidToPgUUID(matchResult.Item.ID)
			lineType = "INVENTORY"
			// In a real setup, we'd look up the item's account.
			// For now, use matched item's data.
			matchedItems = append(matchedItems, fmt.Sprintf("%s - %s %s - Rp%s",
				matchResult.Item.Code, matchResult.Item.Name,
				formatQtyUnit(item.Qty, item.Unit), formatRupiah(totalPrice)))

		case matcher.Ambiguous:
			ambiguousItems = append(ambiguousItems, fmt.Sprintf("%s - Rp%s (cocok: %s)",
				item.Description, formatRupiah(totalPrice),
				candidateNames(matchResult.Candidates)))

		case matcher.Unmatched:
			unmatchedItems = append(unmatchedItems, fmt.Sprintf("%s %s - Rp%s",
				item.Description, formatQtyUnit(item.Qty, item.Unit), formatRupiah(totalPrice)))
		}

		description := item.Description
		if item.Unit != "" {
			description = fmt.Sprintf("%s %s", description, formatQtyUnit(item.Qty, item.Unit))
		}

		_, err := h.store.CreateAcctReimbursementRequest(r.Context(), database.CreateAcctReimbursementRequestParams{
			ExpenseDate: expenseDate,
			ItemID:      itemID,
			Description: description,
			Qty:         qtyPg,
			UnitPrice:   pricePg,
			Amount:      amountPg,
			LineType:    lineType,
			AccountID:   accountID,
			Status:      "Draft",
			Requester:   req.SenderName,
			ReceiptLink: pgtype.Text{},
		})
		if err != nil {
			log.Printf("ERROR: create reimbursement from whatsapp: %v", err)
			continue
		}

		totalAmount = totalAmount.Add(totalPrice)
	}

	// Format reply message
	dateStr := parsed.ExpenseDate.Format("2 Jan")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Reimbursement Draft (%s):\n\n", dateStr))

	if len(matchedItems) > 0 {
		sb.WriteString(fmt.Sprintf("Cocok (%d):\n", len(matchedItems)))
		for _, item := range matchedItems {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}

	if len(ambiguousItems) > 0 {
		sb.WriteString(fmt.Sprintf("Ambigu (%d):\n", len(ambiguousItems)))
		for _, item := range ambiguousItems {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}

	if len(unmatchedItems) > 0 {
		sb.WriteString(fmt.Sprintf("Tidak cocok (%d):\n", len(unmatchedItems)))
		for _, item := range unmatchedItems {
			sb.WriteString(fmt.Sprintf("- %s\n", item))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("Total: Rp%s (%d item)\n", formatRupiah(totalAmount), len(parsed.Items)))
	sb.WriteString("Status: Draft\n")
	sb.WriteString(fmt.Sprintf("Requester: %s", req.SenderName))

	writeJSON(w, http.StatusOK, whatsAppResponse{
		ReplyMessage:   sb.String(),
		ItemsCreated:   len(parsed.Items),
		ItemsMatched:   len(matchedItems),
		ItemsAmbiguous: len(ambiguousItems),
		ItemsUnmatched: len(unmatchedItems),
	})
}

// --- Helpers ---

func formatRupiah(d decimal.Decimal) string {
	if d.GreaterThanOrEqual(decimal.NewFromInt(1_000_000)) {
		jt := d.Div(decimal.NewFromInt(1_000_000))
		return jt.StringFixed(1) + "Jt"
	}
	if d.GreaterThanOrEqual(decimal.NewFromInt(1_000)) {
		k := d.Div(decimal.NewFromInt(1_000))
		return k.StringFixed(0) + "K"
	}
	return d.StringFixed(0)
}

func formatQtyUnit(qty float64, unit string) string {
	if unit == "" {
		return ""
	}
	if qty == float64(int(qty)) {
		return fmt.Sprintf("%d%s", int(qty), unit)
	}
	return fmt.Sprintf("%.1f%s", qty, unit)
}

func candidateNames(items []matcher.Item) string {
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	return strings.Join(names, ", ")
}
```

**Step 4: Run tests to verify they pass**

```bash
cd api && go test ./internal/accounting/handler/ -v -run TestFromWhatsApp
```

Expected: all tests pass.

**Step 5: Commit**

```bash
git add api/internal/accounting/handler/whatsapp.go api/internal/accounting/handler/whatsapp_test.go
git commit -m "feat(accounting): add WhatsApp reimbursement endpoint with item matching"
```

---

## Task 6: Wire Routes

**Files:**
- Modify: `api/internal/router/router.go`

**Step 1: Add reimbursement + whatsapp routes**

In `api/internal/router/router.go`, inside the OWNER-only accounting route group (after the purchases handler), add:

```go
// Reimbursements
reimbursementHandler := accthandler.NewReimbursementHandler(queries)
r.Route("/accounting/reimbursements", func(r chi.Router) {
    reimbursementHandler.RegisterRoutes(r)

    // WhatsApp webhook — load items for matcher
    allItems, err := queries.ListAcctItems(context.Background())
    if err != nil {
        log.Printf("WARNING: failed to load items for matcher: %v", err)
        allItems = []database.AcctItem{}
    }
    matcherItems := make([]matcherpkg.Item, len(allItems))
    for i, item := range allItems {
        matcherItems[i] = matcherpkg.Item{
            ID:       item.ID,
            Code:     item.ItemCode,
            Name:     item.ItemName,
            Keywords: item.Keywords,
            Unit:     item.Unit,
        }
    }
    // Default expense account — first active expense account
    var defaultAccountID uuid.UUID
    accts, err := queries.ListAcctAccounts(context.Background())
    if err == nil {
        for _, a := range accts {
            if a.LineType == "EXPENSE" {
                defaultAccountID = a.ID
                break
            }
        }
    }
    whatsappHandler := accthandler.NewWhatsAppHandler(queries, matcherItems, defaultAccountID)
    r.Post("/from-whatsapp", whatsappHandler.FromWhatsApp)
})
```

Add imports at the top:
```go
import (
    "context"
    matcherpkg "github.com/kiwari-pos/api/internal/accounting/matcher"
    // ... existing imports
)
```

**Step 2: Verify compilation**

```bash
cd api && go build ./...
```

Expected: compiles without errors.

**Step 3: Commit**

```bash
git add api/internal/router/router.go
git commit -m "feat(accounting): wire reimbursement and whatsapp routes"
```

---

## Task 7: Admin TypeScript Types

**Files:**
- Modify: `admin/src/lib/types/api.ts`

**Step 1: Add reimbursement types**

Append after the `AcctCashTransaction` interface (line 342):

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

export interface BatchAssignResponse {
	batch_id: string;
	assigned: number;
}

export interface BatchPostResponse {
	batch_id: string;
	posted: number;
	transactions: AcctCashTransaction[];
}

export interface WhatsAppReimbursementResponse {
	reply_message: string;
	items_created: number;
	items_matched: number;
	items_ambiguous: number;
	items_unmatched: number;
}
```

**Step 2: Verify build**

```bash
cd admin && pnpm build
```

Expected: builds without errors.

**Step 3: Commit**

```bash
git add admin/src/lib/types/api.ts
git commit -m "feat(accounting): add reimbursement TypeScript types"
```

---

## Task 8: Sidebar Update

**Files:**
- Modify: `admin/src/lib/components/Sidebar.svelte`

**Step 1: Add Reimburse nav item**

In `Sidebar.svelte`, add to the `keuanganItems` array (line 24-27), inserting after Pembelian:

```typescript
const keuanganItems: NavItem[] = [
    { label: 'Pembelian', href: '/accounting/purchases', icon: '##', roles: ['OWNER'] },
    { label: 'Reimburse', href: '/accounting/reimbursements', icon: '##', roles: ['OWNER'] },
    { label: 'Master Data', href: '/accounting/master', icon: '##', roles: ['OWNER'] }
];
```

**Step 2: Verify build**

```bash
cd admin && pnpm build
```

**Step 3: Commit**

```bash
git add admin/src/lib/components/Sidebar.svelte
git commit -m "feat(accounting): add Reimburse link to sidebar"
```

---

## Task 9: Admin Reimbursement Page

**Files:**
- Create: `admin/src/routes/(app)/accounting/reimbursements/+page.server.ts`
- Create: `admin/src/routes/(app)/accounting/reimbursements/+page.svelte`

This is the largest task. The page supports:
1. **List view** with status/requester filters
2. **Review mode** — edit Draft items (match item, update status to Ready)
3. **Batch assign** — select Draft items → create batch
4. **Batch post** — pick payment_date + cash_account → post

**Step 1: Create the server file**

Create `admin/src/routes/(app)/accounting/reimbursements/+page.server.ts`:

```typescript
import { fail, redirect } from '@sveltejs/kit';
import { apiRequest } from '$lib/server/api';
import type {
	AcctReimbursementRequest,
	AcctItem,
	AcctAccount,
	AcctCashAccount,
	BatchAssignResponse,
	BatchPostResponse
} from '$lib/types/api';
import type { PageServerLoad, Actions } from './$types';

export const load: PageServerLoad = async ({ locals, cookies, url }) => {
	const user = locals.user!;
	if (user.role !== 'OWNER') {
		redirect(302, '/');
	}

	const accessToken = cookies.get('access_token')!;

	// Build query params from URL
	const status = url.searchParams.get('status') || '';
	const requester = url.searchParams.get('requester') || '';
	let queryParams = '?limit=200&offset=0';
	if (status) queryParams += `&status=${encodeURIComponent(status)}`;
	if (requester) queryParams += `&requester=${encodeURIComponent(requester)}`;

	const [reimbursementsResult, itemsResult, accountsResult, cashAccountsResult] =
		await Promise.all([
			apiRequest<AcctReimbursementRequest[]>(
				`/accounting/reimbursements/${queryParams}`,
				{ accessToken }
			),
			apiRequest<AcctItem[]>('/accounting/master/items', { accessToken }),
			apiRequest<AcctAccount[]>('/accounting/master/accounts', { accessToken }),
			apiRequest<AcctCashAccount[]>('/accounting/master/cash-accounts', { accessToken })
		]);

	return {
		reimbursements: reimbursementsResult.ok ? reimbursementsResult.data : [],
		items: itemsResult.ok ? itemsResult.data : [],
		accounts: accountsResult.ok ? accountsResult.data : [],
		cashAccounts: cashAccountsResult.ok ? cashAccountsResult.data : [],
		filterStatus: status,
		filterRequester: requester
	};
};

export const actions: Actions = {
	update: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';
		const dataStr = formData.get('data')?.toString() ?? '';

		let data;
		try {
			data = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest(`/accounting/reimbursements/${id}`, {
			method: 'PUT',
			body: data,
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true };
	},

	delete: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const id = formData.get('id')?.toString() ?? '';

		const result = await apiRequest(`/accounting/reimbursements/${id}`, {
			method: 'DELETE',
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true };
	},

	batchAssign: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const idsStr = formData.get('ids')?.toString() ?? '';

		let ids: string[];
		try {
			ids = JSON.parse(idsStr);
		} catch {
			return fail(400, { error: 'IDs tidak valid' });
		}

		const result = await apiRequest<BatchAssignResponse>('/accounting/reimbursements/batch', {
			method: 'POST',
			body: { ids },
			accessToken
		});

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true, batchId: result.data.batch_id, assigned: result.data.assigned };
	},

	batchPost: async ({ request, cookies, locals }) => {
		const user = locals.user!;
		if (user.role !== 'OWNER') return fail(403, { error: 'Akses ditolak' });

		const accessToken = cookies.get('access_token')!;
		const formData = await request.formData();
		const dataStr = formData.get('data')?.toString() ?? '';

		let data;
		try {
			data = JSON.parse(dataStr);
		} catch {
			return fail(400, { error: 'Data tidak valid' });
		}

		const result = await apiRequest<BatchPostResponse>(
			'/accounting/reimbursements/batch/post',
			{
				method: 'POST',
				body: data,
				accessToken
			}
		);

		if (!result.ok) {
			return fail(result.status || 400, { error: result.message });
		}
		return { success: true, posted: result.data.posted };
	}
};
```

**Step 2: Create the page component**

Create `admin/src/routes/(app)/accounting/reimbursements/+page.svelte`.

This is a large file (~800-1000 lines). Key sections:

1. **Script block:** Props from server, reactive state for filters/selection/modals
2. **Filter bar:** Status dropdown + requester input + apply button
3. **Selection + batch controls:** Checkbox per row, "Buat Batch" button for selected Draft items
4. **Table:** Columns: date, requester, description, qty, price, amount, item match, status, batch, actions
5. **Edit modal:** For Draft/Ready items — item autocomplete, line_type, account, qty, price, status
6. **Batch post dialog:** Pick payment_date + cash_account → post

The page follows the same patterns as the master data page (scoped CSS, `$state()`, `use:enhance`, form actions). Due to the length, the implementer should follow the existing purchase page and master data page patterns. Key guidance:

- Use `$state()` for: `editingId`, `selectedIds`, `showPostDialog`, `postBatchId`, `filterStatus`
- Use `$derived()` for: grouped-by-batch view, filtered list, total selected amount
- Item matching in edit mode: reuse the keyword autocomplete from purchase page
- Batch post dialog: date picker + cash account dropdown + confirm button
- Status badges: Draft (yellow), Ready (blue), Posted (green) — use existing design tokens
- Group rows by `batch_id` when viewing Ready/Posted items
- Each row has: edit (Draft/Ready only), delete (Draft only), checkbox (Draft only for batch assign)

**Step 3: Verify build**

```bash
cd admin && pnpm build
```

Expected: builds without errors.

**Step 4: Commit**

```bash
git add admin/src/routes/\(app\)/accounting/reimbursements/
git commit -m "feat(accounting): add reimbursement admin page with batch workflow"
```

---

## Task 10: Build Verification

**Step 1: Run all Go tests**

```bash
cd api && go test ./... -v
```

Expected: all tests pass (435 existing + ~25 new = ~460 tests).

**Step 2: Run admin build**

```bash
cd admin && pnpm build
```

Expected: builds without errors.

**Step 3: Compile API binary**

```bash
cd api && go build ./cmd/server/
```

Expected: compiles without errors.

**Step 4: Merge and cleanup**

If using a feature branch/worktree, merge to main and clean up.

---

## n8n Workflow Changes (Manual, Outside Codebase)

The n8n workflow shrinks from 13 nodes to 5. This is done manually in the n8n UI:

**New flow:**
```
WAHA Trigger
  → IF: Is Target Group
    → IF: Is Command (starts with reimbursement format)
      → Deduplicate Messages (prevent double-processing)
        → HTTP Request: POST /accounting/reimbursements/from-whatsapp
            body: { sender_phone, sender_name, message_text, chat_id }
          → Send WhatsApp Reply (use reply_message from API response)
```

All parsing, matching, and formatting logic is now in the Go API. n8n just does: filter → forward → reply.

---

## API Endpoint Summary

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/accounting/reimbursements/` | List with filters (status, requester, batch_id, dates) |
| GET | `/accounting/reimbursements/{id}` | Get single |
| POST | `/accounting/reimbursements/` | Create manual entry |
| PUT | `/accounting/reimbursements/{id}` | Update (review, match item, change status) |
| DELETE | `/accounting/reimbursements/{id}` | Delete (Draft only) |
| POST | `/accounting/reimbursements/batch` | Assign batch_id to selected Draft items |
| POST | `/accounting/reimbursements/batch/post` | Post a batch → creates cash_transactions |
| POST | `/accounting/reimbursements/from-whatsapp` | WhatsApp webhook (parse + match + create) |
