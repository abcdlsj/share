package main

import (
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", httpRequestHandler)
	http.ListenAndServeTLS(":8010", "k.crt", "k.key", nil)
}

func httpRequestHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}
