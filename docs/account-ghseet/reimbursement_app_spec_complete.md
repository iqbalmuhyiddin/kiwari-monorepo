# Reimbursement & Inventory Pricing App – Complete Specification

## 1. Purpose
Replace fragile Google Sheets formulas and ad-hoc n8n logic with a deterministic application that:
- Parses expense inputs (manual, WhatsApp, or form-based)
- Matches items against MASTER_ITEM reliably
- Calculates prices using clear fallback rules
- Writes clean, auditable records to Google Sheets
- Supports future migration to ERP (Frappe/ERPNext)

---

## 2. Core Problems to Solve
1. Keyword matching ambiguity (e.g. cabe hijau vs cabe merah tanjung)
2. Spreadsheet formulas failing on empty / non-numeric values
3. Pricing logic spread across multiple sheets
4. Hard-to-debug automation (n8n + Sheets)
5. High human error during reimbursement input

---

## 3. Data Model

### 3.1 MASTER_ITEM (Source of Truth)
Fields:
- item_code (PK)
- item_name
- item_category
- unit
- is_inventory (bool)
- is_active (bool)
- item_display
- item_display_unit
- average_price (number | null)
- last_price (number | null)
- for_hpp (number | null)
- keywords (CSV, lowercase, quoted-safe)

Rules:
- keywords MUST be comma-separated, quoted if needed
- color/variant words (merah, hijau, kriting, tanjung) are mandatory keywords

---

### 3.2 REIMBURSEMENT_REQUEST
Fields:
- batch_id
- expense_date
- item_code (nullable)
- item_display
- description (raw user input)
- qty
- unit_price
- amount (qty * unit_price)
- line_type (INVENTORY / EXPENSE)
- account_display
- account_code
- status (Draft | Ready | Posted)
- requester
- receipt_link
- posted_at

---

### 3.3 CASH_TRANSACTION
Ledger-only, append-only.

Fields (simplified):
- transaction_id
- transaction_date
- item_display
- description
- qty
- unit_price
- amount
- line_type
- account_display
- cash_account_display
- reimbursement_batch_id
- month

---

## 4. Item Matching Logic (Critical)

### 4.1 Normalization
For every input line:
- lowercase
- remove punctuation
- normalize units (kg, pcs, iket, batang)
- tokenize words

### 4.2 Matching Algorithm
For each parsed item:
1. Score-based matching against MASTER_ITEM.keywords
2. Score = number of keyword intersections
3. Apply penalty if color mismatch (merah vs hijau)
4. Pick highest score > threshold
5. If tie → mark as ambiguous
6. If none → unmatched

Never use:
- item_name.includes(keyword)
- naive substring matching

---

## 5. Pricing Logic (Single Source)

### 5.1 Price Resolution Order
Given item_code:
1. Use MASTER_ITEM.average_price if numeric > 0
2. Else use MASTER_ITEM.last_price if numeric > 0
3. Else fallback to:
   - Last CASH_TRANSACTION (same item_code, INVENTORY)
4. Else require manual input

### 5.2 Numeric Validation
A value is valid if:
- typeof === number
- !isNaN
- > 0

Empty strings, "", null, text → invalid.

---

## 6. Reimbursement Accounting Logic

### 6.1 Posting Rules
For each batch:

**Leg A – Expense Recognition**
- DR Inventory / Expense account
- CR Reimbursement Payable (2101)
- No cash account involved

**Leg B – Payment**
- DR Reimbursement Payable (2101)
- CR Cash / Bank account (selected by user)

### 6.2 Constraints
- Batch can be posted once (idempotent)
- Append ledger first, update status last
- Validation before any write

---

## 7. Automation Inputs

### 7.1 WhatsApp (WAHA)
Input:
- Multiline text
- Optional date line
- Items: name + qty + total price

Example:
20 jan
cabe merah tanjung 5kg 500k

Parser output:
- expense_date
- raw description
- qty
- total_price

---

## 8. UI Requirements

### 8.1 Reimbursement Draft Review
- Show matched items
- Highlight:
  - unmatched
  - ambiguous
  - price fallback used
- Allow manual override before posting

### 8.2 Posting Dialog
- Date picker
- Cash account selector (cash/bank only)
- Confirmation summary

---

## 9. Non-Goals
- Auto approval
- Auto payment execution
- Inventory stock deduction (future phase)

---

## 10. Migration Readiness
- All IDs stable
- No spreadsheet-only logic
- All calculations reproducible in code
- ERP-compatible chart of accounts

---

## 11. Tech Assumptions
- Backend: Node.js / Bun / Python (TBD)
- Automation: n8n (optional)
- Storage: Google Sheets (temporary), ERPNext later
- Auth: Google OAuth2

---

## 12. Acceptance Criteria
- No ambiguous cabe matching
- No empty prices when data exists
- Deterministic pricing
- Posting never corrupts ledger
- Easy to debug via logs

---

END OF SPEC
