// Package model contains the label parser for extracting parameter labels from NONMEM control streams.
package model

import (
	"fmt"
	"regexp"
	"strings"
)

// ParameterLabels holds extracted labels from a control stream.
// Labels are extracted from comments following parameter definitions.
type ParameterLabels struct {
	Thetas map[int]string    // Position (1-based) -> label (e.g., 1 -> "KA")
	Omegas map[string]string // Matrix position -> label (e.g., "1,1" -> "iiv CL")
	Sigmas map[string]string // Matrix position -> label (e.g., "1,1" -> "prop")
}

// ParseParameterLabels extracts parameter labels from control stream content.
// It parses $THETA, $OMEGA, and $SIGMA blocks looking for comment labels.
// Returns an empty ParameterLabels struct (not nil) if no labels are found.
func ParseParameterLabels(content string) *ParameterLabels {
	labels := &ParameterLabels{
		Thetas: make(map[int]string),
		Omegas: make(map[string]string),
		Sigmas: make(map[string]string),
	}

	lines := strings.Split(content, "\n")

	var currentBlock string
	var thetaIndex, omegaIndex, sigmaIndex int

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for block start
		upperLine := strings.ToUpper(trimmed)
		if strings.HasPrefix(upperLine, "$THETA") {
			currentBlock = "theta"
			// Check if there are inline values on the same line as $THETA
			afterKeyword := strings.TrimSpace(trimmed[6:])
			if afterKeyword != "" && !strings.HasPrefix(afterKeyword, ";") {
				// Parse inline theta values
				thetaIndex = parseInlineThetas(afterKeyword, thetaIndex, labels)
			}

			continue
		}
		if strings.HasPrefix(upperLine, "$OMEGA") {
			currentBlock = "omega"
			// Check if there are inline values on the same line as $OMEGA
			afterKeyword := strings.TrimSpace(trimmed[6:])
			if afterKeyword != "" && !strings.HasPrefix(afterKeyword, ";") {
				omegaIndex = parseInlineMatrix(afterKeyword, omegaIndex, labels.Omegas)
			}

			continue
		}
		if strings.HasPrefix(upperLine, "$SIGMA") {
			currentBlock = "sigma"
			// Check if there are inline values on the same line as $SIGMA
			afterKeyword := strings.TrimSpace(trimmed[6:])
			if afterKeyword != "" && !strings.HasPrefix(afterKeyword, ";") {
				sigmaIndex = parseInlineMatrix(afterKeyword, sigmaIndex, labels.Sigmas)
			}

			continue
		}

		// Check for new block (ends current block)
		if strings.HasPrefix(trimmed, "$") {
			currentBlock = ""

			continue
		}

		// Skip empty lines and pure comment lines
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}

		// Parse line based on current block
		switch currentBlock {
		case "theta":
			thetaIndex++
			if label := extractLabel(line); label != "" {
				labels.Thetas[thetaIndex] = label
			}
		case "omega":
			valueCount := countMatrixValues(trimmed)
			if valueCount > 0 {
				// For multiple values on one line, only the last one can have a label
				for i := 0; i < valueCount-1; i++ {
					omegaIndex++
				}
				omegaIndex++
				// For diagonal omega, use "n,n" format
				pos := formatMatrixPosition(omegaIndex)
				if label := extractLabel(line); label != "" {
					labels.Omegas[pos] = label
				}
			}
		case "sigma":
			valueCount := countMatrixValues(trimmed)
			if valueCount > 0 {
				// For multiple values on one line, only the last one can have a label
				for i := 0; i < valueCount-1; i++ {
					sigmaIndex++
				}
				sigmaIndex++
				pos := formatMatrixPosition(sigmaIndex)
				if label := extractLabel(line); label != "" {
					labels.Sigmas[pos] = label
				}
			}
		}
	}

	return labels
}

// extractLabel extracts the label from a comment on a parameter line.
// Examples:
//   - "(0, 2)  ; KA"       -> "KA"
//   - "0.05    ; iiv CL"   -> "iiv CL"
//   - "(0, 0.5) ;1 prop"   -> "1 prop"
//   - "1 FIX"              -> "" (no comment)
func extractLabel(line string) string {
	// Find semicolon that starts a comment
	idx := strings.Index(line, ";")
	if idx == -1 {
		return ""
	}

	// Extract everything after the semicolon
	comment := strings.TrimSpace(line[idx+1:])

	// Clean up the label
	return normalizeLabel(comment)
}

// normalizeLabel cleans up a label for consistent comparison.
func normalizeLabel(label string) string {
	// Trim whitespace
	label = strings.TrimSpace(label)

	// Remove leading semicolons (handles double comments like ";; Label")
	label = strings.TrimLeft(label, "; ")
	label = strings.TrimSpace(label)

	// Remove any trailing comments or annotations
	// Some models have nested comments like "KA ; absorption rate"
	if idx := strings.Index(label, ";"); idx > 0 {
		label = strings.TrimSpace(label[:idx])
	}

	return label
}

// NormalizeForComparison normalizes a label for correlation matching.
// This converts to lowercase and removes common prefixes for comparison.
func NormalizeForComparison(label string) string {
	if label == "" {
		return ""
	}

	// Convert to lowercase for case-insensitive matching
	normalized := strings.ToLower(strings.TrimSpace(label))

	return normalized
}

// parseInlineMatrix handles cases like "$OMEGA 0.04 0.04" or "$SIGMA 0.1 0.05 ; label"
// where values are on the same line as the block keyword.
func parseInlineMatrix(content string, startIndex int, labelMap map[string]string) int {
	// Extract label if present
	label := ""
	if idx := strings.Index(content, ";"); idx >= 0 {
		label = normalizeLabel(content[idx+1:])
		content = content[:idx]
	}
	content = strings.TrimSpace(content)

	valueCount := countMatrixValues(content)
	if valueCount == 0 {
		return startIndex
	}

	// Increment for each value, apply label to the last one
	for i := 0; i < valueCount; i++ {
		startIndex++
		if i == valueCount-1 && label != "" {
			pos := formatMatrixPosition(startIndex)
			labelMap[pos] = label
		}
	}

	return startIndex
}

// countMatrixValues counts the number of OMEGA/SIGMA values on a line.
// Handles formats like "0.04 0.04" (2 values) or "0.05" (1 value).
// Does not count BLOCK declarations - those are handled separately.
func countMatrixValues(line string) int {
	// Remove comment portion
	if idx := strings.Index(line, ";"); idx >= 0 {
		line = line[:idx]
	}
	line = strings.TrimSpace(line)

	if line == "" {
		return 0
	}

	// Check for BLOCK syntax - this declares structure, not individual values
	upperLine := strings.ToUpper(line)
	if strings.Contains(upperLine, "BLOCK") {
		return 0
	}

	// Count numeric values (handles both simple and parenthesized)
	count := 0
	parts := strings.Fields(line)
	for _, part := range parts {
		upperPart := strings.ToUpper(part)
		// Skip modifiers
		if upperPart == "FIX" || upperPart == "FIXED" ||
			upperPart == "SAME" || upperPart == "DIAGONAL" ||
			upperPart == "SD" || upperPart == "VARIANCE" ||
			upperPart == "CORRELATION" || upperPart == "CHOLESKY" {

			continue
		}
		// Check if it looks like a numeric value (starts with digit, minus, or paren)
		if len(part) > 0 && (part[0] == '(' || part[0] == '-' || part[0] == '.' ||
			(part[0] >= '0' && part[0] <= '9')) {

			count++
		}
	}

	return count
}

// parseInlineThetas handles cases like "$THETA 1 2 3" or "$THETA (0,1) (0,2)"
// where one or more values are on the same line as $THETA. A trailing comment
// labels the last value on the line (e.g. "$THETA (0,1) ; CL" labels THETA1 "CL"),
// matching how parseInlineMatrix treats OMEGA/SIGMA blocks.
func parseInlineThetas(content string, startIndex int, labels *ParameterLabels) int {
	// Extract a trailing label comment, if present, before counting values.
	label := ""
	if idx := strings.Index(content, ";"); idx >= 0 {
		label = normalizeLabel(content[idx+1:])
		content = strings.TrimSpace(content[:idx])
	}

	// Count the theta values declared on this line.
	var count int
	if strings.Contains(content, "(") {
		// Parenthesized values like (0, 2) or (0.02)
		parenPattern := regexp.MustCompile(`\([^)]+\)`)
		count = len(parenPattern.FindAllString(content, -1))
	} else {
		// Space-separated values, e.g. "0.5 0.1 4 30 200"
		for _, part := range strings.Fields(content) {
			// Skip modifiers like FIX, FIXED
			if strings.ToUpper(part) == "FIX" || strings.ToUpper(part) == "FIXED" {
				continue
			}
			count++
		}
	}

	// Advance the index for each value; the label, if any, belongs to the last.
	for i := 0; i < count; i++ {
		startIndex++
		if i == count-1 && label != "" {
			labels.Thetas[startIndex] = label
		}
	}

	return startIndex
}

// formatMatrixPosition formats a diagonal matrix position.
// For simplicity, we assume diagonal elements only (1,1), (2,2), etc.
// Returns format like "1,1" or "2,2" to match OMEGA(1,1) naming.
func formatMatrixPosition(index int) string {
	return fmt.Sprintf("%d,%d", index, index)
}

// GetThetaLabel returns the label for a theta parameter by its position (1-based).
func (p *ParameterLabels) GetThetaLabel(position int) string {
	if p == nil || p.Thetas == nil {
		return ""
	}

	return p.Thetas[position]
}

// GetOmegaLabel returns the label for an omega parameter by its matrix position.
func (p *ParameterLabels) GetOmegaLabel(position string) string {
	if p == nil || p.Omegas == nil {
		return ""
	}

	return p.Omegas[position]
}

// GetSigmaLabel returns the label for a sigma parameter by its matrix position.
func (p *ParameterLabels) GetSigmaLabel(position string) string {
	if p == nil || p.Sigmas == nil {
		return ""
	}

	return p.Sigmas[position]
}
