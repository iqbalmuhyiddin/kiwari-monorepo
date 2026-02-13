package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// --- Mock Payroll Store ---

type mockPayrollStore struct {
	entries              []database.AcctPayrollEntry
	unpostedEntries      []database.AcctPayrollEntry
	transactions         []database.AcctCashTransaction
	nextTransactionCode  string
	markPostedCalled     bool
}

func newMockPayrollStore() *mockPayrollStore {
	return &mockPayrollStore{
		entries:             []database.AcctPayrollEntry{},
		unpostedEntries:     []database.AcctPayrollEntry{},
		transactions:        []database.AcctCashTransaction{},
		nextTransactionCode: "PCS000000",
	}
}

func (m *mockPayrollStore) ListAcctPayrollEntries(ctx context.Context, arg database.ListAcctPayrollEntriesParams) ([]database.AcctPayrollEntry, error) {
	return m.entries, nil
}

func (m *mockPayrollStore) GetAcctPayrollEntry(ctx context.Context, id uuid.UUID) (database.AcctPayrollEntry, error) {
	for _, e := range m.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return database.AcctPayrollEntry{}, pgx.ErrNoRows
}

func (m *mockPayrollStore) CreateAcctPayrollEntry(ctx context.Context, arg database.CreateAcctPayrollEntryParams) (database.AcctPayrollEntry, error) {
	e := database.AcctPayrollEntry{
		ID:            uuid.New(),
		PayrollDate:   arg.PayrollDate,
		PeriodType:    arg.PeriodType,
		PeriodRef:     arg.PeriodRef,
		EmployeeName:  arg.EmployeeName,
		GrossPay:      arg.GrossPay,
		PaymentMethod: arg.PaymentMethod,
		CashAccountID: arg.CashAccountID,
		OutletID:      arg.OutletID,
		CreatedAt:     time.Now(),
	}
	m.entries = append(m.entries, e)
	return e, nil
}

func (m *mockPayrollStore) UpdateAcctPayrollEntry(ctx context.Context, arg database.UpdateAcctPayrollEntryParams) (database.AcctPayrollEntry, error) {
	for i, e := range m.entries {
		if e.ID == arg.ID && !e.PostedAt.Valid {
			m.entries[i].PayrollDate = arg.PayrollDate
			m.entries[i].PeriodType = arg.PeriodType
			m.entries[i].PeriodRef = arg.PeriodRef
			m.entries[i].EmployeeName = arg.EmployeeName
			m.entries[i].GrossPay = arg.GrossPay
			m.entries[i].PaymentMethod = arg.PaymentMethod
			m.entries[i].CashAccountID = arg.CashAccountID
			m.entries[i].OutletID = arg.OutletID
			return m.entries[i], nil
		}
	}
	return database.AcctPayrollEntry{}, pgx.ErrNoRows
}

func (m *mockPayrollStore) DeleteAcctPayrollEntry(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockPayrollStore) ListUnpostedPayrollEntries(ctx context.Context, ids []uuid.UUID) ([]database.AcctPayrollEntry, error) {
	return m.unpostedEntries, nil
}

func (m *mockPayrollStore) MarkPayrollEntriesPosted(ctx context.Context, ids []uuid.UUID) error {
	m.markPostedCalled = true
	return nil
}

func (m *mockPayrollStore) CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
	tx := database.AcctCashTransaction{
		ID:                   uuid.New(),
		TransactionCode:      arg.TransactionCode,
		TransactionDate:      arg.TransactionDate,
		ItemID:               arg.ItemID,
		Description:          arg.Description,
		Quantity:             arg.Quantity,
		UnitPrice:            arg.UnitPrice,
		Amount:               arg.Amount,
		LineType:             arg.LineType,
		AccountID:            arg.AccountID,
		CashAccountID:        arg.CashAccountID,
		OutletID:             arg.OutletID,
		ReimbursementBatchID: arg.ReimbursementBatchID,
		CreatedAt:            time.Now(),
	}
	m.transactions = append(m.transactions, tx)
	return tx, nil
}

func (m *mockPayrollStore) GetNextTransactionCode(ctx context.Context) (string, error) {
	return m.nextTransactionCode, nil
}

// --- Helper functions ---

func setupPayrollRouter(store handler.PayrollStore) *chi.Mux {
	h := handler.NewPayrollHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/payroll", h.RegisterRoutes)
	return r
}

// --- Tests ---

func TestListPayrollEntries(t *testing.T) {
	store := newMockPayrollStore()
	cashAcctID := uuid.New()
	outletID := uuid.New()

	store.entries = []database.AcctPayrollEntry{
		{
			ID:            uuid.New(),
			PayrollDate:   makePgDate(2026, 1, 20),
			PeriodType:    "Monthly",
			PeriodRef:     pgtype.Text{String: "Jan 2026", Valid: true},
			EmployeeName:  "John Doe",
			GrossPay:      makePgNumeric("5000000.00"),
			PaymentMethod: "TRANSFER",
			CashAccountID: cashAcctID,
			OutletID:      makePgUUID(outletID),
			CreatedAt:     time.Now(),
		},
		{
			ID:            uuid.New(),
			PayrollDate:   makePgDate(2026, 1, 21),
			PeriodType:    "Daily",
			EmployeeName:  "Jane Smith",
			GrossPay:      makePgNumeric("200000.00"),
			PaymentMethod: "CASH",
			CashAccountID: cashAcctID,
			OutletID:      makePgUUID(outletID),
			CreatedAt:     time.Now(),
		},
	}

	router := setupPayrollRouter(store)

	req := httptest.NewRequest("GET", "/accounting/payroll/?start_date=2026-01-20&end_date=2026-01-21", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp))
	}

	if resp[0]["employee_name"] != "John Doe" {
		t.Errorf("expected employee_name 'John Doe', got %v", resp[0]["employee_name"])
	}
	if resp[0]["period_type"] != "Monthly" {
		t.Errorf("expected period_type 'Monthly', got %v", resp[0]["period_type"])
	}
	if resp[1]["employee_name"] != "Jane Smith" {
		t.Errorf("expected employee_name 'Jane Smith', got %v", resp[1]["employee_name"])
	}
}

func TestCreatePayrollBatch(t *testing.T) {
	store := newMockPayrollStore()
	router := setupPayrollRouter(store)

	cashAccountID := uuid.New()
	outletID := uuid.New().String()

	reqBody := map[string]interface{}{
		"payroll_date":    "2026-01-20",
		"period_type":     "Monthly",
		"period_ref":      "Jan 2026",
		"cash_account_id": cashAccountID.String(),
		"outlet_id":       outletID,
		"employees": []map[string]interface{}{
			{
				"employee_name":  "John Doe",
				"gross_pay":      "5000000.00",
				"payment_method": "TRANSFER",
			},
			{
				"employee_name":  "Jane Smith",
				"gross_pay":      "4500000.00",
				"payment_method": "TRANSFER",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/payroll/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(resp) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp))
	}

	if resp[0]["employee_name"] != "John Doe" {
		t.Errorf("expected employee_name 'John Doe', got %v", resp[0]["employee_name"])
	}
	if resp[0]["gross_pay"] != "5000000.00" {
		t.Errorf("expected gross_pay '5000000.00', got %v", resp[0]["gross_pay"])
	}
	if resp[1]["employee_name"] != "Jane Smith" {
		t.Errorf("expected employee_name 'Jane Smith', got %v", resp[1]["employee_name"])
	}
	if resp[1]["gross_pay"] != "4500000.00" {
		t.Errorf("expected gross_pay '4500000.00', got %v", resp[1]["gross_pay"])
	}

	// Verify entries were created in store
	if len(store.entries) != 2 {
		t.Errorf("expected 2 entries in store, got %d", len(store.entries))
	}
}

func TestCreatePayrollBatch_MissingFields(t *testing.T) {
	store := newMockPayrollStore()
	router := setupPayrollRouter(store)

	// Missing employees array
	reqBody := map[string]interface{}{
		"payroll_date":    "2026-01-20",
		"period_type":     "Monthly",
		"cash_account_id": uuid.New().String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/payroll/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreatePayrollBatch_InvalidPeriodType(t *testing.T) {
	store := newMockPayrollStore()
	router := setupPayrollRouter(store)

	cashAccountID := uuid.New()

	reqBody := map[string]interface{}{
		"payroll_date":    "2026-01-20",
		"period_type":     "Quarterly", // Invalid
		"cash_account_id": cashAccountID.String(),
		"employees": []map[string]interface{}{
			{
				"employee_name":  "John Doe",
				"gross_pay":      "5000000.00",
				"payment_method": "TRANSFER",
			},
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/payroll/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "period_type must be Daily, Weekly, or Monthly" {
		t.Errorf("expected period_type error, got %v", resp["error"])
	}
}

func TestUpdatePayrollEntry(t *testing.T) {
	store := newMockPayrollStore()
	cashAcctID := uuid.New()
	entryID := uuid.New()

	// Pre-populate with an unposted entry
	store.entries = []database.AcctPayrollEntry{
		{
			ID:            entryID,
			PayrollDate:   makePgDate(2026, 1, 20),
			PeriodType:    "Monthly",
			EmployeeName:  "John Doe",
			GrossPay:      makePgNumeric("5000000.00"),
			PaymentMethod: "TRANSFER",
			CashAccountID: cashAcctID,
			CreatedAt:     time.Now(),
		},
	}

	router := setupPayrollRouter(store)

	newCashAcctID := uuid.New()
	reqBody := map[string]interface{}{
		"payroll_date":    "2026-01-21",
		"period_type":     "Monthly",
		"period_ref":      "Jan 2026",
		"employee_name":   "John Doe",
		"gross_pay":       "5500000.00",
		"payment_method":  "TRANSFER",
		"cash_account_id": newCashAcctID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", fmt.Sprintf("/accounting/payroll/%s", entryID.String()), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["gross_pay"] != "5500000.00" {
		t.Errorf("expected gross_pay '5500000.00', got %v", resp["gross_pay"])
	}
	if resp["payroll_date"] != "2026-01-21" {
		t.Errorf("expected payroll_date '2026-01-21', got %v", resp["payroll_date"])
	}
}

func TestUpdatePayrollEntry_AlreadyPosted(t *testing.T) {
	store := newMockPayrollStore()
	cashAcctID := uuid.New()
	entryID := uuid.New()

	// Pre-populate with a posted entry
	postedAt := time.Now()
	store.entries = []database.AcctPayrollEntry{
		{
			ID:            entryID,
			PayrollDate:   makePgDate(2026, 1, 20),
			PeriodType:    "Monthly",
			EmployeeName:  "John Doe",
			GrossPay:      makePgNumeric("5000000.00"),
			PaymentMethod: "TRANSFER",
			CashAccountID: cashAcctID,
			PostedAt:      pgtype.Timestamptz{Time: postedAt, Valid: true},
			CreatedAt:     time.Now(),
		},
	}

	router := setupPayrollRouter(store)

	reqBody := map[string]interface{}{
		"payroll_date":    "2026-01-21",
		"period_type":     "Monthly",
		"employee_name":   "John Doe",
		"gross_pay":       "5500000.00",
		"payment_method":  "TRANSFER",
		"cash_account_id": cashAcctID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", fmt.Sprintf("/accounting/payroll/%s", entryID.String()), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "entry not found or already posted" {
		t.Errorf("expected 'entry not found or already posted' error, got %v", resp["error"])
	}
}

func TestDeletePayrollEntry(t *testing.T) {
	store := newMockPayrollStore()
	router := setupPayrollRouter(store)

	entryID := uuid.New()

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/accounting/payroll/%s", entryID.String()), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPostPayroll(t *testing.T) {
	store := newMockPayrollStore()
	cashAcctID := uuid.New()
	outletID := uuid.New()
	accountID := uuid.New()
	entry1ID := uuid.New()
	entry2ID := uuid.New()

	// Pre-populate unposted entries
	store.unpostedEntries = []database.AcctPayrollEntry{
		{
			ID:            entry1ID,
			PayrollDate:   makePgDate(2026, 1, 20),
			PeriodType:    "Monthly",
			PeriodRef:     pgtype.Text{String: "Jan 2026", Valid: true},
			EmployeeName:  "John Doe",
			GrossPay:      makePgNumeric("5000000.00"),
			PaymentMethod: "TRANSFER",
			CashAccountID: cashAcctID,
			OutletID:      makePgUUID(outletID),
			CreatedAt:     time.Now(),
		},
		{
			ID:            entry2ID,
			PayrollDate:   makePgDate(2026, 1, 20),
			PeriodType:    "Monthly",
			PeriodRef:     pgtype.Text{String: "Jan 2026", Valid: true},
			EmployeeName:  "Jane Smith",
			GrossPay:      makePgNumeric("4500000.00"),
			PaymentMethod: "TRANSFER",
			CashAccountID: cashAcctID,
			OutletID:      makePgUUID(outletID),
			CreatedAt:     time.Now(),
		},
	}

	router := setupPayrollRouter(store)

	reqBody := map[string]interface{}{
		"ids": []string{
			entry1ID.String(),
			entry2ID.String(),
		},
		"account_id": accountID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/payroll/post", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	postedCount := int(resp["posted_count"].(float64))
	if postedCount != 2 {
		t.Errorf("expected posted_count 2, got %d", postedCount)
	}

	txCreated := int(resp["transactions_created"].(float64))
	if txCreated != 2 {
		t.Errorf("expected transactions_created 2, got %d", txCreated)
	}

	// Verify transactions were created
	if len(store.transactions) != 2 {
		t.Fatalf("expected 2 transactions created, got %d", len(store.transactions))
	}

	// Verify first transaction details
	tx0 := store.transactions[0]
	if tx0.TransactionCode != "PCS000001" {
		t.Errorf("expected transaction code 'PCS000001', got %s", tx0.TransactionCode)
	}
	if tx0.LineType != "EXPENSE" {
		t.Errorf("expected line_type 'EXPENSE', got %s", tx0.LineType)
	}
	if tx0.Description != "Gaji John Doe Jan 2026" {
		t.Errorf("expected description 'Gaji John Doe Jan 2026', got %s", tx0.Description)
	}

	// Verify second transaction
	tx1 := store.transactions[1]
	if tx1.TransactionCode != "PCS000002" {
		t.Errorf("expected transaction code 'PCS000002', got %s", tx1.TransactionCode)
	}
	if tx1.Description != "Gaji Jane Smith Jan 2026" {
		t.Errorf("expected description 'Gaji Jane Smith Jan 2026', got %s", tx1.Description)
	}

	// Verify mark posted was called
	if !store.markPostedCalled {
		t.Error("expected MarkPayrollEntriesPosted to be called")
	}
}

func TestPostPayroll_NoneUnposted(t *testing.T) {
	store := newMockPayrollStore()
	accountID := uuid.New()

	// No unposted entries
	store.unpostedEntries = []database.AcctPayrollEntry{}

	router := setupPayrollRouter(store)

	reqBody := map[string]interface{}{
		"ids": []string{
			uuid.New().String(),
		},
		"account_id": accountID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/payroll/post", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["error"] != "no unposted payroll entries found" {
		t.Errorf("expected error 'no unposted payroll entries found', got %v", resp["error"])
	}
}
