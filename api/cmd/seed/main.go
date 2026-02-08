package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// CLI flags
	email := flag.String("email", "", "Owner email address")
	password := flag.String("password", "", "Owner password")
	name := flag.String("name", "", "Owner full name")
	flag.Parse()

	// Fall back to environment variables
	if *email == "" {
		*email = os.Getenv("SEED_EMAIL")
	}
	if *password == "" {
		*password = os.Getenv("SEED_PASSWORD")
	}
	if *name == "" {
		*name = os.Getenv("SEED_NAME")
	}

	// Fall back to defaults
	if *email == "" {
		*email = "admin@kiwari.com"
	}
	if *password == "" {
		*password = "password123"
		log.Println("WARNING: Using default password 'password123'. Change immediately in production!")
	}
	if *name == "" {
		*name = "Admin Kiwari"
	}

	// Load database URL from environment
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://pos:pos@localhost:5432/pos_db?sslmode=disable"
	}

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer pool.Close()

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Seed in a transaction (atomicity: both outlet + user or neither)
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("Failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	outletID, err := seedOutlet(ctx, tx)
	if err != nil {
		log.Fatalf("Failed to seed outlet: %v", err)
	}

	userID, err := seedOwner(ctx, tx, outletID, *email, *password, *name)
	if err != nil {
		log.Fatalf("Failed to seed owner: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Failed to commit: %v", err)
	}

	log.Println("Seed completed successfully")
	log.Printf("Outlet ID: %s", outletID)
	log.Printf("Owner ID: %s", userID)
}

// seedOutlet creates the initial outlet if it doesn't exist.
func seedOutlet(ctx context.Context, tx pgx.Tx) (uuid.UUID, error) {
	const (
		outletName    = "Kiwari Nasi Bakar"
		outletAddress = "Jl. Contoh No. 1, Jakarta"
		outletPhone   = "081234567890"
	)

	// Check if outlet already exists
	var existingID uuid.UUID
	checkSQL := `SELECT id FROM outlets WHERE name = $1 AND is_active = true LIMIT 1`
	err := tx.QueryRow(ctx, checkSQL, outletName).Scan(&existingID)
	if err == nil {
		log.Printf("Outlet '%s' already exists (ID: %s), skipping", outletName, existingID)
		return existingID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, fmt.Errorf("check outlet: %w", err)
	}

	// Create outlet
	insertSQL := `
		INSERT INTO outlets (name, address, phone, is_active)
		VALUES ($1, $2, $3, true)
		RETURNING id
	`
	var newID uuid.UUID
	err = tx.QueryRow(ctx, insertSQL, outletName, outletAddress, outletPhone).Scan(&newID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert outlet: %w", err)
	}

	log.Printf("Created outlet '%s' (ID: %s)", outletName, newID)
	return newID, nil
}

// seedOwner creates the owner user if it doesn't exist.
func seedOwner(ctx context.Context, tx pgx.Tx, outletID uuid.UUID, email, password, fullName string) (uuid.UUID, error) {
	// Check if user already exists
	var existingID uuid.UUID
	checkSQL := `SELECT id FROM users WHERE email = $1 LIMIT 1`
	err := tx.QueryRow(ctx, checkSQL, email).Scan(&existingID)
	if err == nil {
		log.Printf("User '%s' already exists (ID: %s), skipping", email, existingID)
		return existingID, nil
	}
	if err != pgx.ErrNoRows {
		return uuid.Nil, fmt.Errorf("check user: %w", err)
	}

	// Hash password
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, fmt.Errorf("hash password: %w", err)
	}

	// Create user
	insertSQL := `
		INSERT INTO users (outlet_id, email, hashed_password, full_name, role, is_active)
		VALUES ($1, $2, $3, $4, 'OWNER', true)
		RETURNING id
	`
	var newID uuid.UUID
	err = tx.QueryRow(ctx, insertSQL, outletID, email, string(hashed), fullName).Scan(&newID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert user: %w", err)
	}

	log.Printf("Created owner user '%s' (ID: %s)", email, newID)
	return newID, nil
}
