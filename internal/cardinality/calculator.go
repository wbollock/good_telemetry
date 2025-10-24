// ABOUTME: Cardinality calculator - estimates time series cardinality and memory usage
// ABOUTME: Implements formulas from robustperception.io for RAM and ingestion rate calculations

package cardinality

import (
	"fmt"
	"regexp"
	"strings"
)

type Analysis struct {
	EstimatedSeries      int
	MemoryEstimateBytes  int64
	MemoryEstimateHuman  string
	CardinalityLevel     string
	HighCardinalityRisks []string
	LabelAnalysis        map[string]LabelInfo
	Warnings             []string
}

type LabelInfo struct {
	Name               string
	EstimatedValues    int
	CardinalityRisk    string
	IsHighCardinality  bool
	RecommendedAction  string
}

const (
	// Based on robustperception.io article
	// Memory per series varies by churn and scrape interval
	// Conservative estimate: 3KB per series for low churn
	// Higher estimate: 6KB per series for high churn
	memoryPerSeriesBytes = 3000

	// Thresholds
	lowCardinalityThreshold    = 100
	mediumCardinalityThreshold = 1000
	highCardinalityThreshold   = 10000
)

var highCardinalityPatterns = map[string]*regexp.Regexp{
	"user_id":       regexp.MustCompile(`(?i)^(user_?id|userid|user_?name|username)$`),
	"email":         regexp.MustCompile(`(?i)^(email|e_?mail)$`),
	"ip_address":    regexp.MustCompile(`(?i)^(ip_?addr|ip_?address|client_?ip)$`),
	"timestamp":     regexp.MustCompile(`(?i)^(timestamp|ts|epoch|unix_?time|created_?at|updated_?at)$`),
	"uuid":          regexp.MustCompile(`(?i)^(uuid|guid)$`),
	"session":       regexp.MustCompile(`(?i)^(session_?id|session)$`),
	"trace_id":      regexp.MustCompile(`(?i)^(trace_?id|span_?id|request_?id)$`),
	"url_path":      regexp.MustCompile(`(?i)^(path|url|uri)$`),
	"inode":         regexp.MustCompile(`(?i)^(inode|file_?id|fd)$`),
	"volume":        regexp.MustCompile(`(?i)^(vol|volume|volume_?id|disk|disk_?id)$`),
}

func Analyze(allLabels []map[string]string) *Analysis {
	if len(allLabels) == 0 {
		return &Analysis{
			EstimatedSeries:     1,
			MemoryEstimateBytes: memoryPerSeriesBytes,
			MemoryEstimateHuman: formatBytes(memoryPerSeriesBytes),
			CardinalityLevel:    "Low",
			LabelAnalysis:       make(map[string]LabelInfo),
		}
	}

	labelCounts := make(map[string]map[string]bool)

	for _, labels := range allLabels {
		for key, value := range labels {
			if labelCounts[key] == nil {
				labelCounts[key] = make(map[string]bool)
			}
			labelCounts[key][value] = true
		}
	}

	analysis := &Analysis{
		LabelAnalysis: make(map[string]LabelInfo),
		Warnings:      []string{},
	}

	totalCardinality := 1
	hasHighCardinalityRisk := false

	for labelName, values := range labelCounts {
		uniqueValues := len(values)
		totalCardinality *= uniqueValues

		info := LabelInfo{
			Name:            labelName,
			EstimatedValues: uniqueValues,
		}

		// Check for high-cardinality patterns
		for patternName, pattern := range highCardinalityPatterns {
			if pattern.MatchString(labelName) {
				info.IsHighCardinality = true
				info.CardinalityRisk = "HIGH"
				info.RecommendedAction = fmt.Sprintf("Remove %s label (detected as %s) - unbounded cardinality", labelName, patternName)
				analysis.HighCardinalityRisks = append(analysis.HighCardinalityRisks, info.RecommendedAction)
				hasHighCardinalityRisk = true
				break
			}
		}

		if !info.IsHighCardinality {
			if uniqueValues > 100 {
				info.CardinalityRisk = "MEDIUM"
				info.RecommendedAction = fmt.Sprintf("Review %s label - %d unique values is high", labelName, uniqueValues)
				analysis.Warnings = append(analysis.Warnings, info.RecommendedAction)
			} else if uniqueValues > 20 {
				info.CardinalityRisk = "LOW-MEDIUM"
				info.RecommendedAction = fmt.Sprintf("Monitor %s label - %d unique values", labelName, uniqueValues)
			} else {
				info.CardinalityRisk = "LOW"
				info.RecommendedAction = "Good cardinality"
			}
		}

		analysis.LabelAnalysis[labelName] = info
	}

	// Set overall estimates
	if hasHighCardinalityRisk {
		analysis.EstimatedSeries = 1000000 // Assume very high for unbounded
		analysis.CardinalityLevel = "CRITICAL"
		analysis.Warnings = append(analysis.Warnings, "Unbounded cardinality detected - could create millions of series")
	} else {
		analysis.EstimatedSeries = totalCardinality
		if totalCardinality < lowCardinalityThreshold {
			analysis.CardinalityLevel = "Low"
		} else if totalCardinality < mediumCardinalityThreshold {
			analysis.CardinalityLevel = "Medium"
		} else if totalCardinality < highCardinalityThreshold {
			analysis.CardinalityLevel = "High"
			analysis.Warnings = append(analysis.Warnings, fmt.Sprintf("High cardinality: ~%d series", totalCardinality))
		} else {
			analysis.CardinalityLevel = "Very High"
			analysis.Warnings = append(analysis.Warnings, fmt.Sprintf("Very high cardinality: ~%d series - consider reducing labels", totalCardinality))
		}
	}

	// Calculate memory based on robustperception.io formula
	// RAM = (number of active series) × (memory per series)
	analysis.MemoryEstimateBytes = int64(analysis.EstimatedSeries) * memoryPerSeriesBytes
	analysis.MemoryEstimateHuman = formatBytes(analysis.MemoryEstimateBytes)

	return analysis
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Simple version for basic metrics without full label analysis
func EstimateSimple(numSeries int) string {
	bytes := int64(numSeries) * memoryPerSeriesBytes
	return formatBytes(bytes)
}

// Check if a metric name follows best practices
func ValidateMetricName(name string) []string {
	var issues []string

	if !regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`).MatchString(name) {
		issues = append(issues, "Metric name contains invalid characters")
	}

	if strings.Contains(name, "__") {
		issues = append(issues, "Metric name contains double underscore (reserved for Prometheus internal use)")
	}

	// Check if it looks like it should have a unit suffix but doesn't
	if strings.Contains(name, "time") || strings.Contains(name, "duration") || strings.Contains(name, "latency") {
		if !strings.HasSuffix(name, "_seconds") && !strings.HasSuffix(name, "_milliseconds") {
			issues = append(issues, "Time measurement should use _seconds suffix")
		}
	}

	if strings.Contains(name, "size") || strings.Contains(name, "memory") {
		if !strings.HasSuffix(name, "_bytes") {
			issues = append(issues, "Size measurement should use _bytes suffix")
		}
	}

	return issues
}
