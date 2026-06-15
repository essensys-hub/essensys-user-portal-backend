package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/api"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/config"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/essensys-hub/essensys-user-portal-backend/internal/observability"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	store := data.NewPortalStore(db)
	users := data.NewUserStore(db)
	audit := data.NewAuditStore(db)
	inventory := data.NewAdminInventoryStore(db)
	news := data.NewNewsletterStore(db)
	iot := data.NewLegacyIoTStore(db)
	paths, err := sortedSQLMigrations(cfg.MigrationsDir)
	if err != nil {
		log.Printf("WARNING: list migrations: %v", err)
	} else if len(paths) > 0 {
		if err := store.RunMigrations(paths...); err != nil {
			log.Printf("WARNING: migrations: %v", err)
		} else {
			log.Printf("Applied %d migration(s)", len(paths))
		}
	}

	if cfg.ConsolidatedMode {
		if err := users.EnsureTableExists(); err != nil {
			log.Printf("WARNING: users table: %v", err)
		}
		if err := audit.EnsureTableExists(); err != nil {
			log.Printf("WARNING: audit table: %v", err)
		}
		if err := news.EnsureTablesExist(); err != nil {
			log.Printf("WARNING: newsletter tables: %v", err)
		}
		log.Println("CONSOLIDATED_MODE=true — identity/admin/legacyiot routes active")
		go iot.BackfillMissingMachineGeo()
	}

	nrApp := observability.InitNewRelic()
	router := api.NewRouter(store, users, audit, inventory, news, iot, nrApp, cfg)
	log.Printf("essensys-user-portal-backend listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
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
