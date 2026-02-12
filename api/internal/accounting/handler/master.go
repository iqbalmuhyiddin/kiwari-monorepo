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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interfaces ---

// AcctAccountStore defines the database methods needed by account handlers.
type AcctAccountStore interface {
	ListAcctAccounts(ctx context.Context) ([]database.AcctAccount, error)
	GetAcctAccount(ctx context.Context, id uuid.UUID) (database.AcctAccount, error)
	CreateAcctAccount(ctx context.Context, arg database.CreateAcctAccountParams) (database.AcctAccount, error)
	UpdateAcctAccount(ctx context.Context, arg database.UpdateAcctAccountParams) (database.AcctAccount, error)
	SoftDeleteAcctAccount(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// AcctItemStore defines the database methods needed by item handlers.
type AcctItemStore interface {
	ListAcctItems(ctx context.Context) ([]database.AcctItem, error)
	GetAcctItem(ctx context.Context, id uuid.UUID) (database.AcctItem, error)
	CreateAcctItem(ctx context.Context, arg database.CreateAcctItemParams) (database.AcctItem, error)
	UpdateAcctItem(ctx context.Context, arg database.UpdateAcctItemParams) (database.AcctItem, error)
	SoftDeleteAcctItem(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// AcctCashAccountStore defines the database methods needed by cash account handlers.
type AcctCashAccountStore interface {
	ListAcctCashAccounts(ctx context.Context) ([]database.AcctCashAccount, error)
	GetAcctCashAccount(ctx context.Context, id uuid.UUID) (database.AcctCashAccount, error)
	CreateAcctCashAccount(ctx context.Context, arg database.CreateAcctCashAccountParams) (database.AcctCashAccount, error)
	UpdateAcctCashAccount(ctx context.Context, arg database.UpdateAcctCashAccountParams) (database.AcctCashAccount, error)
	SoftDeleteAcctCashAccount(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
}

// --- MasterHandler ---

// MasterHandler handles CRUD endpoints for accounting master data.
type MasterHandler struct {
	acctStore     AcctAccountStore
	itemStore     AcctItemStore
	cashAcctStore AcctCashAccountStore
}

// NewMasterHandler creates a new MasterHandler.
func NewMasterHandler(acctStore AcctAccountStore, itemStore AcctItemStore, cashAcctStore AcctCashAccountStore) *MasterHandler {
	return &MasterHandler{
		acctStore:     acctStore,
		itemStore:     itemStore,
		cashAcctStore: cashAcctStore,
	}
}

// --- Request / Response types for Accounts ---

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
		IsActive:    a.IsActive,
		CreatedAt:   a.CreatedAt,
	}
}

// --- Request / Response types for Items ---

type createItemRequest struct {
	ItemCode     string  `json:"item_code"`
	ItemName     string  `json:"item_name"`
	ItemCategory string  `json:"item_category"`
	Unit         string  `json:"unit"`
	IsInventory  bool    `json:"is_inventory"`
	AveragePrice *string `json:"average_price"`
	LastPrice    *string `json:"last_price"`
	ForHpp       *string `json:"for_hpp"`
	Keywords     string  `json:"keywords"`
}

type updateItemRequest struct {
	ItemName     string  `json:"item_name"`
	ItemCategory string  `json:"item_category"`
	Unit         string  `json:"unit"`
	IsInventory  bool    `json:"is_inventory"`
	AveragePrice *string `json:"average_price"`
	LastPrice    *string `json:"last_price"`
	ForHpp       *string `json:"for_hpp"`
	Keywords     string  `json:"keywords"`
}

type itemResponse struct {
	ID           uuid.UUID `json:"id"`
	ItemCode     string    `json:"item_code"`
	ItemName     string    `json:"item_name"`
	ItemCategory string    `json:"item_category"`
	Unit         string    `json:"unit"`
	IsInventory  bool      `json:"is_inventory"`
	IsActive     bool      `json:"is_active"`
	AveragePrice *string   `json:"average_price"`
	LastPrice    *string   `json:"last_price"`
	ForHpp       *string   `json:"for_hpp"`
	Keywords     string    `json:"keywords"`
	CreatedAt    time.Time `json:"created_at"`
}

func toItemResponse(i database.AcctItem) itemResponse {
	resp := itemResponse{
		ID:           i.ID,
		ItemCode:     i.ItemCode,
		ItemName:     i.ItemName,
		ItemCategory: i.ItemCategory,
		Unit:         i.Unit,
		IsInventory:  i.IsInventory,
		IsActive:     i.IsActive,
		Keywords:     i.Keywords,
		CreatedAt:    i.CreatedAt,
	}
	resp.AveragePrice = numericToStringPtr(i.AveragePrice)
	resp.LastPrice = numericToStringPtr(i.LastPrice)
	resp.ForHpp = numericToStringPtr(i.ForHpp)
	return resp
}

// numericToStringPtr converts pgtype.Numeric to *string with 2 decimal places.
func numericToStringPtr(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	val, err := n.Value()
	if err != nil || val == nil {
		return nil
	}
	s, ok := val.(string)
	if !ok {
		return nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil
	}
	result := d.StringFixed(2)
	return &result
}

// --- Request / Response types for Cash Accounts ---

type createCashAccountRequest struct {
	CashAccountCode string  `json:"cash_account_code"`
	CashAccountName string  `json:"cash_account_name"`
	BankName        *string `json:"bank_name"`
	Ownership       string  `json:"ownership"`
}

type updateCashAccountRequest struct {
	CashAccountName string  `json:"cash_account_name"`
	BankName        *string `json:"bank_name"`
	Ownership       string  `json:"ownership"`
}

type cashAccountResponse struct {
	ID              uuid.UUID `json:"id"`
	CashAccountCode string    `json:"cash_account_code"`
	CashAccountName string    `json:"cash_account_name"`
	BankName        *string   `json:"bank_name"`
	Ownership       string    `json:"ownership"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
}

func toCashAccountResponse(c database.AcctCashAccount) cashAccountResponse {
	resp := cashAccountResponse{
		ID:              c.ID,
		CashAccountCode: c.CashAccountCode,
		CashAccountName: c.CashAccountName,
		Ownership:       c.Ownership,
		IsActive:        c.IsActive,
		CreatedAt:       c.CreatedAt,
	}
	if c.BankName.Valid {
		resp.BankName = &c.BankName.String
	}
	return resp
}

// --- Helper functions ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR: failed to encode JSON response: %v", err)
	}
}

func stringToPgNumeric(s *string) pgtype.Numeric {
	if s == nil || *s == "" {
		return pgtype.Numeric{}
	}
	var n pgtype.Numeric
	if err := n.Scan(*s); err != nil {
		return pgtype.Numeric{}
	}
	return n
}

func stringToPgText(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// --- Account Routes ---

// RegisterAccountRoutes registers account CRUD endpoints.
func (h *MasterHandler) RegisterAccountRoutes(r chi.Router) {
	r.Get("/", h.ListAccounts)
	r.Post("/", h.CreateAccount)
	r.Put("/{id}", h.UpdateAccount)
	r.Delete("/{id}", h.DeleteAccount)
}

// ListAccounts returns all active accounts.
func (h *MasterHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.acctStore.ListAcctAccounts(r.Context())
	if err != nil {
		log.Printf("ERROR: list accounts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateAccount adds a new account.
func (h *MasterHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req createAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.AccountCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_code is required"})
		return
	}
	if req.AccountName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_name is required"})
		return
	}
	if req.AccountType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_type is required"})
		return
	}
	if req.LineType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type is required"})
		return
	}

	// Validate account_type
	validAccountTypes := map[string]bool{
		"Asset":     true,
		"Liability": true,
		"Equity":    true,
		"Revenue":   true,
		"Expense":   true,
	}
	if !validAccountTypes[req.AccountType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_type must be one of: Asset, Liability, Equity, Revenue, Expense"})
		return
	}

	// Validate line_type
	validLineTypes := map[string]bool{
		"ASSET":     true,
		"INVENTORY": true,
		"EXPENSE":   true,
		"SALES":     true,
		"COGS":      true,
		"LIABILITY": true,
		"CAPITAL":   true,
		"DRAWING":   true,
	}
	if !validLineTypes[req.LineType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type must be one of: ASSET, INVENTORY, EXPENSE, SALES, COGS, LIABILITY, CAPITAL, DRAWING"})
		return
	}

	account, err := h.acctStore.CreateAcctAccount(r.Context(), database.CreateAcctAccountParams{
		AccountCode: req.AccountCode,
		AccountName: req.AccountName,
		AccountType: req.AccountType,
		LineType:    req.LineType,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "account_code already exists"})
			return
		}
		log.Printf("ERROR: create account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toAccountResponse(account))
}

// UpdateAccount modifies an existing account.
func (h *MasterHandler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account ID"})
		return
	}

	var req updateAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.AccountName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_name is required"})
		return
	}
	if req.AccountType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_type is required"})
		return
	}
	if req.LineType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type is required"})
		return
	}

	// Validate account_type
	validAccountTypes := map[string]bool{
		"Asset":     true,
		"Liability": true,
		"Equity":    true,
		"Revenue":   true,
		"Expense":   true,
	}
	if !validAccountTypes[req.AccountType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_type must be one of: Asset, Liability, Equity, Revenue, Expense"})
		return
	}

	// Validate line_type
	validLineTypes := map[string]bool{
		"ASSET":     true,
		"INVENTORY": true,
		"EXPENSE":   true,
		"SALES":     true,
		"COGS":      true,
		"LIABILITY": true,
		"CAPITAL":   true,
		"DRAWING":   true,
	}
	if !validLineTypes[req.LineType] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "line_type must be one of: ASSET, INVENTORY, EXPENSE, SALES, COGS, LIABILITY, CAPITAL, DRAWING"})
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
		log.Printf("ERROR: update account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toAccountResponse(account))
}

// DeleteAccount soft-deletes an account.
func (h *MasterHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account ID"})
		return
	}

	_, err = h.acctStore.SoftDeleteAcctAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "account not found"})
			return
		}
		log.Printf("ERROR: delete account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Item Routes ---

// RegisterItemRoutes registers item CRUD endpoints.
func (h *MasterHandler) RegisterItemRoutes(r chi.Router) {
	r.Get("/", h.ListItems)
	r.Post("/", h.CreateItem)
	r.Put("/{id}", h.UpdateItem)
	r.Delete("/{id}", h.DeleteItem)
}

// ListItems returns all active items.
func (h *MasterHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.itemStore.ListAcctItems(r.Context())
	if err != nil {
		log.Printf("ERROR: list items: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]itemResponse, len(items))
	for i, item := range items {
		resp[i] = toItemResponse(item)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateItem adds a new item.
func (h *MasterHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ItemCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_code is required"})
		return
	}
	if req.ItemName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_name is required"})
		return
	}
	if req.ItemCategory == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_category is required"})
		return
	}
	if req.Unit == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unit is required"})
		return
	}
	if req.Keywords == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "keywords is required"})
		return
	}

	// Validate item_category
	validCategories := map[string]bool{
		"Raw Material": true,
		"Packaging":    true,
		"Consumable":   true,
	}
	if !validCategories[req.ItemCategory] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_category must be one of: Raw Material, Packaging, Consumable"})
		return
	}

	item, err := h.itemStore.CreateAcctItem(r.Context(), database.CreateAcctItemParams{
		ItemCode:     req.ItemCode,
		ItemName:     req.ItemName,
		ItemCategory: req.ItemCategory,
		Unit:         req.Unit,
		IsInventory:  req.IsInventory,
		AveragePrice: stringToPgNumeric(req.AveragePrice),
		LastPrice:    stringToPgNumeric(req.LastPrice),
		ForHpp:       stringToPgNumeric(req.ForHpp),
		Keywords:     req.Keywords,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "item_code already exists"})
			return
		}
		log.Printf("ERROR: create item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toItemResponse(item))
}

// UpdateItem modifies an existing item.
func (h *MasterHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item ID"})
		return
	}

	var req updateItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.ItemName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_name is required"})
		return
	}
	if req.ItemCategory == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_category is required"})
		return
	}
	if req.Unit == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unit is required"})
		return
	}
	if req.Keywords == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "keywords is required"})
		return
	}

	// Validate item_category
	validCategories := map[string]bool{
		"Raw Material": true,
		"Packaging":    true,
		"Consumable":   true,
	}
	if !validCategories[req.ItemCategory] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item_category must be one of: Raw Material, Packaging, Consumable"})
		return
	}

	item, err := h.itemStore.UpdateAcctItem(r.Context(), database.UpdateAcctItemParams{
		ID:           id,
		ItemName:     req.ItemName,
		ItemCategory: req.ItemCategory,
		Unit:         req.Unit,
		IsInventory:  req.IsInventory,
		AveragePrice: stringToPgNumeric(req.AveragePrice),
		LastPrice:    stringToPgNumeric(req.LastPrice),
		ForHpp:       stringToPgNumeric(req.ForHpp),
		Keywords:     req.Keywords,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		log.Printf("ERROR: update item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toItemResponse(item))
}

// DeleteItem soft-deletes an item.
func (h *MasterHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item ID"})
		return
	}

	_, err = h.itemStore.SoftDeleteAcctItem(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
		log.Printf("ERROR: delete item: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Cash Account Routes ---

// RegisterCashAccountRoutes registers cash account CRUD endpoints.
func (h *MasterHandler) RegisterCashAccountRoutes(r chi.Router) {
	r.Get("/", h.ListCashAccounts)
	r.Post("/", h.CreateCashAccount)
	r.Put("/{id}", h.UpdateCashAccount)
	r.Delete("/{id}", h.DeleteCashAccount)
}

// ListCashAccounts returns all active cash accounts.
func (h *MasterHandler) ListCashAccounts(w http.ResponseWriter, r *http.Request) {
	cashAccounts, err := h.cashAcctStore.ListAcctCashAccounts(r.Context())
	if err != nil {
		log.Printf("ERROR: list cash accounts: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]cashAccountResponse, len(cashAccounts))
	for i, ca := range cashAccounts {
		resp[i] = toCashAccountResponse(ca)
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateCashAccount adds a new cash account.
func (h *MasterHandler) CreateCashAccount(w http.ResponseWriter, r *http.Request) {
	var req createCashAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.CashAccountCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_code is required"})
		return
	}
	if req.CashAccountName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_name is required"})
		return
	}
	if req.Ownership == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownership is required"})
		return
	}

	// Validate ownership
	validOwnership := map[string]bool{
		"Business": true,
		"Personal": true,
	}
	if !validOwnership[req.Ownership] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownership must be one of: Business, Personal"})
		return
	}

	cashAccount, err := h.cashAcctStore.CreateAcctCashAccount(r.Context(), database.CreateAcctCashAccountParams{
		CashAccountCode: req.CashAccountCode,
		CashAccountName: req.CashAccountName,
		BankName:        stringToPgText(req.BankName),
		Ownership:       req.Ownership,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "cash_account_code already exists"})
			return
		}
		log.Printf("ERROR: create cash account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toCashAccountResponse(cashAccount))
}

// UpdateCashAccount modifies an existing cash account.
func (h *MasterHandler) UpdateCashAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash account ID"})
		return
	}

	var req updateCashAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.CashAccountName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_name is required"})
		return
	}
	if req.Ownership == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownership is required"})
		return
	}

	// Validate ownership
	validOwnership := map[string]bool{
		"Business": true,
		"Personal": true,
	}
	if !validOwnership[req.Ownership] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownership must be one of: Business, Personal"})
		return
	}

	cashAccount, err := h.cashAcctStore.UpdateAcctCashAccount(r.Context(), database.UpdateAcctCashAccountParams{
		ID:              id,
		CashAccountName: req.CashAccountName,
		BankName:        stringToPgText(req.BankName),
		Ownership:       req.Ownership,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "cash account not found"})
			return
		}
		log.Printf("ERROR: update cash account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, toCashAccountResponse(cashAccount))
}

// DeleteCashAccount soft-deletes a cash account.
func (h *MasterHandler) DeleteCashAccount(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash account ID"})
		return
	}

	_, err = h.cashAcctStore.SoftDeleteAcctCashAccount(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "cash account not found"})
			return
		}
		log.Printf("ERROR: delete cash account: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
