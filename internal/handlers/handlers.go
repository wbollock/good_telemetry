// ABOUTME: HTTP handlers for Good Telemetry web routes
// ABOUTME: Processes metric evaluation requests and serves UI responses

package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wbollock/good_telemetry/internal/llm"
	"github.com/wbollock/good_telemetry/internal/metrics"
)

type Handler struct {
	llmClient *llm.Client
}

func NewHandler(llmClient *llm.Client) *Handler {
	return &Handler{
		llmClient: llmClient,
	}
}

func (h *Handler) Index(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Good Telemetry",
	})
}

func (h *Handler) Evaluate(c *gin.Context) {
	log.Println("[Evaluate] Received evaluation request")

	var req struct {
		Metrics string `form:"metrics" binding:"required"`
	}

	if err := c.ShouldBind(&req); err != nil {
		log.Printf("[Evaluate] Error binding request: %v", err)
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": "Please provide metrics to evaluate",
		})
		return
	}

	log.Printf("[Evaluate] Input metrics:\n%s", req.Metrics)

	// Parse metrics
	parsed, err := metrics.Parse(req.Metrics)
	if err != nil {
		log.Printf("[Evaluate] Error parsing metrics: %v", err)
		c.HTML(http.StatusBadRequest, "error.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	log.Printf("[Evaluate] Parsed %d metric(s), sending to LLM...", len(parsed.Metrics))

	// Evaluate with LLM
	evaluation, err := h.llmClient.Evaluate(parsed)
	if err != nil {
		log.Printf("[Evaluate] Error calling LLM: %v", err)
		c.HTML(http.StatusInternalServerError, "error.html", gin.H{
			"error": "Failed to evaluate metrics: " + err.Error(),
		})
		return
	}

	log.Printf("[Evaluate] LLM evaluation complete. Verdict: %s", evaluation.Verdict)

	// Return evaluation result (htmx will swap this into the page)
	c.HTML(http.StatusOK, "result.html", gin.H{
		"evaluation": evaluation,
		"metrics":    parsed,
	})
}

func (h *Handler) Examples(c *gin.Context) {
	examples := getHardcodedExamples()
	c.HTML(http.StatusOK, "examples.html", gin.H{
		"examples": examples,
	})
}

// Hardcoded showcase examples
func getHardcodedExamples() []Example {
	return []Example{
		{
			Metrics: `http_requests_total{method="GET", handler="/api/users", status="200"} 1027`,
			Verdict: "Good",
			Issues:  []string{},
			Recommendations: []string{
				"This is a well-structured counter metric",
				"Uses appropriate _total suffix",
				"Labels are low-cardinality and meaningful",
			},
			CardinalityEstimate: "Low (3 methods × ~10 handlers × 5 status codes = ~150 series)",
			MemoryEstimate:      "~3KB RAM per series = ~450KB total",
		},
		{
			Metrics: `api_response_time{user_id="12345", endpoint="/profile"} 0.234`,
			Verdict: "Needs Improvement",
			Issues: []string{
				"user_id is unbounded high-cardinality label",
				"Missing _seconds suffix for time measurement",
				"Should be a histogram, not gauge",
			},
			Recommendations: []string{
				"Remove user_id label - use it in logs instead",
				"Rename to api_response_duration_seconds",
				"Convert to histogram for percentile calculations",
			},
			CardinalityEstimate: "CRITICAL: Unbounded (1 series per user × endpoints = potentially millions)",
			MemoryEstimate:      "Could easily exceed 10GB+ with 100k users",
		},
		{
			Metrics: `cache_hit_ratio 0.87`,
			Verdict: "Needs Improvement",
			Issues: []string{
				"Ratio should be calculated in queries, not stored as metric",
				"Missing labels to identify which cache",
			},
			Recommendations: []string{
				"Store cache_hits_total and cache_misses_total instead",
				"Add cache_name label",
				"Calculate ratio: cache_hits_total / (cache_hits_total + cache_misses_total)",
			},
			CardinalityEstimate: "N/A - antipattern",
			MemoryEstimate:      "N/A",
		},
		{
			Metrics: `volume_attachment{vol="vol-abc123", inode="1048576", timestamp="1729783200", cluster="prod-east"} 1`,
			Verdict: "Poor",
			Issues: []string{
				"vol label creates series per volume (2566+ unique values)",
				"inode label is extremely high-cardinality (529+ unique values)",
				"timestamp as label is a cardinal sin - creates infinite series",
				"Combines multiple unbounded labels = cardinality explosion",
			},
			Recommendations: []string{
				"Remove vol label - aggregate at pool/cluster level instead",
				"Remove inode completely - use logs for per-inode tracking",
				"NEVER use timestamp as a label - Prometheus already timestamps samples",
				"Keep only cluster/pool labels for aggregation",
				"Real example: 2566 vols × 529 inodes × 1606 timestamps = 2.18 BILLION series",
			},
			CardinalityEstimate: "CATASTROPHIC: 2.18+ billion potential series (2566 vol × 529 inode × 1606 timestamp)",
			MemoryEstimate:      "6.5+ TB RAM required (likely to crash Prometheus entirely)",
		},
	}
}

type Example struct {
	Metrics             string
	Verdict             string
	Issues              []string
	Recommendations     []string
	CardinalityEstimate string
	MemoryEstimate      string
}
