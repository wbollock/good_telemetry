// ABOUTME: Web server for Good Telemetry - serves UI and handles metric evaluation requests
// ABOUTME: Communicates with LLM backend for analysis and displays hardcoded showcase examples

package main

import (
	"html/template"
	"log"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wbollock/good_telemetry/internal/handlers"
	"github.com/wbollock/good_telemetry/internal/llm"
)

func main() {
	// Load configuration from environment
	llmURL := os.Getenv("LLM_BACKEND_URL")
	if llmURL == "" {
		llmURL = "http://localhost:11434" // Default Ollama local URL
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "llama2"
	}

	port := os.Getenv("WEB_PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize LLM client
	llmClient := llm.NewClient(llmURL, model)

	// Set up gin router
	r := gin.Default()

	// Register custom template functions
	r.SetFuncMap(template.FuncMap{
		"lower": strings.ToLower,
	})

	// Load HTML templates
	r.LoadHTMLGlob("web/templates/*")
	r.Static("/static", "./web/static")

	// Initialize handlers
	h := handlers.NewHandler(llmClient)

	// Routes
	r.GET("/", h.Index)
	r.POST("/evaluate", h.Evaluate)
	r.GET("/examples", h.Examples)

	log.Printf("Starting Good Telemetry web server on :%s", port)
	log.Printf("LLM Backend: %s (model: %s)", llmURL, model)

	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}
