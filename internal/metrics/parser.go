// ABOUTME: Prometheus metric parser - extracts metric name, labels, values from input
// ABOUTME: Supports various Prometheus exposition formats

package metrics

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wbollock/good_telemetry/internal/cardinality"
)

type Metric struct {
	Name   string
	Labels map[string]string
	Value  string
	Raw    string
}

type ParsedMetrics struct {
	Metrics             []Metric
	CardinalityAnalysis *cardinality.Analysis
}

var (
	// Matches: metric_name{label1="value1",label2="value2"} value (with optional value)
	metricWithLabelsRegex = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)\{([^}]*)\}(?:\s+([0-9.eE+-]+))?`)
	// Matches: metric_name value (no labels, with optional value)
	simpleMetricRegex = regexp.MustCompile(`^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\s+([0-9.eE+-]+))?$`)
)

func Parse(input string) (*ParsedMetrics, error) {
	lines := strings.Split(strings.TrimSpace(input), "\n")
	var metrics []Metric

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		metric, err := parseLine(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		metrics = append(metrics, metric)
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("no valid metrics found")
	}

	// Extract labels for cardinality analysis
	var allLabels []map[string]string
	for _, m := range metrics {
		allLabels = append(allLabels, m.Labels)
	}

	// Calculate cardinality
	analysis := cardinality.Analyze(allLabels)

	return &ParsedMetrics{
		Metrics:             metrics,
		CardinalityAnalysis: analysis,
	}, nil
}

func parseLine(line string) (Metric, error) {
	// Try parsing with labels first
	if matches := metricWithLabelsRegex.FindStringSubmatch(line); matches != nil {
		labels, err := parseLabels(matches[2])
		if err != nil {
			return Metric{}, err
		}

		value := "0"
		if len(matches) > 3 && matches[3] != "" {
			value = matches[3]
		}

		return Metric{
			Name:   matches[1],
			Labels: labels,
			Value:  value,
			Raw:    line,
		}, nil
	}

	// Try simple format without labels
	if matches := simpleMetricRegex.FindStringSubmatch(line); matches != nil {
		value := "0"
		if len(matches) > 2 && matches[2] != "" {
			value = matches[2]
		}

		return Metric{
			Name:   matches[1],
			Labels: make(map[string]string),
			Value:  value,
			Raw:    line,
		}, nil
	}

	return Metric{}, fmt.Errorf("invalid metric format: %s", line)
}

func parseLabels(labelStr string) (map[string]string, error) {
	labels := make(map[string]string)
	if labelStr == "" {
		return labels, nil
	}

	// Split by comma, but handle quoted values
	labelPairs := splitLabels(labelStr)

	for _, pair := range labelPairs {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format: %s", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)

		labels[key] = value
	}

	return labels, nil
}

func splitLabels(s string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for _, ch := range s {
		switch ch {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(ch)
		case ',':
			if inQuotes {
				current.WriteRune(ch)
			} else {
				if current.Len() > 0 {
					result = append(result, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}
