package main

import (
	"fmt"
	"net/http"
)

type handlerAPI struct{}

func (h *handlerAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "this is home")
}

func main() {
	mux := http.NewServeMux()
	h := handlerAPI{}

	mux.Handle("/", &h)

	http.ListenAndServe(":8080", mux)
}
