package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

type Item struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck())
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		env("DB_HOST", "db"), env("DB_PORT", "5432"), env("DB_USER", "app"),
		env("DB_PASSWORD", ""), env("DB_NAME", "appdb"))

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := waitForDB(db, 30, 2*time.Second); err != nil {
		log.Fatalf("db not reachable: %v", err)
	}
	log.Println("db connection established")

	staticDir := env("STATIC_DIR", "/app/static")
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.QueryContext(r.Context(), `SELECT id, name FROM items ORDER BY id`)
		if err != nil {
			log.Printf("query: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		items := []Item{}
		for rows.Next() {
			var it Item
			if err := rows.Scan(&it.ID, &it.Name); err != nil {
				log.Printf("scan: %v", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			items = append(items, it)
		}
		if err := rows.Err(); err != nil {
			log.Printf("rows: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(items)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "db unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})

	mux.Handle("/", http.FileServer(http.Dir(staticDir)))

	srv := &http.Server{
		Addr:              ":" + env("HTTP_PORT", "8080"),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s, static from %s", srv.Addr, staticDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func waitForDB(db *sql.DB, attempts int, delay time.Duration) error {
	var err error
	for i := 1; i <= attempts; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err == nil {
			return nil
		}
		log.Printf("waiting for db (%d/%d): %v", i, attempts, err)
		time.Sleep(delay)
	}
	return err
}

func healthcheck() int {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + env("HTTP_PORT", "8080") + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
