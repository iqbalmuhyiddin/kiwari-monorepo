package router

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	accthandler "github.com/kiwari-pos/api/internal/accounting/handler"
	"github.com/kiwari-pos/api/internal/config"
	"github.com/kiwari-pos/api/internal/database"
	"github.com/kiwari-pos/api/internal/handler"
	mw "github.com/kiwari-pos/api/internal/middleware"
	"github.com/kiwari-pos/api/internal/service"
	"github.com/kiwari-pos/api/internal/ws"
)

// New creates a Chi router with all application routes wired up.
// Applies authentication, outlet scoping, and role-based middleware as needed.
func New(cfg *config.Config, queries *database.Queries, pool *pgxpool.Pool, hub *ws.Hub) chi.Router {
	r := chi.NewRouter()

	// Standard middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",                  // SvelteKit dev server
			"https://admin.nasibakarkiwari.com",      // Production admin
			"https://stg-admin.nasibakarkiwari.com",  // Staging admin
			"https://pos.nasibakarkiwari.com",        // Legacy admin (remove after migration)
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	}))

	// Public routes
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","version":"1.0.0"}`))
	})

	// Auth routes (public)
	authHandler := handler.NewAuthHandler(queries, cfg.JWTSecret)
	authHandler.RegisterRoutes(r)

	// WebSocket route (handles auth internally via query param)
	r.Get("/ws/outlets/{oid}/orders", func(w http.ResponseWriter, r *http.Request) {
		ws.ServeWS(hub, cfg.JWTSecret, w, r)
	})

	// Protected routes (require authentication)
	r.Group(func(r chi.Router) {
		r.Use(mw.Authenticate(cfg.JWTSecret))

		// Owner-only routes (not outlet-scoped)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireRole("OWNER"))
			reportsHandler := handler.NewReportsHandler(queries)
			r.Route("/reports", reportsHandler.RegisterOwnerRoutes)
		})

		// Accounting routes (OWNER only, not outlet-scoped)
		r.Group(func(r chi.Router) {
			r.Use(mw.RequireRole("OWNER"))

			// Master data
			masterHandler := accthandler.NewMasterHandler(queries, queries, queries)
			r.Route("/accounting/master/accounts", masterHandler.RegisterAccountRoutes)
			r.Route("/accounting/master/items", masterHandler.RegisterItemRoutes)
			r.Route("/accounting/master/cash-accounts", masterHandler.RegisterCashAccountRoutes)

			// Purchases
			purchaseHandler := accthandler.NewPurchaseHandler(queries)
			r.Route("/accounting/purchases", purchaseHandler.RegisterRoutes)
		})

		// Outlet-scoped routes
		r.Route("/outlets/{oid}", func(r chi.Router) {
			r.Use(mw.RequireOutlet)

			// Users
			userHandler := handler.NewUserHandler(queries)
			r.Route("/users", userHandler.RegisterRoutes)

			// Categories
			categoryHandler := handler.NewCategoryHandler(queries)
			r.Route("/categories", categoryHandler.RegisterRoutes)

			// Products
			productHandler := handler.NewProductHandler(queries)
			r.Route("/products", func(r chi.Router) {
				productHandler.RegisterRoutes(r)

				// Nested product routes (variants, modifiers, combos)
				r.Route("/{pid}", func(r chi.Router) {
					variantHandler := handler.NewVariantHandler(queries)
					variantHandler.RegisterRoutes(r)

					modifierHandler := handler.NewModifierHandler(queries)
					modifierHandler.RegisterRoutes(r)

					comboHandler := handler.NewComboHandler(queries)
					comboHandler.RegisterRoutes(r)
				})
			})

			// Orders
			newOrderStore := func(db database.DBTX) service.OrderStore {
				return database.New(db)
			}
			orderService := service.NewOrderService(pool, newOrderStore)
			orderHandler := handler.NewOrderHandler(
				orderService,
				queries,
				pool,
				func(db database.DBTX) handler.OrderStore {
					return database.New(db)
				},
			)
			r.Route("/orders", func(r chi.Router) {
				orderHandler.RegisterRoutes(r)

				// Payments (nested under orders)
				r.Route("/{id}/payments", func(r chi.Router) {
					paymentHandler := handler.NewPaymentHandler(
						queries,
						pool,
						func(db database.DBTX) handler.PaymentStore {
							return database.New(db)
						},
					)
					paymentHandler.RegisterRoutes(r)
				})
			})

			// Customers
			customerHandler := handler.NewCustomerHandler(queries)
			r.Route("/customers", customerHandler.RegisterRoutes)

			// Reports (outlet-scoped)
			reportsHandler := handler.NewReportsHandler(queries)
			r.Route("/reports", reportsHandler.RegisterRoutes)
		})
	})

	log.Println("Router initialized with all handlers")
	return r
}
