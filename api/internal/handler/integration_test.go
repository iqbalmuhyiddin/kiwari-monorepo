//go:build integration

package handler_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kiwari-pos/api/internal/config"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/router"
	"github.com/kiwari-pos/api/internal/ws"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/crypto/bcrypt"
)

// TestIntegrationFlow exercises the full API lifecycle against a real PostgreSQL database.
// This is the first test that runs the full stack with all handlers wired through the router.
func TestIntegrationFlow(t *testing.T) {
	ctx := context.Background()

	// Start PostgreSQL container
	pgContainer, connStr, cleanup := setupPostgresContainer(t, ctx)
	defer cleanup()

	// Run migrations
	runMigrations(t, connStr)

	// Create pgxpool connection
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	defer pool.Close()

	// Initialize dependencies
	cfg := &config.Config{
		Port:        "8081",
		DatabaseURL: connStr,
		JWTSecret:   "integration-test-secret",
	}
	queries := database.New(pool)
	hub := ws.NewHub()
	// NOTE: hub.Run() goroutine leaks on test exit — Hub has no shutdown mechanism.
	// Acceptable for tests; production should add context-based shutdown.
	go hub.Run()

	// Build router
	r := router.New(cfg, queries, pool, hub)

	// Create HTTP test server
	server := httptest.NewServer(r)
	defer server.Close()

	// --- 1. Create outlet (manual DB insert - no outlet handler) ---
	outletID := createOutlet(t, ctx, pool)

	// --- 2. Create owner user (manual DB insert to bootstrap) ---
	ownerID := createOwnerUser(t, ctx, pool, outletID)

	// --- 3. Login as owner ---
	token := login(t, server, "owner@test.com", "password123")

	// --- 4. Create cashier user through API ---
	cashierResp := createCashierUser(t, server, outletID, token)
	cashierID := uuid.MustParse(cashierResp["id"].(string))

	// --- 5. Create category ---
	categoryResp := createCategory(t, server, outletID, token)
	categoryID := uuid.MustParse(categoryResp["id"].(string))

	// --- 6. Create product in that category ---
	productResp := createProduct(t, server, outletID, categoryID, token)
	productID := uuid.MustParse(productResp["id"].(string))

	// --- 7. Create variant group + variant for product ---
	variantGroupResp := createVariantGroup(t, server, outletID, productID, token)
	variantGroupID := uuid.MustParse(variantGroupResp["id"].(string))
	variantResp := createVariant(t, server, outletID, productID, variantGroupID, token)
	variantID := uuid.MustParse(variantResp["id"].(string))

	// --- 8. Create modifier group + modifier for product ---
	modifierGroupResp := createModifierGroup(t, server, outletID, productID, token)
	modifierGroupID := uuid.MustParse(modifierGroupResp["id"].(string))
	modifierResp := createModifier(t, server, outletID, productID, modifierGroupID, token)
	modifierID := uuid.MustParse(modifierResp["id"].(string))

	// --- 9. Create order with items (including variant and modifier selections) ---
	orderResp := createOrder(t, server, outletID, productID, variantID, modifierID, token)
	orderID := uuid.MustParse(orderResp["id"].(string))
	totalAmount := orderResp["total_amount"].(string)

	// Assert price snapshot calculation is correct:
	// Base price: 25000, Variant adjustment: 2000 → Unit price: 27000
	// Item quantity: 2 → Item line: 27000 * 2 = 54000
	// Modifier: 3000 * 1 = 3000 (added once per item line, not per item qty)
	// Expected total: 54000 + 3000 = 57000
	expectedTotal := "57000.00"
	if totalAmount != expectedTotal {
		t.Fatalf("order total_amount: got %s, want %s (price snapshot verification failed)", totalAmount, expectedTotal)
	}

	// --- 10. Add multi-payment to order (split CASH + QRIS) ---
	// First payment: 30000 CASH (partial payment)
	payment1Resp := addPayment(t, server, outletID, orderID, "30000", token)
	payment1, ok := payment1Resp["payment"].(map[string]interface{})
	if !ok {
		t.Fatalf("payment1 response missing 'payment' field")
	}
	if payment1["payment_method"].(string) != "CASH" {
		t.Fatalf("payment1 method: got %s, want CASH", payment1["payment_method"].(string))
	}

	// Verify order is NOT completed after partial payment
	orderAfterPartial := getOrder(t, server, outletID, orderID, token)
	if orderAfterPartial["status"].(string) == "COMPLETED" {
		t.Fatalf("order status after partial payment: got COMPLETED, want NOT COMPLETED (multi-payment test)")
	}

	// Second payment: 27000 QRIS (remaining balance)
	payment2Resp := addPaymentQRIS(t, server, outletID, orderID, "27000", token)
	payment2, ok := payment2Resp["payment"].(map[string]interface{})
	if !ok {
		t.Fatalf("payment2 response missing 'payment' field")
	}
	if payment2["payment_method"].(string) != "QRIS" {
		t.Fatalf("payment2 method: got %s, want QRIS", payment2["payment_method"].(string))
	}

	// --- 11. Verify order auto-completes when fully paid (after second payment) ---
	verifyOrderCompleted(t, server, outletID, orderID, token)

	// --- 12. Create customer, associate with order ---
	customerResp := createCustomer(t, server, outletID, token)
	customerID := uuid.MustParse(customerResp["id"].(string))
	associateCustomerWithOrder(t, ctx, pool, orderID, customerID)

	// --- 13. Check customer stats ---
	statsResp := getCustomerStats(t, server, outletID, customerID, token)
	if statsResp["total_orders"].(float64) != 1 {
		t.Fatalf("customer total_orders: got %v, want 1", statsResp["total_orders"])
	}

	t.Logf("Integration test passed: container=%s, outlet=%s, owner=%s, cashier=%s, product=%s, order=%s, customer=%s",
		pgContainer.GetContainerID(), outletID, ownerID, cashierID, productID, orderID, customerID)
}

// --- Setup helpers ---

func setupPostgresContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string, func()) {
	t.Helper()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("pos_test"),
		tcpostgres.WithUsername("pos"),
		tcpostgres.WithPassword("pos"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("terminate container: %v", err)
		}
	}

	return pgContainer, connStr, cleanup
}

func runMigrations(t *testing.T, connStr string) {
	t.Helper()

	// Connect with stdlib for migrate
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open db for migrations: %v", err)
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		t.Fatalf("create migrate driver: %v", err)
	}

	// Path relative to this test file's package directory (api/internal/handler/).
	// Go test sets cwd to the package directory.
	m, err := migrate.NewWithDatabaseInstance(
		"file://../../migrations",
		"postgres", driver)
	if err != nil {
		t.Fatalf("create migrate instance: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("run migrations: %v", err)
	}
}

func createOutlet(t *testing.T, ctx context.Context, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO outlets (name, address, phone)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		"Test Outlet", "123 Test St", "08123456789",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create outlet: %v", err)
	}
	return id
}

func createOwnerUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, outletID uuid.UUID) uuid.UUID {
	t.Helper()
	// Hash password using bcrypt
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	var id uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (outlet_id, email, hashed_password, full_name, role)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id`,
		outletID, "owner@test.com", string(hashedPassword), "Test Owner", "OWNER",
	).Scan(&id)
	if err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	return id
}

// --- API call helpers ---

func createCashierUser(t *testing.T, server *httptest.Server, outletID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"email":     "cashier@test.com",
		"password":  "password123",
		"full_name": "Test Cashier",
		"role":      "CASHIER",
		"pin":       "1234",
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/users", outletID), body, token)
}

func login(t *testing.T, server *httptest.Server, email, password string) string {
	t.Helper()
	body := map[string]interface{}{
		"email":    email,
		"password": password,
	}
	resp := httpPostJSON(t, server, "/auth/login", body, "")
	token, ok := resp["access_token"].(string)
	if !ok || token == "" {
		t.Fatalf("login failed: no access_token in response: %+v", resp)
	}
	return token
}

func createCategory(t *testing.T, server *httptest.Server, outletID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":        "Main Dishes",
		"description": "Primary menu items",
		"sort_order":  1,
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/categories", outletID), body, token)
}

func createProduct(t *testing.T, server *httptest.Server, outletID, categoryID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"category_id":       categoryID.String(),
		"name":              "Nasi Bakar Ayam",
		"description":       "Grilled rice with chicken",
		"base_price":        "25000",
		"station":           "GRILL",
		"preparation_time":  15,
		"is_combo":          false,
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/products", outletID), body, token)
}

func createVariantGroup(t *testing.T, server *httptest.Server, outletID, productID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":        "Spice Level",
		"is_required": false,
		"sort_order":  1,
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/products/%s/variant-groups", outletID, productID), body, token)
}

func createVariant(t *testing.T, server *httptest.Server, outletID, productID, variantGroupID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":             "Extra Spicy",
		"price_adjustment": "2000",
		"sort_order":       1,
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/products/%s/variant-groups/%s/variants", outletID, productID, variantGroupID), body, token)
}

func createModifierGroup(t *testing.T, server *httptest.Server, outletID, productID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":       "Add-ons",
		"min_select": 0,
		"max_select": 3,
		"sort_order": 1,
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/products/%s/modifier-groups", outletID, productID), body, token)
}

func createModifier(t *testing.T, server *httptest.Server, outletID, productID, modifierGroupID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":       "Extra Sambal",
		"price":      "3000",
		"sort_order": 1,
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/products/%s/modifier-groups/%s/modifiers", outletID, productID, modifierGroupID), body, token)
}

func createOrder(t *testing.T, server *httptest.Server, outletID, productID, variantID, modifierID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"order_type": "DINE_IN",
		"items": []map[string]interface{}{
			{
				"product_id": productID.String(),
				"variant_id": variantID.String(),
				"quantity":   2,
				"modifiers": []map[string]interface{}{
					{
						"modifier_id": modifierID.String(),
						"quantity":    1,
					},
				},
			},
		},
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/orders", outletID), body, token)
}

func addPayment(t *testing.T, server *httptest.Server, outletID, orderID uuid.UUID, amount, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"payment_method":  "CASH",
		"amount":          amount,
		"amount_received": "100000",
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/orders/%s/payments", outletID, orderID), body, token)
}

func addPaymentQRIS(t *testing.T, server *httptest.Server, outletID, orderID uuid.UUID, amount, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"payment_method":   "QRIS",
		"amount":           amount,
		"reference_number": "QRIS-REF-12345",
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/orders/%s/payments", outletID, orderID), body, token)
}

func getOrder(t *testing.T, server *httptest.Server, outletID, orderID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	return httpGetJSON(t, server, fmt.Sprintf("/outlets/%s/orders/%s", outletID, orderID), token)
}

func verifyOrderCompleted(t *testing.T, server *httptest.Server, outletID, orderID uuid.UUID, token string) {
	t.Helper()
	resp := httpGetJSON(t, server, fmt.Sprintf("/outlets/%s/orders/%s", outletID, orderID), token)
	status, ok := resp["status"].(string)
	if !ok {
		t.Fatalf("order status missing from response")
	}
	if status != "COMPLETED" {
		t.Fatalf("order status: got %s, want COMPLETED", status)
	}
}

func createCustomer(t *testing.T, server *httptest.Server, outletID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	body := map[string]interface{}{
		"name":  "John Doe",
		"phone": "081234567890",
		"email": "john@test.com",
	}
	return httpPostJSON(t, server, fmt.Sprintf("/outlets/%s/customers", outletID), body, token)
}

// No API endpoint exists to associate customer with order post-creation.
// Direct DB update is required to test customer stats.
func associateCustomerWithOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, orderID, customerID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`UPDATE orders SET customer_id = $1 WHERE id = $2`,
		customerID, orderID,
	)
	if err != nil {
		t.Fatalf("associate customer with order: %v", err)
	}
}

func getCustomerStats(t *testing.T, server *httptest.Server, outletID, customerID uuid.UUID, token string) map[string]interface{} {
	t.Helper()
	return httpGetJSON(t, server, fmt.Sprintf("/outlets/%s/customers/%s/stats", outletID, customerID), token)
}

// --- HTTP helpers ---

func httpPostJSON(t *testing.T, server *httptest.Server, path string, body map[string]interface{}, token string) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	req, err := http.NewRequest("POST", server.URL+path, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("POST %s: status %d, body: %v", path, resp.StatusCode, errResp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}

func httpGetJSON(t *testing.T, server *httptest.Server, path string, token string) map[string]interface{} {
	t.Helper()
	req, err := http.NewRequest("GET", server.URL+path, nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		t.Fatalf("GET %s: status %d, body: %v", path, resp.StatusCode, errResp)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return result
}
