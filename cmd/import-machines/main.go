package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/essensys-hub/essensys-user-portal-backend/internal/data"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type persistenceData struct {
	Machines map[string]*importMachine `json:"machines"`
	Details  map[string]*importDetail  `json:"details"`
}

type importMachine struct {
	NoSerie   string `json:"no_serie"`
	IsActive  bool   `json:"is_active"`
	HashedPkey string `json:"hashed_pkey"`
}

type importDetail struct {
	NoSerie    string    `json:"no_serie"`
	IP         string    `json:"ip"`
	LastSeen   time.Time `json:"last_seen"`
	RawAuth    string    `json:"raw_auth"`
	RawDecoded string    `json:"raw_decoded"`
}

func main() {
	input := flag.String("input", "machines.json", "path to support-site machines.json")
	dsn := flag.String("dsn", "", "postgres DSN (or use DB_* env vars)")
	flag.Parse()

	if *dsn == "" {
		*dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			env("DB_HOST", "127.0.0.1"),
			env("DB_PORT", "5432"),
			env("DB_USER", "essensys"),
			env("DB_PASSWORD", ""),
			env("DB_NAME", "essensys_db"),
		)
	}

	f, err := os.Open(*input)
	if err != nil {
		log.Fatalf("open input: %v", err)
	}
	defer f.Close()

	var pd persistenceData
	if err := json.NewDecoder(f).Decode(&pd); err != nil {
		log.Fatalf("decode json: %v", err)
	}

	db, err := sqlx.Connect("postgres", *dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	store := data.NewLegacyIoTStore(db)

	imported := 0
	for hash, m := range pd.Machines {
		if hash == "" {
			continue
		}
		detail := pd.Details[hash]
		clientID := m.NoSerie
		ip := ""
		lastSeen := time.Time{}
		var authJSON json.RawMessage
		if detail != nil {
			if detail.NoSerie != "" {
				clientID = detail.NoSerie
			}
			ip = detail.IP
			lastSeen = detail.LastSeen
			authJSON, _ = json.Marshal(map[string]string{
				"raw_auth":    detail.RawAuth,
				"raw_decoded": detail.RawDecoded,
			})
		}
		if clientID == "" {
			clientID = m.NoSerie
		}
		if err := store.ImportMachine(hash, clientID, ip, m.IsActive, lastSeen, authJSON); err != nil {
			log.Printf("skip %s: %v", hash[:min(8, len(hash))], err)
			continue
		}
		imported++
	}
	log.Printf("Imported %d machine(s) from %s", imported, *input)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
