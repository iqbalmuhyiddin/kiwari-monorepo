package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/auth"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/enum"
	"github.com/kiwari-pos/api/internal/handler"
	"github.com/kiwari-pos/api/internal/middleware"
	"github.com/shopspring/decimal"
)

// testJWTSecret is defined in orders_test.go

// --- Mock Store ---

type mockReportsStore struct {
	dailySales        []database.GetDailySalesRow
	productSales      []database.GetProductSalesRow
	paymentSummary    []database.GetPaymentSummaryRow
	hourlySales       []database.GetHourlySalesRow
	outletComparison  []database.GetOutletComparisonRow
	dailySalesErr     error
	productSalesErr   error
	paymentSummaryErr error
	hourlySalesErr    error
	outletCompErr     error
}

func (m *mockReportsStore) GetDailySales(ctx context.Context, arg database.GetDailySalesParams) ([]database.GetDailySalesRow, error) {
	if m.dailySalesErr != nil {
		return nil, m.dailySalesErr
	}
	return m.dailySales, nil
}

func (m *mockReportsStore) GetProductSales(ctx context.Context, arg database.GetProductSalesParams) ([]database.GetProductSalesRow, error) {
	if m.productSalesErr != nil {
		return nil, m.productSalesErr
	}
	return m.productSales, nil
}

func (m *mockReportsStore) GetPaymentSummary(ctx context.Context, arg database.GetPaymentSummaryParams) ([]database.GetPaymentSummaryRow, error) {
	if m.paymentSummaryErr != nil {
		return nil, m.paymentSummaryErr
	}
	return m.paymentSummary, nil
}

func (m *mockReportsStore) GetHourlySales(ctx context.Context, arg database.GetHourlySalesParams) ([]database.GetHourlySalesRow, error) {
	if m.hourlySalesErr != nil {
		return nil, m.hourlySalesErr
	}
	return m.hourlySales, nil
}

func (m *mockReportsStore) GetOutletComparison(ctx context.Context, arg database.GetOutletComparisonParams) ([]database.GetOutletComparisonRow, error) {
	if m.outletCompErr != nil {
		return nil, m.outletCompErr
	}
	return m.outletComparison, nil
}

// --- Test Helpers ---

func toNumeric(s string) pgtype.Numeric {
	d, _ := decimal.NewFromString(s)
	n := pgtype.Numeric{}
	n.Scan(d.String())
	return n
}

func toDate(s string) pgtype.Date {
	t, _ := time.Parse("2006-01-02", s)
	var date pgtype.Date
	date.Scan(t)
	return date
}

func setupReportsRouter(store handler.ReportsStore) http.Handler {
	h := handler.NewReportsHandler(store)
	r := chi.NewRouter()
	r.Route("/outlets/{oid}/reports", h.RegisterRoutes)
	return r
}

func setupOwnerReportsRouter(store handler.ReportsStore) http.Handler {
	h := handler.NewReportsHandler(store)
	r := chi.NewRouter()
	r.Use(middleware.Authenticate(testJWTSecret))
	r.Route("/reports", func(r chi.Router) {
		h.RegisterOwnerRoutes(r)
	})
	return r
}

// --- Daily Sales Tests ---

func TestDailySales(t *testing.T) {
	outletID := uuid.New()
	store := &mockReportsStore{
		dailySales: []database.GetDailySalesRow{
			{
				SaleDate:      toDate("2026-02-01"),
				OrderCount:    10,
				TotalRevenue:  toNumeric("500000.00"),
				TotalDiscount: toNumeric("50000.00"),
				NetRevenue:    toNumeric("450000.00"),
			},
			{
				SaleDate:      toDate("2026-02-02"),
				OrderCount:    15,
				TotalRevenue:  toNumeric("750000.00"),
				TotalDiscount: toNumeric("75000.00"),
				NetRevenue:    toNumeric("675000.00"),
			},
		},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/daily-sales?start_date=2026-02-01&end_date=2026-02-02", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp))
	}

	if resp[0]["date"] != "2026-02-01" {
		t.Errorf("expected date 2026-02-01, got %v", resp[0]["date"])
	}
	if resp[0]["order_count"] != float64(10) {
		t.Errorf("expected order_count 10, got %v", resp[0]["order_count"])
	}
	if resp[0]["total_revenue"] != "500000.00" {
		t.Errorf("expected total_revenue 500000.00, got %v", resp[0]["total_revenue"])
	}
}

func TestDailySales_DefaultDateRange(t *testing.T) {
	outletID := uuid.New()
	store := &mockReportsStore{
		dailySales: []database.GetDailySalesRow{},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/daily-sales", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestDailySales_InvalidDate(t *testing.T) {
	outletID := uuid.New()
	store := &mockReportsStore{}
	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/daily-sales?start_date=invalid", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

func TestDailySales_EmptyResult(t *testing.T) {
	outletID := uuid.New()
	store := &mockReportsStore{
		dailySales: []database.GetDailySalesRow{},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/daily-sales?start_date=2026-02-01&end_date=2026-02-02", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 0 {
		t.Errorf("expected empty array, got %d items", len(resp))
	}
}

func TestDailySales_StartAfterEnd(t *testing.T) {
	outletID := uuid.New()
	store := &mockReportsStore{}
	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/daily-sales?start_date=2026-02-10&end_date=2026-02-01", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rr.Code)
	}
}

// --- Product Sales Tests ---

func TestProductSales(t *testing.T) {
	outletID := uuid.New()
	productID1 := uuid.New()
	productID2 := uuid.New()

	store := &mockReportsStore{
		productSales: []database.GetProductSalesRow{
			{
				ProductID:    productID1,
				ProductName:  "Nasi Bakar Ayam",
				QuantitySold: 50,
				TotalRevenue: toNumeric("1250000.00"),
			},
			{
				ProductID:    productID2,
				ProductName:  "Es Teh Manis",
				QuantitySold: 30,
				TotalRevenue: toNumeric("150000.00"),
			},
		},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/product-sales?start_date=2026-02-01&end_date=2026-02-02&limit=10", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp))
	}

	if resp[0]["product_name"] != "Nasi Bakar Ayam" {
		t.Errorf("expected product name 'Nasi Bakar Ayam', got %v", resp[0]["product_name"])
	}
	if resp[0]["quantity_sold"] != float64(50) {
		t.Errorf("expected quantity_sold 50, got %v", resp[0]["quantity_sold"])
	}
}

func TestProductSales_DefaultLimit(t *testing.T) {
	outletID := uuid.New()
	store := &mockReportsStore{
		productSales: []database.GetProductSalesRow{},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/product-sales", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

// --- Payment Summary Tests ---

func TestPaymentSummary(t *testing.T) {
	outletID := uuid.New()

	store := &mockReportsStore{
		paymentSummary: []database.GetPaymentSummaryRow{
			{
				PaymentMethod:    enum.PaymentMethodCash,
				TransactionCount: 20,
				TotalAmount:      toNumeric("800000.00"),
			},
			{
				PaymentMethod:    enum.PaymentMethodQRIS,
				TransactionCount: 15,
				TotalAmount:      toNumeric("600000.00"),
			},
		},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/payment-summary?start_date=2026-02-01&end_date=2026-02-02", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp))
	}

	if resp[0]["payment_method"] != "CASH" {
		t.Errorf("expected payment_method 'CASH', got %v", resp[0]["payment_method"])
	}
	if resp[0]["transaction_count"] != float64(20) {
		t.Errorf("expected transaction_count 20, got %v", resp[0]["transaction_count"])
	}
}

// --- Hourly Sales Tests ---

func TestHourlySales(t *testing.T) {
	outletID := uuid.New()

	store := &mockReportsStore{
		hourlySales: []database.GetHourlySalesRow{
			{
				Hour:         12,
				OrderCount:   15,
				TotalRevenue: toNumeric("750000.00"),
			},
			{
				Hour:         18,
				OrderCount:   20,
				TotalRevenue: toNumeric("1000000.00"),
			},
		},
	}

	router := setupReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/outlets/%s/reports/hourly-sales?start_date=2026-02-01&end_date=2026-02-02", outletID), nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp))
	}

	if resp[0]["hour"] != float64(12) {
		t.Errorf("expected hour 12, got %v", resp[0]["hour"])
	}
	if resp[0]["order_count"] != float64(15) {
		t.Errorf("expected order_count 15, got %v", resp[0]["order_count"])
	}
}

// --- Outlet Comparison Tests ---

func TestOutletComparison_Success(t *testing.T) {
	outlet1 := uuid.New()
	outlet2 := uuid.New()

	store := &mockReportsStore{
		outletComparison: []database.GetOutletComparisonRow{
			{
				OutletID:     outlet1,
				OutletName:   "Cabang 1",
				OrderCount:   100,
				TotalRevenue: toNumeric("5000000.00"),
			},
			{
				OutletID:     outlet2,
				OutletName:   "Cabang 2",
				OrderCount:   80,
				TotalRevenue: toNumeric("4000000.00"),
			},
		},
	}

	router := setupOwnerReportsRouter(store)

	// Create owner claims and generate token
	claims := &auth.Claims{
		UserID:   uuid.New(),
		OutletID: uuid.New(),
		Role:     "OWNER",
	}
	token, err := auth.GenerateToken(testJWTSecret, claims.UserID, claims.OutletID, claims.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/reports/outlet-comparison?start_date=2026-02-01&end_date=2026-02-02", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	var resp []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp) != 2 {
		t.Errorf("expected 2 rows, got %d", len(resp))
	}

	if resp[0]["outlet_name"] != "Cabang 1" {
		t.Errorf("expected outlet_name 'Cabang 1', got %v", resp[0]["outlet_name"])
	}
	if resp[0]["total_revenue"] != "5000000.00" {
		t.Errorf("expected total_revenue 5000000.00, got %v", resp[0]["total_revenue"])
	}
}

func TestOutletComparison_Forbidden(t *testing.T) {
	store := &mockReportsStore{}
	router := setupOwnerReportsRouter(store)

	// Create non-owner claims and generate token
	claims := &auth.Claims{
		UserID:   uuid.New(),
		OutletID: uuid.New(),
		Role:     "MANAGER",
	}
	token, err := auth.GenerateToken(testJWTSecret, claims.UserID, claims.OutletID, claims.Role)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/reports/outlet-comparison?start_date=2026-02-01&end_date=2026-02-02", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", rr.Code)
	}
}

func TestOutletComparison_NoClaims(t *testing.T) {
	store := &mockReportsStore{}
	router := setupOwnerReportsRouter(store)

	req := httptest.NewRequest(http.MethodGet, "/reports/outlet-comparison", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rr.Code)
	}
}
