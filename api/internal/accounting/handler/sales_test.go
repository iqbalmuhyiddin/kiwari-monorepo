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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock Sales Store ---

type mockSalesStore struct {
	summaries           []database.AcctSalesDailySummary
	aggregateRows       []database.AggregatePOSSalesRow
	unpostedSummaries   []database.AcctSalesDailySummary
	transactions        []database.AcctCashTransaction
	nextTransactionCode string
	createErr           error // allows returning an error on create
	markPostedCalled    bool
}

func newMockSalesStore() *mockSalesStore {
	return &mockSalesStore{
		summaries:           []database.AcctSalesDailySummary{},
		aggregateRows:       []database.AggregatePOSSalesRow{},
		unpostedSummaries:   []database.AcctSalesDailySummary{},
		transactions:        []database.AcctCashTransaction{},
		nextTransactionCode: "PCS000000",
	}
}

func (m *mockSalesStore) ListAcctSalesDailySummaries(ctx context.Context, arg database.ListAcctSalesDailySummariesParams) ([]database.AcctSalesDailySummary, error) {
	return m.summaries, nil
}

func (m *mockSalesStore) GetAcctSalesDailySummary(ctx context.Context, id uuid.UUID) (database.AcctSalesDailySummary, error) {
	for _, s := range m.summaries {
		if s.ID == id {
			return s, nil
		}
	}
	return database.AcctSalesDailySummary{}, pgx.ErrNoRows
}

func (m *mockSalesStore) CreateAcctSalesDailySummary(ctx context.Context, arg database.CreateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error) {
	if m.createErr != nil {
		return database.AcctSalesDailySummary{}, m.createErr
	}
	s := database.AcctSalesDailySummary{
		ID:             uuid.New(),
		SalesDate:      arg.SalesDate,
		Channel:        arg.Channel,
		PaymentMethod:  arg.PaymentMethod,
		GrossSales:     arg.GrossSales,
		DiscountAmount: arg.DiscountAmount,
		NetSales:       arg.NetSales,
		CashAccountID:  arg.CashAccountID,
		OutletID:       arg.OutletID,
		Source:         arg.Source,
		CreatedAt:      time.Now(),
	}
	m.summaries = append(m.summaries, s)
	return s, nil
}

func (m *mockSalesStore) UpdateAcctSalesDailySummary(ctx context.Context, arg database.UpdateAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error) {
	for i, s := range m.summaries {
		if s.ID == arg.ID && s.Source == "manual" && !s.PostedAt.Valid {
			m.summaries[i].Channel = arg.Channel
			m.summaries[i].PaymentMethod = arg.PaymentMethod
			m.summaries[i].GrossSales = arg.GrossSales
			m.summaries[i].DiscountAmount = arg.DiscountAmount
			m.summaries[i].NetSales = arg.NetSales
			m.summaries[i].CashAccountID = arg.CashAccountID
			return m.summaries[i], nil
		}
	}
	return database.AcctSalesDailySummary{}, pgx.ErrNoRows
}

func (m *mockSalesStore) DeleteAcctSalesDailySummary(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockSalesStore) UpsertAcctSalesDailySummary(ctx context.Context, arg database.UpsertAcctSalesDailySummaryParams) (database.AcctSalesDailySummary, error) {
	s := database.AcctSalesDailySummary{
		ID:             uuid.New(),
		SalesDate:      arg.SalesDate,
		Channel:        arg.Channel,
		PaymentMethod:  arg.PaymentMethod,
		GrossSales:     arg.GrossSales,
		DiscountAmount: arg.DiscountAmount,
		NetSales:       arg.NetSales,
		CashAccountID:  arg.CashAccountID,
		OutletID:       arg.OutletID,
		Source:         "pos",
		CreatedAt:      time.Now(),
	}
	return s, nil
}

func (m *mockSalesStore) ListUnpostedSalesSummaries(ctx context.Context, arg database.ListUnpostedSalesSummariesParams) ([]database.AcctSalesDailySummary, error) {
	return m.unpostedSummaries, nil
}

func (m *mockSalesStore) MarkSalesSummariesPosted(ctx context.Context, arg database.MarkSalesSummariesPostedParams) error {
	m.markPostedCalled = true
	return nil
}

func (m *mockSalesStore) AggregatePOSSales(ctx context.Context, arg database.AggregatePOSSalesParams) ([]database.AggregatePOSSalesRow, error) {
	return m.aggregateRows, nil
}

func (m *mockSalesStore) CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error) {
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

func (m *mockSalesStore) GetNextTransactionCode(ctx context.Context) (string, error) {
	return m.nextTransactionCode, nil
}

// --- Helper functions ---

func setupSalesRouter(store handler.SalesStore) *chi.Mux {
	h := handler.NewSalesHandler(store)
	r := chi.NewRouter()
	r.Route("/accounting/sales", h.RegisterRoutes)
	return r
}

func makePgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

// --- Tests ---

func TestListSalesSummaries(t *testing.T) {
	store := newMockSalesStore()
	cashAcctID := uuid.New()
	outletID := uuid.New()

	store.summaries = []database.AcctSalesDailySummary{
		{
			ID:             uuid.New(),
			SalesDate:      makePgDate(2026, 1, 20),
			Channel:        "Dine In",
			PaymentMethod:  "CASH",
			GrossSales:     makePgNumeric("500000.00"),
			DiscountAmount: makePgNumeric("0.00"),
			NetSales:       makePgNumeric("500000.00"),
			CashAccountID:  cashAcctID,
			OutletID:       makePgUUID(outletID),
			Source:         "pos",
			CreatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			SalesDate:      makePgDate(2026, 1, 20),
			Channel:        "Take Away",
			PaymentMethod:  "QRIS",
			GrossSales:     makePgNumeric("250000.00"),
			DiscountAmount: makePgNumeric("10000.00"),
			NetSales:       makePgNumeric("240000.00"),
			CashAccountID:  cashAcctID,
			OutletID:       makePgUUID(outletID),
			Source:         "manual",
			CreatedAt:      time.Now(),
		},
	}

	router := setupSalesRouter(store)

	req := httptest.NewRequest("GET", "/accounting/sales/?start_date=2026-01-20&end_date=2026-01-20", nil)
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
		t.Fatalf("expected 2 summaries, got %d", len(resp))
	}

	if resp[0]["channel"] != "Dine In" {
		t.Errorf("expected channel 'Dine In', got %v", resp[0]["channel"])
	}
	if resp[1]["channel"] != "Take Away" {
		t.Errorf("expected channel 'Take Away', got %v", resp[1]["channel"])
	}
}

func TestCreateSalesSummary(t *testing.T) {
	store := newMockSalesStore()
	router := setupSalesRouter(store)

	cashAccountID := uuid.New()
	outletID := uuid.New().String()

	reqBody := map[string]interface{}{
		"sales_date":      "2026-01-20",
		"channel":         "Grab Food",
		"payment_method":  "TRANSFER",
		"gross_sales":     "1000000.00",
		"discount_amount": "50000.00",
		"net_sales":       "950000.00",
		"cash_account_id": cashAccountID.String(),
		"outlet_id":       outletID,
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["channel"] != "Grab Food" {
		t.Errorf("expected channel 'Grab Food', got %v", resp["channel"])
	}
	if resp["payment_method"] != "TRANSFER" {
		t.Errorf("expected payment_method 'TRANSFER', got %v", resp["payment_method"])
	}
	if resp["source"] != "manual" {
		t.Errorf("expected source 'manual', got %v", resp["source"])
	}
	if resp["gross_sales"] != "1000000.00" {
		t.Errorf("expected gross_sales '1000000.00', got %v", resp["gross_sales"])
	}
	if resp["discount_amount"] != "50000.00" {
		t.Errorf("expected discount_amount '50000.00', got %v", resp["discount_amount"])
	}
	if resp["net_sales"] != "950000.00" {
		t.Errorf("expected net_sales '950000.00', got %v", resp["net_sales"])
	}
}

func TestCreateSalesSummary_MissingFields(t *testing.T) {
	store := newMockSalesStore()
	router := setupSalesRouter(store)

	// Missing channel, payment_method, etc.
	reqBody := map[string]interface{}{
		"sales_date": "2026-01-20",
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateSalesSummary_DuplicateConflict(t *testing.T) {
	store := newMockSalesStore()
	// Set up mock to return unique violation
	store.createErr = &pgconn.PgError{Code: "23505"}
	router := setupSalesRouter(store)

	cashAccountID := uuid.New()

	reqBody := map[string]interface{}{
		"sales_date":      "2026-01-20",
		"channel":         "Dine In",
		"payment_method":  "CASH",
		"gross_sales":     "500000.00",
		"discount_amount": "0.00",
		"net_sales":       "500000.00",
		"cash_account_id": cashAccountID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSalesSummary(t *testing.T) {
	store := newMockSalesStore()
	cashAcctID := uuid.New()
	summaryID := uuid.New()

	// Pre-populate with a manual, unposted summary
	store.summaries = []database.AcctSalesDailySummary{
		{
			ID:             summaryID,
			SalesDate:      makePgDate(2026, 1, 20),
			Channel:        "Grab Food",
			PaymentMethod:  "TRANSFER",
			GrossSales:     makePgNumeric("500000.00"),
			DiscountAmount: makePgNumeric("0.00"),
			NetSales:       makePgNumeric("500000.00"),
			CashAccountID:  cashAcctID,
			Source:         "manual",
			CreatedAt:      time.Now(),
		},
	}

	router := setupSalesRouter(store)

	newCashAcctID := uuid.New()
	reqBody := map[string]interface{}{
		"channel":         "Grab Food",
		"payment_method":  "TRANSFER",
		"gross_sales":     "600000.00",
		"discount_amount": "20000.00",
		"net_sales":       "580000.00",
		"cash_account_id": newCashAcctID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", fmt.Sprintf("/accounting/sales/%s", summaryID.String()), bytes.NewBuffer(body))
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

	if resp["gross_sales"] != "600000.00" {
		t.Errorf("expected gross_sales '600000.00', got %v", resp["gross_sales"])
	}
	if resp["net_sales"] != "580000.00" {
		t.Errorf("expected net_sales '580000.00', got %v", resp["net_sales"])
	}
}

func TestUpdateSalesSummary_PostedOrPOS(t *testing.T) {
	store := newMockSalesStore()
	cashAcctID := uuid.New()
	summaryID := uuid.New()

	// Pre-populate with a POS summary (not manual) — should not be updatable
	store.summaries = []database.AcctSalesDailySummary{
		{
			ID:             summaryID,
			SalesDate:      makePgDate(2026, 1, 20),
			Channel:        "Dine In",
			PaymentMethod:  "CASH",
			GrossSales:     makePgNumeric("500000.00"),
			DiscountAmount: makePgNumeric("0.00"),
			NetSales:       makePgNumeric("500000.00"),
			CashAccountID:  cashAcctID,
			Source:         "pos", // POS source — not editable
			CreatedAt:      time.Now(),
		},
	}

	router := setupSalesRouter(store)

	reqBody := map[string]interface{}{
		"channel":         "Dine In",
		"payment_method":  "CASH",
		"gross_sales":     "600000.00",
		"discount_amount": "0.00",
		"net_sales":       "600000.00",
		"cash_account_id": cashAcctID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("PUT", fmt.Sprintf("/accounting/sales/%s", summaryID.String()), bytes.NewBuffer(body))
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

	if resp["error"] != "summary not found, not manual, or already posted" {
		t.Errorf("expected specific error message, got %v", resp["error"])
	}
}

func TestDeleteSalesSummary(t *testing.T) {
	store := newMockSalesStore()
	router := setupSalesRouter(store)

	summaryID := uuid.New()

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/accounting/sales/%s", summaryID.String()), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSyncPOS(t *testing.T) {
	store := newMockSalesStore()
	outletID := uuid.New()
	cashAcctID := uuid.New()
	qrisCashAcctID := uuid.New()

	// Pre-populate aggregate rows
	store.aggregateRows = []database.AggregatePOSSalesRow{
		{
			SalesDate:     makePgDate(2026, 1, 20),
			OrderType:     "DINE_IN",
			PaymentMethod: "CASH",
			TotalAmount:   "500000.00",
		},
		{
			SalesDate:     makePgDate(2026, 1, 20),
			OrderType:     "TAKEAWAY",
			PaymentMethod: "QRIS",
			TotalAmount:   "250000.00",
		},
	}

	router := setupSalesRouter(store)

	reqBody := map[string]interface{}{
		"start_date": "2026-01-20",
		"end_date":   "2026-01-20",
		"outlet_id":  outletID.String(),
		"payment_method_accounts": map[string]string{
			"CASH": cashAcctID.String(),
			"QRIS": qrisCashAcctID.String(),
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/sync-pos", bytes.NewBuffer(body))
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

	syncedCount := int(resp["synced_count"].(float64))
	if syncedCount != 2 {
		t.Errorf("expected synced_count 2, got %d", syncedCount)
	}

	summaries := resp["summaries"].([]interface{})
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(summaries))
	}

	// Verify channel mapping
	s0 := summaries[0].(map[string]interface{})
	if s0["channel"] != "Dine In" {
		t.Errorf("expected channel 'Dine In', got %v", s0["channel"])
	}
	if s0["source"] != "pos" {
		t.Errorf("expected source 'pos', got %v", s0["source"])
	}

	s1 := summaries[1].(map[string]interface{})
	if s1["channel"] != "Take Away" {
		t.Errorf("expected channel 'Take Away', got %v", s1["channel"])
	}
}

func TestSyncPOS_MissingPaymentMethodMapping(t *testing.T) {
	store := newMockSalesStore()
	outletID := uuid.New()
	cashAcctID := uuid.New()

	// Aggregate has CASH and QRIS, but we only provide mapping for CASH
	store.aggregateRows = []database.AggregatePOSSalesRow{
		{
			SalesDate:     makePgDate(2026, 1, 20),
			OrderType:     "DINE_IN",
			PaymentMethod: "CASH",
			TotalAmount:   "500000.00",
		},
		{
			SalesDate:     makePgDate(2026, 1, 20),
			OrderType:     "TAKEAWAY",
			PaymentMethod: "QRIS",
			TotalAmount:   "250000.00",
		},
	}

	router := setupSalesRouter(store)

	reqBody := map[string]interface{}{
		"start_date": "2026-01-20",
		"end_date":   "2026-01-20",
		"outlet_id":  outletID.String(),
		"payment_method_accounts": map[string]string{
			"CASH": cashAcctID.String(),
			// Missing QRIS mapping
		},
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/sync-pos", bytes.NewBuffer(body))
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

	expectedErr := "no cash account mapping for payment method QRIS"
	if resp["error"] != expectedErr {
		t.Errorf("expected error '%s', got %v", expectedErr, resp["error"])
	}
}

func TestPostSales(t *testing.T) {
	store := newMockSalesStore()
	cashAcctID := uuid.New()
	outletID := uuid.New()
	accountID := uuid.New()

	// Pre-populate unposted summaries
	store.unpostedSummaries = []database.AcctSalesDailySummary{
		{
			ID:             uuid.New(),
			SalesDate:      makePgDate(2026, 1, 20),
			Channel:        "Dine In",
			PaymentMethod:  "CASH",
			GrossSales:     makePgNumeric("500000.00"),
			DiscountAmount: makePgNumeric("0.00"),
			NetSales:       makePgNumeric("500000.00"),
			CashAccountID:  cashAcctID,
			OutletID:       makePgUUID(outletID),
			Source:         "pos",
			CreatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			SalesDate:      makePgDate(2026, 1, 20),
			Channel:        "Take Away",
			PaymentMethod:  "QRIS",
			GrossSales:     makePgNumeric("250000.00"),
			DiscountAmount: makePgNumeric("0.00"),
			NetSales:       makePgNumeric("250000.00"),
			CashAccountID:  cashAcctID,
			OutletID:       makePgUUID(outletID),
			Source:         "pos",
			CreatedAt:      time.Now(),
		},
	}

	router := setupSalesRouter(store)

	outletIDStr := outletID.String()
	reqBody := map[string]interface{}{
		"sales_date": "2026-01-20",
		"outlet_id":  outletIDStr,
		"account_id": accountID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/post", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
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
	if tx0.LineType != "SALES" {
		t.Errorf("expected line_type 'SALES', got %s", tx0.LineType)
	}
	if tx0.Description != "Penjualan Dine In CASH 2026-01-20" {
		t.Errorf("expected description 'Penjualan Dine In CASH 2026-01-20', got %s", tx0.Description)
	}

	// Verify mark posted was called
	if !store.markPostedCalled {
		t.Error("expected MarkSalesSummariesPosted to be called")
	}
}

func TestPostSales_NoneUnposted(t *testing.T) {
	store := newMockSalesStore()
	accountID := uuid.New()

	// No unposted summaries
	store.unpostedSummaries = []database.AcctSalesDailySummary{}

	router := setupSalesRouter(store)

	reqBody := map[string]interface{}{
		"sales_date": "2026-01-20",
		"account_id": accountID.String(),
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/accounting/sales/post", bytes.NewBuffer(body))
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

	if resp["error"] != "no unposted sales summaries found" {
		t.Errorf("expected error 'no unposted sales summaries found', got %v", resp["error"])
	}
}
