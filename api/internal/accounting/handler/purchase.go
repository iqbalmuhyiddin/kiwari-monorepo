package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/shopspring/decimal"
)

// --- Store interface ---

// PurchaseStore defines the database methods needed by purchase handlers.
type PurchaseStore interface {
	CreateAcctCashTransaction(ctx context.Context, arg database.CreateAcctCashTransactionParams) (database.AcctCashTransaction, error)
	GetNextTransactionCode(ctx context.Context) (string, error)
	UpdateAcctItemLastPrice(ctx context.Context, arg database.UpdateAcctItemLastPriceParams) error
}

// --- PurchaseHandler ---

// PurchaseHandler handles purchase entry endpoints.
type PurchaseHandler struct {
	store PurchaseStore
}

// NewPurchaseHandler creates a new PurchaseHandler.
func NewPurchaseHandler(store PurchaseStore) *PurchaseHandler {
	return &PurchaseHandler{
		store: store,
	}
}

// RegisterRoutes registers purchase endpoints.
func (h *PurchaseHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.CreatePurchase)
}

// --- Request / Response types ---

type createPurchaseRequest struct {
	TransactionDate string                `json:"transaction_date"` // "2026-01-20"
	AccountID       string                `json:"account_id"`       // UUID (inventory account)
	CashAccountID   string                `json:"cash_account_id"`  // UUID
	OutletID        *string               `json:"outlet_id"`        // optional UUID
	Items           []purchaseItemRequest `json:"items"`
}

type purchaseItemRequest struct {
	ItemID      *string `json:"item_id"`    // optional UUID (for matching)
	Description string  `json:"description"`
	Quantity    string  `json:"quantity"`   // decimal string
	UnitPrice   string  `json:"unit_price"` // decimal string
}

type purchaseResponse struct {
	Transactions []transactionResponse `json:"transactions"`
}

type transactionResponse struct {
	ID              uuid.UUID `json:"id"`
	TransactionCode string    `json:"transaction_code"`
	TransactionDate string    `json:"transaction_date"`
	Description     string    `json:"description"`
	Quantity        string    `json:"quantity"`
	UnitPrice       string    `json:"unit_price"`
	Amount          string    `json:"amount"`
	LineType        string    `json:"line_type"`
	ItemID          *string   `json:"item_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// --- Handlers ---

// CreatePurchase records a purchase entry with multiple items.
func (h *PurchaseHandler) CreatePurchase(w http.ResponseWriter, r *http.Request) {
	var req createPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.TransactionDate == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "transaction_date is required"})
		return
	}
	if req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "account_id is required"})
		return
	}
	if req.CashAccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cash_account_id is required"})
		return
	}
	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items cannot be empty"})
		return
	}

	// Validate and parse transaction_date
	date, err := time.Parse("2006-01-02", req.TransactionDate)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transaction_date format, expected YYYY-MM-DD"})
		return
	}
	pgDate := pgtype.Date{Time: date, Valid: true}

	// Parse account_id
	accountID, err := uuid.Parse(req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id"})
		return
	}

	// Parse cash_account_id
	cashAccountID, err := uuid.Parse(req.CashAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cash_account_id"})
		return
	}

	// Parse optional outlet_id
	var outletID pgtype.UUID
	if req.OutletID != nil && *req.OutletID != "" {
		id, err := uuid.Parse(*req.OutletID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid outlet_id"})
			return
		}
		outletID = uuidToPgUUID(id)
	}

	// Get next transaction code
	maxCode, err := h.store.GetNextTransactionCode(r.Context())
	if err != nil {
		log.Printf("ERROR: get next transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	// Parse and increment transaction code
	// maxCode format: "PCS000000" or "PCS000123"
	numStr := maxCode[3:] // Extract numeric suffix
	nextNum, err := strconv.Atoi(numStr)
	if err != nil {
		log.Printf("ERROR: parse transaction code: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	nextNum++ // Start from next number

	// Process each item
	var transactions []transactionResponse
	for _, itemReq := range req.Items {
		// Validate item fields
		if itemReq.Description == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item description is required"})
			return
		}
		if itemReq.Quantity == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item quantity is required"})
			return
		}
		if itemReq.UnitPrice == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "item unit_price is required"})
			return
		}

		// Parse quantity and unit_price
		qty, err := decimal.NewFromString(itemReq.Quantity)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid quantity format"})
			return
		}
		price, err := decimal.NewFromString(itemReq.UnitPrice)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid unit_price format"})
			return
		}

		// Calculate amount
		amount := qty.Mul(price)

		// Convert to pgtype.Numeric
		var qtyPg, pricePg, amountPg pgtype.Numeric
		qtyStr := qty.StringFixed(2)
		priceStr := price.StringFixed(2)
		amountStr := amount.StringFixed(2)

		if err := qtyPg.Scan(qtyStr); err != nil {
			log.Printf("ERROR: scan quantity: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		if err := pricePg.Scan(priceStr); err != nil {
			log.Printf("ERROR: scan price: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}
		if err := amountPg.Scan(amountStr); err != nil {
			log.Printf("ERROR: scan amount: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Parse optional item_id
		var itemID pgtype.UUID
		if itemReq.ItemID != nil && *itemReq.ItemID != "" {
			id, err := uuid.Parse(*itemReq.ItemID)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item_id"})
				return
			}
			itemID = uuidToPgUUID(id)
		}

		// Generate transaction code
		transactionCode := fmt.Sprintf("PCS%06d", nextNum)
		nextNum++

		// Create transaction
		tx, err := h.store.CreateAcctCashTransaction(r.Context(), database.CreateAcctCashTransactionParams{
			TransactionCode:      transactionCode,
			TransactionDate:      pgDate,
			ItemID:               itemID,
			Description:          itemReq.Description,
			Quantity:             qtyPg,
			UnitPrice:            pricePg,
			Amount:               amountPg,
			LineType:             "INVENTORY",
			AccountID:            accountID,
			CashAccountID:        uuidToPgUUID(cashAccountID),
			OutletID:             outletID,
			ReimbursementBatchID: pgtype.Text{}, // empty
		})
		if err != nil {
			log.Printf("ERROR: create cash transaction: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
			return
		}

		// Update item last_price if item_id is set
		if itemID.Valid {
			if err := h.store.UpdateAcctItemLastPrice(r.Context(), database.UpdateAcctItemLastPriceParams{
				ID:        uuid.UUID(itemID.Bytes),
				LastPrice: pricePg,
			}); err != nil {
				log.Printf("ERROR: update item last price: %v", err)
				// Continue even if this fails (non-critical)
			}
		}

		// Build response
		var itemIDPtr *string
		if itemID.Valid {
			idStr := uuid.UUID(itemID.Bytes).String()
			itemIDPtr = &idStr
		}

		transactions = append(transactions, transactionResponse{
			ID:              tx.ID,
			TransactionCode: tx.TransactionCode,
			TransactionDate: req.TransactionDate,
			Description:     tx.Description,
			Quantity:        qtyStr,
			UnitPrice:       priceStr,
			Amount:          amountStr,
			LineType:        tx.LineType,
			ItemID:          itemIDPtr,
			CreatedAt:       tx.CreatedAt,
		})
	}

	writeJSON(w, http.StatusCreated, purchaseResponse{
		Transactions: transactions,
	})
}

// --- Helper functions ---

// uuidToPgUUID converts google/uuid.UUID to pgtype.UUID.
func uuidToPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
