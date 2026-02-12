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

// --- Handler method ---

// FromWhatsApp processes a WhatsApp reimbursement message webhook.
func (h *WhatsAppHandler) FromWhatsApp(w http.ResponseWriter, r *http.Request) {
	var req whatsAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate required fields
	if req.SenderName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sender_name is required"})
		return
	}
	if req.MessageText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "message_text is required"})
		return
	}

	// Parse message
	parsed, err := parser.ParseMessage(req.MessageText)
	if err != nil {
		reply := fmt.Sprintf("❌ Format pesan salah:\n%s\n\nContoh format yang benar:\n20 jan\ncabe merah 5kg 500k\nbawang merah 2kg 300k", err.Error())
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":         "parse error",
			"reply_message": reply,
		})
		return
	}

	// Convert expense_date to pgtype.Date
	expenseDate := pgtype.Date{Time: parsed.ExpenseDate, Valid: true}

	// Track match statistics
	var matched, ambiguous, unmatched []parsedItemWithMatch
	var itemsCreated int

	// Process each parsed item
	for _, item := range parsed.Items {
		var matchResult matcher.MatchResult
		if h.matcher != nil {
			matchResult = h.matcher.Match(item.Description)
		} else {
			matchResult = matcher.MatchResult{Status: matcher.Unmatched}
		}

		// Calculate unit_price = totalPrice / qty
		totalPriceDec := decimal.NewFromFloat(item.TotalPrice)
		qtyDec := decimal.NewFromFloat(item.Qty)
		if qtyDec.IsZero() {
			log.Printf("WARN: skipping item with zero quantity: %s", item.Description)
			continue
		}
		unitPrice := totalPriceDec.Div(qtyDec)

		// Convert to pgtype.Numeric
		var qtyPg, unitPricePg, amountPg pgtype.Numeric
		if err := qtyPg.Scan(qtyDec.StringFixed(2)); err != nil {
			log.Printf("ERROR: scan quantity: %v", err)
			continue
		}
		if err := unitPricePg.Scan(unitPrice.StringFixed(2)); err != nil {
			log.Printf("ERROR: scan unit_price: %v", err)
			continue
		}
		if err := amountPg.Scan(totalPriceDec.StringFixed(2)); err != nil {
			log.Printf("ERROR: scan amount: %v", err)
			continue
		}

		// Build database params based on match status
		var itemID pgtype.UUID
		var lineType string

		switch matchResult.Status {
		case matcher.Matched:
			itemID = pgtype.UUID{Bytes: matchResult.Item.ID, Valid: true}
			lineType = "INVENTORY"
			matched = append(matched, parsedItemWithMatch{item: item, result: matchResult})
		case matcher.Ambiguous:
			lineType = "EXPENSE"
			ambiguous = append(ambiguous, parsedItemWithMatch{item: item, result: matchResult})
		case matcher.Unmatched:
			lineType = "EXPENSE"
			unmatched = append(unmatched, parsedItemWithMatch{item: item, result: matchResult})
		}

		// Create reimbursement request
		_, err := h.store.CreateAcctReimbursementRequest(r.Context(), database.CreateAcctReimbursementRequestParams{
			ExpenseDate: expenseDate,
			ItemID:      itemID,
			Description: item.Description,
			Qty:         qtyPg,
			UnitPrice:   unitPricePg,
			Amount:      amountPg,
			LineType:    lineType,
			AccountID:   h.defaultAccountID,
			Status:      "Draft",
			Requester:   req.SenderName,
		})
		if err != nil {
			log.Printf("ERROR: create reimbursement request: %v", err)
			continue
		}

		itemsCreated++
	}

	// Check if all items failed to save
	if itemsCreated == 0 && len(parsed.Items) > 0 {
		log.Printf("ERROR: all %d items failed to save", len(parsed.Items))
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":         "failed to save reimbursement items",
			"reply_message": "Maaf, terjadi error saat menyimpan data. Coba lagi nanti.",
		})
		return
	}

	// Build reply message
	replyMessage := buildReplyMessage(matched, ambiguous, unmatched, req.SenderName, parsed.ExpenseDate.Format("2 Jan 2006"))

	writeJSON(w, http.StatusOK, whatsAppResponse{
		ReplyMessage:   replyMessage,
		ItemsCreated:   itemsCreated,
		ItemsMatched:   len(matched),
		ItemsAmbiguous: len(ambiguous),
		ItemsUnmatched: len(unmatched),
	})
}

// --- Helper types and functions ---

type parsedItemWithMatch struct {
	item   parser.ParsedItem
	result matcher.MatchResult
}

func buildReplyMessage(matched, ambiguous, unmatched []parsedItemWithMatch, requester, dateStr string) string {
	var sb strings.Builder

	sb.WriteString("✅ Reimburse diterima!\n\n")

	// Matched section
	if len(matched) > 0 {
		sb.WriteString("✔️ Cocok:\n")
		for _, m := range matched {
			qtyUnit := formatQtyUnit(m.item.Qty, m.item.Unit)
			price := formatRupiah(decimal.NewFromFloat(m.item.TotalPrice))
			sb.WriteString(fmt.Sprintf("• %s %s → %s (%s)\n", m.result.Item.Name, qtyUnit, m.item.Description, price))
		}
		sb.WriteString("\n")
	}

	// Ambiguous section
	if len(ambiguous) > 0 {
		sb.WriteString("⚠️ Ambigu (perlu review):\n")
		for _, a := range ambiguous {
			qtyUnit := formatQtyUnit(a.item.Qty, a.item.Unit)
			price := formatRupiah(decimal.NewFromFloat(a.item.TotalPrice))
			candidates := candidateNames(a.result.Candidates)
			sb.WriteString(fmt.Sprintf("• %s %s (%s)\n  Mungkin: %s\n", a.item.Description, qtyUnit, price, candidates))
		}
		sb.WriteString("\n")
	}

	// Unmatched section
	if len(unmatched) > 0 {
		sb.WriteString("❌ Tidak cocok:\n")
		for _, u := range unmatched {
			qtyUnit := formatQtyUnit(u.item.Qty, u.item.Unit)
			price := formatRupiah(decimal.NewFromFloat(u.item.TotalPrice))
			sb.WriteString(fmt.Sprintf("• %s %s (%s)\n", u.item.Description, qtyUnit, price))
		}
		sb.WriteString("\n")
	}

	// Summary
	totalItems := len(matched) + len(ambiguous) + len(unmatched)
	totalAmount := decimal.Zero
	for _, m := range matched {
		totalAmount = totalAmount.Add(decimal.NewFromFloat(m.item.TotalPrice))
	}
	for _, a := range ambiguous {
		totalAmount = totalAmount.Add(decimal.NewFromFloat(a.item.TotalPrice))
	}
	for _, u := range unmatched {
		totalAmount = totalAmount.Add(decimal.NewFromFloat(u.item.TotalPrice))
	}

	sb.WriteString(fmt.Sprintf("Total: %d item = %s\n", totalItems, formatRupiah(totalAmount)))
	sb.WriteString(fmt.Sprintf("Peminta: %s\n", requester))
	sb.WriteString(fmt.Sprintf("Tanggal: %s", dateStr))

	return sb.String()
}

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
