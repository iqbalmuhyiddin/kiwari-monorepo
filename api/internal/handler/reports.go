package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/middleware"
)

// ReportsStore defines the database methods needed by report handlers.
// Satisfied by *database.Queries; narrow interface for testability.
type ReportsStore interface {
	GetDailySales(ctx context.Context, arg database.GetDailySalesParams) ([]database.GetDailySalesRow, error)
	GetProductSales(ctx context.Context, arg database.GetProductSalesParams) ([]database.GetProductSalesRow, error)
	GetPaymentSummary(ctx context.Context, arg database.GetPaymentSummaryParams) ([]database.GetPaymentSummaryRow, error)
	GetHourlySales(ctx context.Context, arg database.GetHourlySalesParams) ([]database.GetHourlySalesRow, error)
	GetOutletComparison(ctx context.Context, arg database.GetOutletComparisonParams) ([]database.GetOutletComparisonRow, error)
}

// ReportsHandler handles report endpoints.
type ReportsHandler struct {
	store ReportsStore
}

// NewReportsHandler creates a new ReportsHandler.
func NewReportsHandler(store ReportsStore) *ReportsHandler {
	return &ReportsHandler{store: store}
}

// RegisterRoutes registers outlet-scoped report endpoints.
// Expected to be mounted inside an outlet-scoped subrouter: /outlets/{oid}/reports
func (h *ReportsHandler) RegisterRoutes(r chi.Router) {
	r.Get("/daily-sales", h.DailySales)
	r.Get("/product-sales", h.ProductSales)
	r.Get("/payment-summary", h.PaymentSummary)
	r.Get("/hourly-sales", h.HourlySales)
}

// RegisterOwnerRoutes registers owner-only report endpoints.
// Expected to be mounted at the root level: /reports
func (h *ReportsHandler) RegisterOwnerRoutes(r chi.Router) {
	r.Get("/outlet-comparison", h.OutletComparison)
}

// --- Response types ---

type dailySalesResponse struct {
	Date          string `json:"date"`
	OrderCount    int64  `json:"order_count"`
	TotalRevenue  string `json:"total_revenue"`
	TotalDiscount string `json:"total_discount"`
	NetRevenue    string `json:"net_revenue"`
}

type productSalesResponse struct {
	ProductID    uuid.UUID `json:"product_id"`
	ProductName  string    `json:"product_name"`
	QuantitySold int64     `json:"quantity_sold"`
	TotalRevenue string    `json:"total_revenue"`
}

type paymentSummaryResponse struct {
	PaymentMethod    string `json:"payment_method"`
	TransactionCount int64  `json:"transaction_count"`
	TotalAmount      string `json:"total_amount"`
}

type hourlySalesResponse struct {
	Hour         int32  `json:"hour"`
	OrderCount   int64  `json:"order_count"`
	TotalRevenue string `json:"total_revenue"`
}

type outletComparisonResponse struct {
	OutletID     uuid.UUID `json:"outlet_id"`
	OutletName   string    `json:"outlet_name"`
	OrderCount   int64     `json:"order_count"`
	TotalRevenue string    `json:"total_revenue"`
}

// --- Handlers ---

// DailySales returns per-day sales totals for a given date range.
func (h *ReportsHandler) DailySales(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	startDate, endDate, err := parseDateRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	rows, err := h.store.GetDailySales(r.Context(), database.GetDailySalesParams{
		OutletID:    outletID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		log.Printf("ERROR: get daily sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]dailySalesResponse, len(rows))
	for i, row := range rows {
		date := "N/A"
		if row.SaleDate.Valid {
			date = row.SaleDate.Time.Format("2006-01-02")
		}
		resp[i] = dailySalesResponse{
			Date:          date,
			OrderCount:    row.OrderCount,
			TotalRevenue:  numericToString(row.TotalRevenue),
			TotalDiscount: numericToString(row.TotalDiscount),
			NetRevenue:    numericToString(row.NetRevenue),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// ProductSales returns top selling products by quantity and revenue.
func (h *ReportsHandler) ProductSales(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	startDate, endDate, err := parseDateRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Parse limit parameter
	limit := 20
	if s := r.URL.Query().Get("limit"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := h.store.GetProductSales(r.Context(), database.GetProductSalesParams{
		OutletID:    outletID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
		Limit:       int32(limit),
	})
	if err != nil {
		log.Printf("ERROR: get product sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]productSalesResponse, len(rows))
	for i, row := range rows {
		resp[i] = productSalesResponse{
			ProductID:    row.ProductID,
			ProductName:  row.ProductName,
			QuantitySold: row.QuantitySold,
			TotalRevenue: numericToString(row.TotalRevenue),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// PaymentSummary returns breakdown of sales by payment method.
func (h *ReportsHandler) PaymentSummary(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	startDate, endDate, err := parseDateRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	rows, err := h.store.GetPaymentSummary(r.Context(), database.GetPaymentSummaryParams{
		OutletID:      outletID,
		ProcessedAt:   startDate,
		ProcessedAt_2: endDate,
	})
	if err != nil {
		log.Printf("ERROR: get payment summary: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]paymentSummaryResponse, len(rows))
	for i, row := range rows {
		resp[i] = paymentSummaryResponse{
			PaymentMethod:    row.PaymentMethod,
			TransactionCount: row.TransactionCount,
			TotalAmount:      numericToString(row.TotalAmount),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// HourlySales returns sales per hour for peak hour analysis.
func (h *ReportsHandler) HourlySales(w http.ResponseWriter, r *http.Request) {
	outletID, err := uuid.Parse(chi.URLParam(r, "oid"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet ID"})
		return
	}

	startDate, endDate, err := parseDateRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	rows, err := h.store.GetHourlySales(r.Context(), database.GetHourlySalesParams{
		OutletID:    outletID,
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		log.Printf("ERROR: get hourly sales: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]hourlySalesResponse, len(rows))
	for i, row := range rows {
		resp[i] = hourlySalesResponse{
			Hour:         row.Hour,
			OrderCount:   row.OrderCount,
			TotalRevenue: numericToString(row.TotalRevenue),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// OutletComparison returns cross-outlet comparison (owner only).
func (h *ReportsHandler) OutletComparison(w http.ResponseWriter, r *http.Request) {
	// Check role
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "OWNER" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "owner access required"})
		return
	}

	startDate, endDate, err := parseDateRange(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	rows, err := h.store.GetOutletComparison(r.Context(), database.GetOutletComparisonParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		log.Printf("ERROR: get outlet comparison: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]outletComparisonResponse, len(rows))
	for i, row := range rows {
		resp[i] = outletComparisonResponse{
			OutletID:     row.OutletID,
			OutletName:   row.OutletName,
			OrderCount:   row.OrderCount,
			TotalRevenue: numericToString(row.TotalRevenue),
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Helpers ---

// parseDateRange parses start_date and end_date query params in Asia/Jakarta timezone.
// Defaults to last 30 days if not provided.
// Returns (startDate, endDate, error) where endDate is exclusive (next day midnight).
func parseDateRange(r *http.Request) (time.Time, time.Time, error) {
	const layout = "2006-01-02"

	// Load Asia/Jakarta timezone (UTC+7) to match database TIMESTAMPTZ
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		// Fallback to FixedZone if LoadLocation fails
		loc = time.FixedZone("WIB", 7*3600)
	}

	// Get current time in Asia/Jakarta
	now := time.Now().In(loc)

	// Default: last 30 days (midnight to midnight in local time)
	startDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -30)
	endDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, 1) // next day midnight

	// Parse start_date if provided
	if s := r.URL.Query().Get("start_date"); s != "" {
		t, err := time.ParseInLocation(layout, s, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date format: %w", err)
		}
		startDate = t
	}

	// Parse end_date if provided
	if s := r.URL.Query().Get("end_date"); s != "" {
		t, err := time.ParseInLocation(layout, s, loc)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date format: %w", err)
		}
		// Make end_date exclusive by adding 1 day
		endDate = t.AddDate(0, 0, 1)
	}

	// Validate: start_date must be <= end_date (before making end exclusive)
	if startDate.After(endDate) || startDate.Equal(endDate) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date must be before end_date")
	}

	return startDate, endDate, nil
}
