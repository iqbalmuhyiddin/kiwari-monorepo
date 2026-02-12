package handler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/accounting/matcher"
	"github.com/kiwari-pos/api/internal/database"
)

// --- Mock WhatsAppStore ---

type mockWhatsAppStore struct {
	requests map[uuid.UUID]database.AcctReimbursementRequest
}

func newMockWhatsAppStore() *mockWhatsAppStore {
	return &mockWhatsAppStore{requests: make(map[uuid.UUID]database.AcctReimbursementRequest)}
}

func (m *mockWhatsAppStore) CreateAcctReimbursementRequest(_ context.Context, arg database.CreateAcctReimbursementRequestParams) (database.AcctReimbursementRequest, error) {
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

// --- Router setup ---

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
	defaultAccountID := uuid.New()

	// Set up matcher with one item
	items := []matcher.Item{
		{
			ID:       uuid.New(),
			Code:     "RM001",
			Name:     "Cabe Merah Tanjung",
			Keywords: "cabe,merah,tanjung",
			Unit:     "kg",
		},
	}

	router := setupWhatsAppRouter(store, items, defaultAccountID)

	payload := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"message_text": "20 jan\ncabe merah tanjung 5kg 500k",
		"chat_id":      "120363421848364675@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	if resp["items_created"] != float64(1) {
		t.Errorf("items_created: got %v, want 1", resp["items_created"])
	}
	if resp["items_matched"] != float64(1) {
		t.Errorf("items_matched: got %v, want 1", resp["items_matched"])
	}
	if resp["reply_message"] == nil || resp["reply_message"] == "" {
		t.Errorf("reply_message should not be empty")
	}

	// Verify store has 1 request created
	if len(store.requests) != 1 {
		t.Errorf("expected 1 request in store, got %d", len(store.requests))
	}
}

func TestFromWhatsApp_InvalidMessage(t *testing.T) {
	store := newMockWhatsAppStore()
	defaultAccountID := uuid.New()
	router := setupWhatsAppRouter(store, []matcher.Item{}, defaultAccountID)

	// Message without a date line
	payload := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"message_text": "this is not a reimbursement",
		"chat_id":      "120363421848364675@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())
	if resp["error"] == nil {
		t.Errorf("expected error field in response")
	}
}

func TestFromWhatsApp_MissingFields(t *testing.T) {
	store := newMockWhatsAppStore()
	defaultAccountID := uuid.New()
	router := setupWhatsAppRouter(store, []matcher.Item{}, defaultAccountID)

	// Missing message_text
	payload := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"chat_id":      "120363421848364675@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", payload)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())
	if resp["error"] == nil {
		t.Errorf("expected error field in response")
	}
}

func TestFromWhatsApp_UnmatchedItem(t *testing.T) {
	store := newMockWhatsAppStore()
	defaultAccountID := uuid.New()

	// Empty items list — nothing to match
	router := setupWhatsAppRouter(store, []matcher.Item{}, defaultAccountID)

	payload := map[string]interface{}{
		"sender_phone": "+628123456789",
		"sender_name":  "Hamidah",
		"message_text": "20 jan\nxyz unknown item 5kg 500k",
		"chat_id":      "120363421848364675@g.us",
	}

	rr := doRequest(t, router, "POST", "/accounting/reimbursements/from-whatsapp", payload)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	resp := decodeJSON(t, rr.Body.Bytes())

	if resp["items_unmatched"] != float64(1) {
		t.Errorf("items_unmatched: got %v, want 1", resp["items_unmatched"])
	}
	if resp["items_created"] != float64(1) {
		t.Errorf("items_created: got %v, want 1", resp["items_created"])
	}

	// Verify store has 1 request created
	if len(store.requests) != 1 {
		t.Errorf("expected 1 request in store, got %d", len(store.requests))
	}
}
