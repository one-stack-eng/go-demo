package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type store struct {
	mu    sync.Mutex
	next  int
	notes map[string]string
}

func (n *store) getNoteView(w http.ResponseWriter, r *http.Request) {
	n.mu.Lock()
	ids := make([]string, 0, len(n.notes))
	for id := range n.notes {
		ids = append(ids, id)
	}
	n.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, id := range ids {
		fmt.Fprintln(w, id)
	}
}

func (n *store) createNote(w http.ResponseWriter, r *http.Request) {
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

	n.mu.Lock()
	id := strconv.Itoa(n.next)
	n.next++
	n.notes[id] = string(body)
	n.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, id)
}

func (n *store) deleteNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	n.mu.Lock()
	_, ok := n.notes[id]
	if ok {
		delete(n.notes, id)
	}
	n.mu.Unlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (n *store) getNoteByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	n.mu.Lock()
	body, ok := n.notes[id]
	n.mu.Unlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body))
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

	n := &store{
		next:  1,
		notes: make(map[string]string)}
	router.Get("/notes", n.getNoteView)
	router.Get("/notes/{id}", n.getNoteByID)
	router.Post("/notes", n.createNote)
	router.Delete("/notes/{id}", n.deleteNote)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
