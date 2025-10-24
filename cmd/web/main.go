// ABOUTME: Web server for Good Telemetry - serves UI and handles metric evaluation requests
// ABOUTME: Communicates with LLM backend for analysis and stores showcase examples

package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// TODO: Initialize database
	// TODO: Initialize LLM client
	// TODO: Set up routes
	// TODO: Load configuration

	http.HandleFunc("/", handleIndex)

	port := ":8080"
	log.Printf("Starting Good Telemetry web server on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Good Telemetry - Coming Soon")
}
