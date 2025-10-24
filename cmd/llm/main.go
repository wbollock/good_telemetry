// ABOUTME: LLM backend service - wraps Ollama and provides metric evaluation API
// ABOUTME: Handles RAG queries and generates structured analysis responses

package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	// TODO: Initialize Ollama client
	// TODO: Load RAG knowledge base
	// TODO: Set up routes
	// TODO: Load configuration

	http.HandleFunc("/evaluate", handleEvaluate)

	port := ":8081"
	log.Printf("Starting Good Telemetry LLM backend on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}

func handleEvaluate(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "LLM Evaluation Service - Coming Soon")
}
