package main

import (
	"context"
	"log"
	"os"

	"go.temporal.io/sdk/client"

	"payment-processing/internal/repository"
	"payment-processing/pkg/domain"
	"payment-processing/worker"
)

func main() {
	ctx := context.Background()

	// Load configuration
	temporalHost := getEnv("TEMPORAL_HOST", "localhost:7233")

	// Create Temporal client
	temporalClient, err := client.Dial(client.Options{
		HostPort: temporalHost,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Create database connection
	var db *repository.DB
	dbConfig := loadDBConfig()
	if dbConfig != nil {
		db, err = repository.NewDB(ctx, *dbConfig)
		if err != nil {
			log.Printf("Warning: Failed to connect to database: %v", err)
			log.Printf("Running without database - some features will be unavailable")
		} else {
			defer db.Close()

			// Run migrations
			if err := db.RunMigrations(*dbConfig); err != nil {
				log.Printf("Warning: Failed to run migrations: %v", err)
			}
		}
	}

	// Get repositories if database is available
	var repos *domain.Repositories
	if db != nil {
		repos = db.Repositories()
	}

	// Start worker (blocking)
	log.Printf("Starting Payment Worker")
	if err := worker.StartWorkerWithDependencies(temporalClient, repos); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

// loadDBConfig loads database configuration from environment
func loadDBConfig() *repository.Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		// Parse DATABASE_URL format would go here
	}

	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		return nil
	}

	cfg := repository.DefaultConfig()
	cfg.Host = host
	if user != "" {
		cfg.User = user
	}
	if pass := os.Getenv("DB_PASSWORD"); pass != "" {
		cfg.Password = pass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.Database = dbName
	}
	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		cfg.SSLMode = sslMode
	}

	return &cfg
}

// getEnv gets environment variable with a default value
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
