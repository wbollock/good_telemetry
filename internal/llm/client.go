// ABOUTME: LLM client for communicating with Ollama backend
// ABOUTME: Handles prompt construction and response parsing

package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/wbollock/good_telemetry/internal/cardinality"
	"github.com/wbollock/good_telemetry/internal/metrics"
)

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

type Evaluation struct {
	Verdict             string
	OverallScore        string
	Issues              []string
	Recommendations     []string
	ImprovedExample     string
	CardinalityAnalysis string
	MemoryImpact        string
	RawResponse         string
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// ============================================================================
// EVALUATION PROMPT - Edit this to change how the LLM evaluates metrics
// ============================================================================
const systemPrompt = `You are a Prometheus metrics expert following official Prometheus best practices.

EXAMPLES OF GOOD METRICS (These should be rated "Good"):
✓ http_requests_total{method="GET", status="200", endpoint="/api/users"} 15847
✓ node_memory_usage_bytes{instance="web-01", region="us-east-1"} 8589934592
✓ http_request_duration_seconds_bucket{le="0.1", method="POST", status="201"} 9543
✓ process_cpu_seconds_total{instance="api-3", cluster="prod"} 12847.23

GOOD LABEL EXAMPLES (SAFE to use, even in combination):
✓ method (GET, POST, PUT, DELETE) - ~10 values - ALWAYS SAFE
✓ status (200, 404, 500) - ~20 values - ALWAYS SAFE
✓ endpoint (/api/users, /api/posts, /handlers/*) - typically 10-100 values - ALWAYS SAFE FOR WEB APPS
✓ handler, route, path (when normalized/templated) - ALWAYS SAFE
✓ region, zone, cluster - Infrastructure labels - ALWAYS SAFE
✓ instance, job - Standard Prometheus labels - ALWAYS SAFE

COMBINING GOOD LABELS IS FINE:
- 10 methods × 20 statuses × 100 endpoints = 20,000 series (perfectly acceptable)
- endpoint/handler labels with 10-100 values are CRITICAL for web application observability
- Problems only occur with UNBOUNDED labels like user_id, timestamp, etc.

OFFICIAL PROMETHEUS NAMING CONVENTIONS:

1. METRIC NAMING:
   - Use snake_case (e.g., http_requests_total, not httpRequestsTotal)
   - Names should describe WHAT is being measured, not HOW
   - Use base units: seconds (not milliseconds), bytes (not megabytes), etc.
   - Metric names should have a suffix describing the unit (where applicable)
     * _total for counters (monotonically increasing values)
     * _seconds for durations
     * _bytes for sizes
     * _ratio for ratios (0-1)
     * _percent for percentages (0-100)
   - Avoid putting the metric type in the name (no "gauge_", "counter_" prefixes)

2. LABEL NAMING:
   - Use snake_case for label names
   - Labels are key-value pairs for dimensions of a metric
   - EVERY unique combination of labels creates a NEW TIME SERIES

3. CARDINALITY RULES (CRITICAL):
   - High-cardinality labels create MILLIONS of time series and crash Prometheus
   - NEVER use these UNBOUNDED labels:
     * user_id, email, username (unbounded, one per user)
     * ip_address, client_ip (one per client)
     * timestamp, epoch, unix_time, created_at (infinite values)
     * uuid, guid, trace_id, span_id (unbounded identifiers)
     * session_id, request_id (unbounded per request)
     * url_path, full_path (unbounded URLs)
     * inode, file_id (unbounded per file)
     * volume_id, disk_id (potentially unbounded)
   - Put high-cardinality data in LOGS, not metrics

4. METRIC TYPES:
   - Counter: Cumulative metric that only increases (requests_total, errors_total)
   - Gauge: Value that can go up or down (memory_usage_bytes, queue_length)
   - Histogram: Observations in buckets (request_duration_seconds)
   - Summary: Like histogram but with quantiles

5. COMMON ANTIPATTERNS:
   - Storing ratios/percentages as metrics (calculate in queries instead)
   - Using milliseconds instead of seconds for time
   - Combining multiple UNBOUNDED labels (multiplication effect causes cardinality explosion)
   - Missing _total suffix on counters
   - Using camelCase or UPPERCASE`

const evaluationInstructions = `
IMPORTANT - DO NOT flag these as issues:
- Missing # TYPE or # HELP comments (not required for evaluation)
- Missing "instance" or "job" labels (added automatically by Prometheus during scraping)
- Single sample cardinality estimation (expected - users typically submit one metric)
- Missing metric value (values are optional in the exposition format)
- endpoint/handler/route labels (these are SAFE and CRITICAL for web apps)
- method/status labels (these are ALWAYS SAFE)

Focus ONLY on actual problems:
- Naming issues (camelCase, wrong suffixes, wrong units)
- High-cardinality labels (user_id, timestamp, email, ip_address, session_id, etc.)
- Label naming issues (spaces, camelCase, etc.)

When providing IMPROVED EXAMPLE:
- Keep good elements from the original (don't break what works)
- KEEP _total suffix on counters (required by Prometheus conventions)
- KEEP bounded labels like method, status, endpoint (these are correct)
- Use concise names (e.g., http_requests_total, NOT requests_sent_by_get_request)
- Only change what's actually broken

Provide your evaluation in this EXACT format:

VERDICT: [Good/Needs Improvement/Poor]
ISSUES:
- [list specific issues, one per line]
RECOMMENDATIONS:
- [list specific recommendations, one per line]
IMPROVED EXAMPLE:
[show corrected metric with proper naming and labels]`

// ============================================================================

func NewClient(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *Client) Evaluate(parsed *metrics.ParsedMetrics) (*Evaluation, error) {
	log.Printf("[LLM] Starting evaluation with model %s at %s", c.model, c.baseURL)

	// Build the prompt
	prompt := c.buildPrompt(parsed)
	log.Printf("[LLM] Built prompt (%d chars):\n%s\n---END PROMPT---", len(prompt), prompt)

	// Call Ollama API
	reqBody := ollamaRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		log.Printf("[LLM] Error marshaling request: %v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := c.baseURL + "/api/generate"
	log.Printf("[LLM] Calling Ollama API: POST %s", apiURL)

	start := time.Now()
	resp, err := c.httpClient.Post(
		apiURL,
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		log.Printf("[LLM] Error calling Ollama API: %v", err)
		return nil, fmt.Errorf("failed to call Ollama API: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[LLM] Got response status: %d (took %v)", resp.StatusCode, time.Since(start))

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LLM] Non-OK status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("Ollama API returned status %d", resp.StatusCode)
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		log.Printf("[LLM] Error decoding response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[LLM] Received response (%d chars):\n%s\n---END RESPONSE---",
		len(ollamaResp.Response), ollamaResp.Response)

	// Parse the LLM response into structured evaluation
	evaluation := c.parseResponse(ollamaResp.Response, parsed.CardinalityAnalysis)
	log.Printf("[LLM] Parsed evaluation: Verdict=%s, Issues=%d, Recommendations=%d",
		evaluation.Verdict, len(evaluation.Issues), len(evaluation.Recommendations))

	return evaluation, nil
}

func (c *Client) buildPrompt(parsed *metrics.ParsedMetrics) string {
	var sb strings.Builder

	// System prompt with Prometheus best practices
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n")

	// TODO(RAG): Add retrieval-augmented generation here
	// Before evaluating, search docs/ directory for:
	// 1. Similar good metric examples from real production systems
	// 2. Relevant Prometheus documentation sections
	// 3. Company/team-specific metric naming conventions
	// 4. Common patterns for this metric type (counter/gauge/histogram)
	// Use vector embeddings to find most relevant examples and append to prompt
	// This will give LLM concrete examples to learn from instead of generic rules

	// User's metrics
	sb.WriteString("METRICS TO EVALUATE:\n")
	for _, m := range parsed.Metrics {
		sb.WriteString(fmt.Sprintf("%s\n", m.Raw))
	}
	sb.WriteString("\n")

	// Cardinality analysis from our calculator
	if parsed.CardinalityAnalysis != nil {
		sb.WriteString("CARDINALITY ANALYSIS:\n")
		sb.WriteString(fmt.Sprintf("Estimated Series: %d\n", parsed.CardinalityAnalysis.EstimatedSeries))
		sb.WriteString(fmt.Sprintf("Memory Estimate: %s\n", parsed.CardinalityAnalysis.MemoryEstimateHuman))
		sb.WriteString(fmt.Sprintf("Cardinality Level: %s\n", parsed.CardinalityAnalysis.CardinalityLevel))
		if len(parsed.CardinalityAnalysis.HighCardinalityRisks) > 0 {
			sb.WriteString("HIGH CARDINALITY RISKS:\n")
			for _, risk := range parsed.CardinalityAnalysis.HighCardinalityRisks {
				sb.WriteString(fmt.Sprintf("- %s\n", risk))
			}
		}
		sb.WriteString("\n")
	}

	// Output format instructions
	sb.WriteString(evaluationInstructions)

	return sb.String()
}

func (c *Client) parseResponse(response string, cardAnalysis *cardinality.Analysis) *Evaluation {
	eval := &Evaluation{
		RawResponse: response,
	}

	lines := strings.Split(response, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "VERDICT:") {
			eval.Verdict = strings.TrimSpace(strings.TrimPrefix(line, "VERDICT:"))
			continue
		}

		if strings.HasPrefix(line, "ISSUES:") {
			currentSection = "issues"
			continue
		}

		if strings.HasPrefix(line, "RECOMMENDATIONS:") {
			currentSection = "recommendations"
			continue
		}

		if strings.HasPrefix(line, "IMPROVED EXAMPLE:") {
			currentSection = "example"
			continue
		}

		// Parse bullet points
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			item := strings.TrimPrefix(strings.TrimPrefix(line, "- "), "* ")
			switch currentSection {
			case "issues":
				eval.Issues = append(eval.Issues, item)
			case "recommendations":
				eval.Recommendations = append(eval.Recommendations, item)
			}
		} else if currentSection == "example" && line != "" {
			if eval.ImprovedExample != "" {
				eval.ImprovedExample += "\n"
			}
			eval.ImprovedExample += line
		}
	}

	// Add cardinality info
	if cardAnalysis != nil {
		eval.CardinalityAnalysis = fmt.Sprintf("%s (%d estimated series)",
			cardAnalysis.CardinalityLevel,
			cardAnalysis.EstimatedSeries)
		eval.MemoryImpact = cardAnalysis.MemoryEstimateHuman
	}

	// Set defaults if parsing failed
	if eval.Verdict == "" {
		eval.Verdict = "Analysis Completed"
	}
	if len(eval.Issues) == 0 {
		eval.Issues = []string{"See full response for details"}
	}

	return eval
}
