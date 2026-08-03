// stack-demo: a tiny web + worker stack that reads a managed Postgres via the
// DATABASE_URL injected by render.yaml's fromDatabase reference (w1/m24). The same
// image serves both roles: ROLE=web (the default) runs the HTTP server, which
// answers GET / with a real SELECT 1 through DATABASE_URL; ROLE=worker runs a
// heartbeat loop pinging the same database. bex background workers run the
// image's CMD with no override, so the role is selected by env, not argv.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	switch os.Getenv("ROLE") {
	case "worker":
		worker()
	default:
		web()
	}
}

// db opens the managed-Postgres connection bex injected from the database's CNPG
// connection Secret (DATABASE_URL is a secretRef — its value never appears in
// render.yaml or the App spec).
func db() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL not set (fromDatabase not resolved)")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	return db, nil
}

func web() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		db, err := db()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		defer db.Close()
		var one int
		if err := db.QueryRowContext(r.Context(), "SELECT 1").Scan(&one); err != nil {
			http.Error(w, "db query failed: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, "db ok: SELECT 1 = %d\n", one)
	})
	addr := ":" + getenv("PORT", "3000")
	log.Printf("stack-demo web listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func worker() {
	db, err := db()
	if err != nil {
		log.Fatalf("worker: %v", err)
	}
	defer db.Close()
	for i := 1; ; i++ {
		var one int
		if err := db.QueryRowContext(context.Background(), "SELECT 1").Scan(&one); err != nil {
			log.Printf("worker #%d: db ping failed: %v", i, err)
		} else {
			log.Printf("worker #%d: db ok (SELECT 1 = %d)", i, one)
		}
		time.Sleep(10 * time.Second)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
