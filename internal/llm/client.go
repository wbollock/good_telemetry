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

	sb.WriteString("You are a Prometheus metrics expert. Evaluate the following Prometheus metric(s) for quality.\n\n")
	sb.WriteString("METRICS:\n")
	for _, m := range parsed.Metrics {
		sb.WriteString(fmt.Sprintf("%s\n", m.Raw))
	}
	sb.WriteString("\n")

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

	sb.WriteString("Evaluate based on:\n")
	sb.WriteString("1. Metric naming conventions (snake_case, appropriate suffixes like _total, _bytes, _seconds)\n")
	sb.WriteString("2. Label usage (avoid high-cardinality labels like user_id, email, timestamps)\n")
	sb.WriteString("3. Cardinality (combination of labels should not explode)\n")
	sb.WriteString("4. Structure and best practices\n\n")

	sb.WriteString("Provide your evaluation in this format:\n")
	sb.WriteString("VERDICT: [Good/Needs Improvement/Poor]\n")
	sb.WriteString("ISSUES:\n- [list issues, one per line]\n")
	sb.WriteString("RECOMMENDATIONS:\n- [list recommendations, one per line]\n")
	sb.WriteString("IMPROVED EXAMPLE:\n[show corrected version]\n")

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
