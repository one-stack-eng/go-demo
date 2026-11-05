package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Data struct {
	ID int64 `db:"id"`
}

func getNoteView(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query(r.Context(), `SELECT id FROM notes ORDER BY id`)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		dataSlice, err := pgx.CollectRows(rows, pgx.RowToStructByName[Data])
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		for _, id := range dataSlice {
			fmt.Fprintln(w, id.ID)
		}
	}
}

func createNote(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		var id int64
		if err := db.QueryRow(r.Context(), `INSERT INTO notes (body) VALUES ($1) RETURNING id`, body).Scan(&id); err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprintln(w, id)
	}
}

func deleteNote(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		tag, err := db.Exec(r.Context(), `DELETE FROM notes WHERE id = $1`, id)
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func getNoteByID(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		var body []byte
		err = db.QueryRow(r.Context(), `SELECT body FROM notes WHERE id = $1`, id).Scan(&body)
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(body)
	}
}

func main() {
	router := chi.NewRouter()
	router.Use(middleware.Logger)
	server := &http.Server{
		Addr:              ":8080",
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	dsn := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(ctx, dsn)
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	if err != nil {
		log.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS notes (
		id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
		body BYTEA NOT NULL
	)`); err != nil {
		log.Fatal(err)
	}
	cancel()

	router.Get("/notes", getNoteView(pool))
	router.Get("/notes/{id}", getNoteByID(pool))
	router.Post("/notes", createNote(pool))
	router.Delete("/notes/{id}", deleteNote(pool))

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
