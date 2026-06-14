package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"

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
	migrationDir := env("MIGRATIONS_DIR", "migrations")
	paths, err := sortedSQLMigrations(migrationDir)
	if err != nil {
		log.Printf("WARNING: list migrations: %v", err)
	} else if len(paths) > 0 {
		if err := store.RunMigrations(paths...); err != nil {
			log.Printf("WARNING: migrations: %v", err)
		} else {
			log.Printf("Applied %d migration(s)", len(paths))
		}
	}

	router := api.NewRouter(store)
	port := env("PORT", "8081")
	log.Printf("essensys-user-portal-backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal(err)
	}
}

func sortedSQLMigrations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".sql" {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
