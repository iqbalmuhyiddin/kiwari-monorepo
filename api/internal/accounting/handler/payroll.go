package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

type PayrollStore interface {
	ListAcctPayrollEntries(ctx context.Context, arg database.ListAcctPayrollEntriesParams) ([]database.AcctPayrollEntry, error)
	GetAcctPayrollEntry(ctx context.Context, id uuid.UUID) (database.AcctPayrollEntry, error)
	CreateAcctPayrollEntry(ctx context.Context, arg database.CreateAcctPayrollEntryParams) (database.AcctPayrollEntry, error)
	UpdateAcctPayrollEntry(ctx context.Context, arg database.UpdateAcctPayrollEntryParams) (database.AcctPayrollEntry, error)
	DeleteAcctPayrollEntry(ctx context.Context, id uuid.UUID) error
	ListUnpostedPayrollEntries(ctx context.Context, ids []uuid.UUID) ([]database.AcctPayrollEntry, error)
	MarkPayrollEntriesPosted(ctx context.Context, ids []uuid.UUID) error
	// For posting:
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
}

// --- PayrollHandler ---

type PayrollHandler struct {
	store PayrollStore
}

func NewPayrollHandler(store PayrollStore) *PayrollHandler {
	return &PayrollHandler{store: store}
}

func (h *PayrollHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListPayrollEntries)
	r.Post("/", h.CreatePayrollBatch)
	r.Put("/{id}", h.UpdatePayrollEntry)
	r.Delete("/{id}", h.DeletePayrollEntry)
	r.Post("/post", h.PostPayroll)
}

// --- Request / Response types ---

var validPeriodTypes = map[string]bool{
	"Daily":   true,
	"Weekly":  true,
	"Monthly": true,
}

type payrollEntryResponse struct {
	ID            uuid.UUID  `json:"id"`
	PayrollDate   string     `json:"payroll_date"`
	PeriodType    string     `json:"period_type"`
	PeriodRef     *string    `json:"period_ref"`
	EmployeeName  string     `json:"employee_name"`
	GrossPay      string     `json:"gross_pay"`
	PaymentMethod string     `json:"payment_method"`
	CashAccountID string     `json:"cash_account_id"`
	OutletID      *string    `json:"outlet_id"`
	PostedAt      *time.Time `json:"posted_at"`
	CreatedAt     time.Time  `json:"created_at"`
}

type createPayrollBatchRequest struct {
	PayrollDate   string                  `json:"payroll_date"`
	PeriodType    string                  `json:"period_type"`
	PeriodRef     *string                 `json:"period_ref"`
	CashAccountID string                  `json:"cash_account_id"`
	OutletID      *string                 `json:"outlet_id"`
	Employees     []payrollEmployeeEntry  `json:"employees"`
}

type payrollEmployeeEntry struct {
	EmployeeName  string `json:"employee_name"`
	GrossPay      string `json:"gross_pay"`
	PaymentMethod string `json:"payment_method"`
}

type updatePayrollEntryRequest struct {
	PayrollDate   string  `json:"payroll_date"`
	PeriodType    string  `json:"period_type"`
	PeriodRef     *string `json:"period_ref"`
	EmployeeName  string  `json:"employee_name"`
	GrossPay      string  `json:"gross_pay"`
	PaymentMethod string  `json:"payment_method"`
	CashAccountID string  `json:"cash_account_id"`
	OutletID      *string `json:"outlet_id"`
}

type postPayrollRequest struct {
	IDs       []string `json:"ids"`
	AccountID string   `json:"account_id"` // Payroll Expense account UUID
}

type postPayrollResponse struct {
	PostedCount         int `json:"posted_count"`
	TransactionsCreated int `json:"transactions_created"`
}

// --- Handlers ---

func (h *PayrollHandler) ListPayrollEntries(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)

	params := database.ListAcctPayrollEntriesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	if v := r.URL.Query().Get("start_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date"})
			return
		}
		params.StartDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("end_date"); v != "" {
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date"})
			return
		}
		params.EndDate = pgtype.Date{Time: d, Valid: true}
	}
	if v := r.URL.Query().Get("outlet_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		params.OutletID = uuidToPgUUID(id)
	}
	if v := r.URL.Query().Get("period_type"); v != "" {
		params.PeriodType = pgtype.Text{String: v, Valid: true}
	}

	rows, err := h.store.ListAcctPayrollEntries(r.Context(), params)
	if err != nil {
		log.Printf("ERROR: list payroll entries: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	result := make([]payrollEntryResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, toPayrollEntryResponse(row))
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *PayrollHandler) CreatePayrollBatch(w http.ResponseWriter, r *http.Request) {
	var req createPayrollBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.PayrollDate == "" || req.PeriodType == "" || req.CashAccountID == "" || len(req.Employees) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "payroll_date, period_type, cash_account_id, and employees are required"})
		return
	}

	if !validPeriodTypes[req.PeriodType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_type must be Daily, Weekly, or Monthly"})
		return
	}

	date, err := time.Parse("2006-01-02", req.PayrollDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payroll_date format, expected YYYY-MM-DD"})
		return
	}

	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	var periodRef pgtype.Text
	if req.PeriodRef != nil && *req.PeriodRef != "" {
		periodRef = pgtype.Text{String: *req.PeriodRef, Valid: true}
	}

	entries := make([]payrollEntryResponse, 0, len(req.Employees))
	for _, emp := range req.Employees {
		if emp.EmployeeName == "" || emp.GrossPay == "" || emp.PaymentMethod == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "each employee needs employee_name, gross_pay, and payment_method"})
			return
		}

		pay, err := decimal.NewFromString(emp.GrossPay)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid gross_pay for %s", emp.EmployeeName)})
			return
		}

		var payPg pgtype.Numeric
		if err := payPg.Scan(pay.StringFixed(2)); err != nil {
			log.Printf("ERROR: scan gross_pay: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		row, err := h.store.CreateAcctPayrollEntry(r.Context(), database.CreateAcctPayrollEntryParams{
			PayrollDate:   pgtype.Date{Time: date, Valid: true},
			PeriodType:    req.PeriodType,
			PeriodRef:     periodRef,
			EmployeeName:  emp.EmployeeName,
			GrossPay:      payPg,
			PaymentMethod: emp.PaymentMethod,
			CashAccountID: cashAcctID,
			OutletID:      outletID,
		})
		if err != nil {
			log.Printf("ERROR: create payroll entry: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		entries = append(entries, toPayrollEntryResponse(row))
	}

	writeJSON(w, http.StatusCreated, entries)
}

func (h *PayrollHandler) UpdatePayrollEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req updatePayrollEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.PayrollDate == "" || req.PeriodType == "" || req.EmployeeName == "" ||
		req.GrossPay == "" || req.PaymentMethod == "" || req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "all fields are required"})
		return
	}

	if !validPeriodTypes[req.PeriodType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_type must be Daily, Weekly, or Monthly"})
		return
	}

	date, err := time.Parse("2006-01-02", req.PayrollDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payroll_date"})
		return
	}

	cashAcctID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	pay, err := decimal.NewFromString(req.GrossPay)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid gross_pay"})
		return
	}

	var payPg pgtype.Numeric
	if err := payPg.Scan(pay.StringFixed(2)); err != nil {
		log.Printf("ERROR: scan gross_pay: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		oid, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(oid)
	}

	var periodRef pgtype.Text
	if req.PeriodRef != nil && *req.PeriodRef != "" {
		periodRef = pgtype.Text{String: *req.PeriodRef, Valid: true}
	}

	row, err := h.store.UpdateAcctPayrollEntry(r.Context(), database.UpdateAcctPayrollEntryParams{
		ID:            id,
		PayrollDate:   pgtype.Date{Time: date, Valid: true},
		PeriodType:    req.PeriodType,
		PeriodRef:     periodRef,
		EmployeeName:  req.EmployeeName,
		GrossPay:      payPg,
		PaymentMethod: req.PaymentMethod,
		CashAccountID: cashAcctID,
		OutletID:      outletID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "entry not found or already posted"})
			return
		}
		log.Printf("ERROR: update payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toPayrollEntryResponse(row))
}

func (h *PayrollHandler) DeletePayrollEntry(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	err = h.store.DeleteAcctPayrollEntry(r.Context(), id)
	if err != nil {
		log.Printf("ERROR: delete payroll entry: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PayrollHandler) PostPayroll(w http.ResponseWriter, r *http.Request) {
	var req postPayrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.IDs) == 0 || req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids and account_id are required"})
		return
	}

	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse UUIDs
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid id: %s", idStr)})
			return
		}
		ids = append(ids, id)
	}

	// Get unposted entries
	entries, err := h.store.ListUnpostedPayrollEntries(r.Context(), ids)
	if err != nil {
		log.Printf("ERROR: list unposted payroll: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if len(entries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no unposted payroll entries found"})
		return
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum, err := parseTransactionCodeNum(maxCode)
	if err != nil {
		log.Printf("ERROR: parse transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Create cash_transactions for each entry
	txCount := 0
	for _, e := range entries {
		transactionCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		periodRef := ""
		if e.PeriodRef.Valid {
			periodRef = " " + e.PeriodRef.String
		}
		desc := fmt.Sprintf("Gaji %s%s", e.EmployeeName, periodRef)

		var onePg pgtype.Numeric
		_ = onePg.Scan("1.00") // constant, cannot fail

		_, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      transactionCode,
			TransactionDate:      e.PayrollDate,
			ItemID:               pgtype.UUID{},
			Description:          desc,
			Quantity:             onePg,
			UnitPrice:            e.GrossPay,
			Amount:               e.GrossPay,
			LineType:             "EXPENSE",
			AccountID:            accountID,
			CashAccountID:        uuidToPgUUID(e.CashAccountID),
			OutletID:             e.OutletID,
			ReimbursementBatchID: pgtype.Text{},
		})
		if err != nil {
			log.Printf("ERROR: create payroll cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		txCount++
	}

	// TODO: Wrap transaction creation + MarkPayrollEntriesPosted in a DB transaction
	// for atomicity. If MarkPayrollEntriesPosted fails after transactions are created,
	// a retry would create duplicates. Same pattern as sales/reimbursement handlers —
	// acceptable for single-user accounting module.
	err = h.store.MarkPayrollEntriesPosted(r.Context(), ids)
	if err != nil {
		log.Printf("ERROR: mark payroll posted: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, postPayrollResponse{
		PostedCount:         len(entries),
		TransactionsCreated: txCount,
	})
}

// --- Helper functions ---

func toPayrollEntryResponse(row database.AcctPayrollEntry) payrollEntryResponse {
	resp := payrollEntryResponse{
		ID:            row.ID,
		PayrollDate:   row.PayrollDate.Time.Format("2006-01-02"),
		PeriodType:    row.PeriodType,
		EmployeeName:  row.EmployeeName,
		GrossPay:      numericToString(row.GrossPay),
		PaymentMethod: row.PaymentMethod,
		CashAccountID: row.CashAccountID.String(),
		CreatedAt:     row.CreatedAt,
	}
	if row.PeriodRef.Valid {
		resp.PeriodRef = &row.PeriodRef.String
	}
	if row.OutletID.Valid {
		idStr := uuid.UUID(row.OutletID.Bytes).String()
		resp.OutletID = &idStr
	}
	if row.PostedAt.Valid {
		t := row.PostedAt.Time
		resp.PostedAt = &t
	}
	return resp
}
