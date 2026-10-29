package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type store struct {
	mu    sync.Mutex
	next  int
	notes map[string]string
}

type noteViewer struct{ notes *store }

func (n *noteViewer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	n.notes.mu.Lock()
	ids := make([]string, 0, len(n.notes.notes))
	for id := range n.notes.notes {
		ids = append(ids, id)
	}
	n.notes.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	for _, id := range ids {
		fmt.Fprintln(w, id)
	}
}

type noteCreator struct{ notes *store }

func (n *noteCreator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	n.notes.mu.Lock()
	id := strconv.Itoa(n.notes.next)
	n.notes.next++
	n.notes.notes[id] = string(body)
	n.notes.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, id)
}

type noteDeleter struct{ notes *store }

func (n *noteDeleter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	n.notes.mu.Lock()
	_, ok := n.notes.notes[id]
	if ok {
		delete(n.notes.notes, id)
	}
	n.notes.mu.Unlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type noteViewerByID struct{ notes *store }

func (n *noteViewerByID) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	n.notes.mu.Lock()
	body, ok := n.notes.notes[id]
	n.notes.mu.Unlock()

	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(body))
}

func main() {
	mux := http.NewServeMux()
	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20}

	noteMap := &store{
		next:  1,
		notes: make(map[string]string)}

	nv := &noteViewer{notes: noteMap}
	nvid := &noteViewerByID{notes: noteMap}
	nc := &noteCreator{notes: noteMap}
	nd := &noteDeleter{notes: noteMap}

	mux.Handle("GET /notes", nv)
	mux.Handle("GET /notes/{id}", nvid)
	mux.Handle("POST /notes", nc)
	mux.Handle("DELETE /notes/{id}", nd)

	server.ListenAndServe()
}
