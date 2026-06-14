package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/api"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env("DB_HOST", "127.0.0.1"),
		env("DB_PORT", "5432"),
		env("DB_USER", "essensys"),
		env("DB_PASSWORD", ""),
		env("DB_NAME", "essensys_db"),
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	store := data.NewPortalStore(db)
	migrationPath := env("MIGRATIONS_PATH", "migrations/001_init.sql")
	if err := store.RunMigrations(migrationPath); err != nil {
		log.Printf("WARNING: migrations: %v", err)
	}

	router := api.NewRouter(store)
	port := env("PORT", "8081")
	log.Printf("essensys-user-portal-backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
