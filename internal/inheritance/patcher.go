// Package inheritance provides functionality for inheriting parameter estimates
// from previous model runs and patching them into new control streams.
package inheritance

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/pharmalytica/janus/internal/model"
)

// PatchOptions configures behavior for patching parameter estimates.
type PatchOptions struct {
	// Strategy determines how parameters are matched between source and target.
	// Defaults to StrategyConservative if not specified.
	Strategy CorrelationStrategy

	// InheritFixed controls whether FIXED parameters in the source should be
	// inherited as FIXED in the target. Defaults to false (preserve target FIX status).
	InheritFixed bool

	// PreserveBounds keeps the original bounds (lower, upper) from the target model
	// and only replaces the initial estimate. Defaults to true.
	PreserveBounds bool

	// WarnOutOfBounds generates warnings when inherited estimates fall outside
	// the target's bounds. Defaults to true.
	WarnOutOfBounds bool
}

// DefaultPatchOptions returns sensible defaults for patching.
func DefaultPatchOptions() PatchOptions {
	return PatchOptions{
		Strategy:        StrategyConservative,
		InheritFixed:    false,
		PreserveBounds:  true,
		WarnOutOfBounds: true,
	}
}

// ParameterMapping records how a source parameter was mapped to a target parameter.
type ParameterMapping struct {
	SourceName     string  // e.g., "THETA1", "OMEGA(1,1)"
	SourceLabel    string  // Label from source if available
	TargetName     string  // e.g., "THETA1", "OMEGA(1,1)"
	TargetLabel    string  // Label from target if available
	SourceEstimate float64 // The estimate value from source
	TargetOriginal float64 // The original estimate in target
	MatchedBy      string  // "label", "position", or "name"
}

// PatchResult contains the results of patching a control stream.
type PatchResult struct {
	// ModifiedContent is the patched control stream content.
	ModifiedContent string

	// Mappings records how each parameter was matched and modified.
	Mappings []ParameterMapping

	// Warnings contains non-fatal issues encountered during patching.
	// Examples: estimate outside bounds, unmatched parameters, etc.
	Warnings []string

	// UnmatchedSource lists parameters in source that couldn't be matched.
	UnmatchedSource []string

	// UnmatchedTarget lists parameters in target that weren't updated.
	UnmatchedTarget []string
}

// PatchControlStream patches the target control stream with parameter estimates
// from the source model summary.
//
// The function preserves the structure of the target control stream (comments,
// formatting, bounds) and only modifies initial estimate values.
//
// Parameters:
//   - targetContent: The control stream text to be modified
//   - sourceSummary: Summary containing parameter estimates to inherit
//   - targetLabels: Labels parsed from the target control stream
//   - opts: Options controlling matching and patching behavior
//
// Returns:
//   - PatchResult containing the modified content and detailed mapping info
//   - error if patching fails completely
func PatchControlStream(
	targetContent string,
	sourceSummary *model.ModelSummary,
	targetLabels *model.ParameterLabels,
	opts PatchOptions,
) (*PatchResult, error) {
	if sourceSummary == nil {
		return nil, fmt.Errorf("source summary is required")
	}

	result := &PatchResult{
		ModifiedContent: targetContent,
		Mappings:        make([]ParameterMapping, 0),
		Warnings:        make([]string, 0),
		UnmatchedSource: make([]string, 0),
		UnmatchedTarget: make([]string, 0),
	}

	// Build matcher for correlating parameters
	matcher := NewMatcher(opts.Strategy)

	// Patch THETAs
	result.ModifiedContent = patchThetaBlock(
		result.ModifiedContent,
		sourceSummary.Parameters.Thetas,
		targetLabels,
		matcher,
		opts,
		result,
	)

	// Patch OMEGAs
	result.ModifiedContent = patchOmegaBlock(
		result.ModifiedContent,
		sourceSummary.Parameters.Omegas,
		targetLabels,
		matcher,
		opts,
		result,
	)

	// Patch SIGMAs
	result.ModifiedContent = patchSigmaBlock(
		result.ModifiedContent,
		sourceSummary.Parameters.Sigmas,
		targetLabels,
		matcher,
		opts,
		result,
	)

	// Add provenance comment if any parameters were patched
	if len(result.Mappings) > 0 {
		result.ModifiedContent = addProvenanceComment(result.ModifiedContent, sourceSummary)
	}

	return result, nil
}

// patchThetaBlock patches all THETA parameters in the control stream.
func patchThetaBlock(
	content string,
	sourceThetas []model.ParameterEstimate,
	targetLabels *model.ParameterLabels,
	matcher *Matcher,
	opts PatchOptions,
	result *PatchResult,
) string {
	if len(sourceThetas) == 0 {
		return content
	}

	// Parse target thetas to understand positions
	targetThetas := parseTargetThetas(content)

	// Build mappings between source and target
	mappings := matcher.MatchThetas(sourceThetas, targetThetas, targetLabels)

	// Track which source thetas have been matched
	matchedSource := make(map[int]bool)

	// Replace theta values in the content
	content = replaceBlockValues(content, "THETA", sourceThetas, mappings, opts, result, matchedSource)

	// Record unmatched source thetas
	for i, theta := range sourceThetas {
		if !matchedSource[i] {
			result.UnmatchedSource = append(result.UnmatchedSource, theta.Name)
		}
	}

	return content
}

// patchOmegaBlock patches all OMEGA parameters in the control stream.
func patchOmegaBlock(
	content string,
	sourceOmegas []model.ParameterEstimate,
	targetLabels *model.ParameterLabels,
	matcher *Matcher,
	opts PatchOptions,
	result *PatchResult,
) string {
	if len(sourceOmegas) == 0 {
		return content
	}

	// Parse target omegas
	targetOmegas := parseTargetOmegas(content)

	// Build mappings
	mappings := matcher.MatchOmegas(sourceOmegas, targetOmegas, targetLabels)

	// Track matched source omegas
	matchedSource := make(map[int]bool)

	// Replace omega values using matrix mappings
	content = replaceMatrixBlockValues(content, "OMEGA", sourceOmegas, mappings, opts, result, matchedSource)

	// Record unmatched
	for i, omega := range sourceOmegas {
		if !matchedSource[i] {
			result.UnmatchedSource = append(result.UnmatchedSource, omega.Name)
		}
	}

	return content
}

// patchSigmaBlock patches all SIGMA parameters in the control stream.
func patchSigmaBlock(
	content string,
	sourceSigmas []model.ParameterEstimate,
	targetLabels *model.ParameterLabels,
	matcher *Matcher,
	opts PatchOptions,
	result *PatchResult,
) string {
	if len(sourceSigmas) == 0 {
		return content
	}

	// Parse target sigmas
	targetSigmas := parseTargetSigmas(content)

	// Build mappings
	mappings := matcher.MatchSigmas(sourceSigmas, targetSigmas, targetLabels)

	// Track matched source sigmas
	matchedSource := make(map[int]bool)

	// Replace sigma values using matrix mappings
	content = replaceMatrixBlockValues(content, "SIGMA", sourceSigmas, mappings, opts, result, matchedSource)

	// Record unmatched
	for i, sigma := range sourceSigmas {
		if !matchedSource[i] {
			result.UnmatchedSource = append(result.UnmatchedSource, sigma.Name)
		}
	}

	return content
}

// replaceBlockValues replaces parameter values in THETA blocks.
func replaceBlockValues(
	content string,
	blockType string,
	sourceParams []model.ParameterEstimate,
	mappings []ThetaMapping,
	opts PatchOptions,
	result *PatchResult,
	matchedSource map[int]bool,
) string {
	lines := strings.Split(content, "\n")
	inBlock := false
	paramIndex := 0

	for i, line := range lines {
		upperLine := strings.TrimSpace(strings.ToUpper(line))

		// Check for block start
		if strings.HasPrefix(upperLine, "$"+blockType) {
			inBlock = true

			continue
		}

		// Check for block end (another $ directive)
		if inBlock && strings.HasPrefix(upperLine, "$") {
			inBlock = false
			paramIndex = 0

			continue
		}

		// Skip if not in block
		if !inBlock {
			continue
		}

		// Skip comments and empty lines
		if isCommentOrEmpty(line) {
			continue
		}

		// Find mapping for this parameter position
		var mapping *ThetaMapping
		for _, m := range mappings {
			if m.TargetIndex == paramIndex {
				mapping = &m

				break
			}
		}

		if mapping != nil && mapping.SourceIndex >= 0 && mapping.SourceIndex < len(sourceParams) {
			sourceParam := sourceParams[mapping.SourceIndex]
			if sourceParam.Estimate != nil {
				newLine := patchThetaLine(line, *sourceParam.Estimate, opts)
				if newLine != line {
					lines[i] = newLine

					result.Mappings = append(result.Mappings, ParameterMapping{
						SourceName:     sourceParam.Name,
						SourceLabel:    sourceParam.Label,
						TargetName:     fmt.Sprintf("%s%d", blockType, paramIndex+1),
						TargetLabel:    mapping.TargetLabel,
						SourceEstimate: *sourceParam.Estimate,
						MatchedBy:      mapping.MatchedBy,
					})

					matchedSource[mapping.SourceIndex] = true
				}
			}
		}

		paramIndex++
	}

	return strings.Join(lines, "\n")
}

// replaceMatrixBlockValues replaces parameter values in OMEGA/SIGMA blocks.
func replaceMatrixBlockValues(
	content string,
	blockType string,
	sourceParams []model.ParameterEstimate,
	mappings []MatrixMapping,
	opts PatchOptions,
	result *PatchResult,
	matchedSource map[int]bool,
) string {
	lines := strings.Split(content, "\n")
	inBlock := false
	elementIndex := 0

	for i, line := range lines {
		upperLine := strings.TrimSpace(strings.ToUpper(line))

		// Check for block start
		if strings.HasPrefix(upperLine, "$"+blockType) {
			inBlock = true

			continue
		}

		// Check for block end (another $ directive)
		if inBlock && strings.HasPrefix(upperLine, "$") {
			inBlock = false
			elementIndex = 0

			continue
		}

		// Skip if not in block
		if !inBlock {
			continue
		}

		// Skip comments, empty lines, and BLOCK/DIAGONAL declarations
		if isCommentOrEmpty(line) {
			continue
		}
		if strings.HasPrefix(upperLine, "BLOCK") || strings.HasPrefix(upperLine, "DIAGONAL") {
			continue
		}

		// Find mapping for this element
		var mapping *MatrixMapping
		for _, m := range mappings {
			if m.TargetIndex == elementIndex {
				mapping = &m

				break
			}
		}

		if mapping != nil && mapping.SourceIndex >= 0 && mapping.SourceIndex < len(sourceParams) {
			sourceParam := sourceParams[mapping.SourceIndex]
			if sourceParam.Estimate != nil {
				newLine := patchMatrixLine(line, *sourceParam.Estimate, opts)
				if newLine != line {
					lines[i] = newLine

					result.Mappings = append(result.Mappings, ParameterMapping{
						SourceName:     sourceParam.Name,
						SourceLabel:    sourceParam.Label,
						TargetName:     fmt.Sprintf("%s(%d,%d)", blockType, mapping.TargetRow+1, mapping.TargetCol+1),
						TargetLabel:    mapping.TargetLabel,
						SourceEstimate: *sourceParam.Estimate,
						MatchedBy:      mapping.MatchedBy,
					})

					matchedSource[mapping.SourceIndex] = true
				}
			}
		}

		elementIndex++
	}

	return strings.Join(lines, "\n")
}

// patchThetaLine replaces the initial estimate in a THETA line while preserving bounds.
func patchThetaLine(line string, newEstimate float64, opts PatchOptions) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") {
		return line
	}

	// Extract leading whitespace
	leadingWS := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	// Split off trailing comment
	parts := strings.SplitN(trimmed, ";", 2)
	paramPart := strings.TrimSpace(parts[0])
	comment := ""
	if len(parts) > 1 {
		comment = " ; " + strings.TrimSpace(parts[1])
	}

	// Check for FIX modifier
	hasFix := false
	fixPattern := regexp.MustCompile(`(?i)\s+FIX(ED)?(\s|$)`)
	if fixPattern.MatchString(paramPart) {
		hasFix = true
		paramPart = fixPattern.ReplaceAllString(paramPart, "")
		paramPart = strings.TrimSpace(paramPart)
	}

	// Check if bounded format (lower, init, upper)
	boundedPattern := regexp.MustCompile(`^\(([^,]*),\s*([^,]*),\s*([^)]*)\)$`)
	if matches := boundedPattern.FindStringSubmatch(paramPart); matches != nil {
		// Bounded format
		lower := strings.TrimSpace(matches[1])
		upper := strings.TrimSpace(matches[3])

		if opts.PreserveBounds {
			paramPart = fmt.Sprintf("(%s, %g, %s)", lower, newEstimate, upper)
		} else {
			paramPart = fmt.Sprintf("%g", newEstimate)
		}
	} else {
		// Simple format - just replace the number
		paramPart = fmt.Sprintf("%g", newEstimate)
	}

	// Reconstruct line
	lineResult := leadingWS + paramPart
	if hasFix {
		lineResult += " FIX"
	}
	lineResult += comment

	return lineResult
}

// patchMatrixLine replaces the value in an OMEGA/SIGMA line.
func patchMatrixLine(line string, newEstimate float64, _ PatchOptions) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") {
		return line
	}

	leadingWS := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	// Split off trailing comment
	parts := strings.SplitN(trimmed, ";", 2)
	paramPart := strings.TrimSpace(parts[0])
	comment := ""
	if len(parts) > 1 {
		comment = " ; " + strings.TrimSpace(parts[1])
	}

	// Check for FIX
	hasFix := false
	fixPattern := regexp.MustCompile(`(?i)\s+FIX(ED)?(\s|$)`)
	if fixPattern.MatchString(paramPart) {
		hasFix = true
	}

	// Replace the value
	paramPart = fmt.Sprintf("%g", newEstimate)

	// Reconstruct
	lineResult := leadingWS + paramPart
	if hasFix {
		lineResult += " FIX"
	}
	lineResult += comment

	return lineResult
}

// addProvenanceComment adds a comment indicating parameter inheritance.
func addProvenanceComment(content string, source *model.ModelSummary) string {
	// Find position after $PROBLEM line to insert comment
	problemPattern := regexp.MustCompile(`(?im)^(\s*\$PROBLEM\s+[^\n]*\n)`)

	provenance := fmt.Sprintf("; Parameters inherited from: %s\n", source.RunID)

	if problemPattern.MatchString(content) {
		content = problemPattern.ReplaceAllString(content, "${1}"+provenance)
	} else {
		// No $PROBLEM found, prepend to file
		content = provenance + content
	}

	return content
}

// isCommentOrEmpty returns true if the line is a comment or empty.
func isCommentOrEmpty(line string) bool {
	trimmed := strings.TrimSpace(line)

	return trimmed == "" || strings.HasPrefix(trimmed, ";")
}

// parseTargetThetas extracts THETA information from the target control stream.
func parseTargetThetas(content string) []TargetParameter {
	return parseBlockParameters(content, "THETA")
}

// parseTargetOmegas extracts OMEGA information from the target control stream.
func parseTargetOmegas(content string) []TargetParameter {
	return parseBlockParameters(content, "OMEGA")
}

// parseTargetSigmas extracts SIGMA information from the target control stream.
func parseTargetSigmas(content string) []TargetParameter {
	return parseBlockParameters(content, "SIGMA")
}

// parseBlockParameters extracts parameter information from a specific block type.
func parseBlockParameters(content, blockType string) []TargetParameter {
	var params []TargetParameter

	lines := strings.Split(content, "\n")
	inBlock := false

	for _, line := range lines {
		upperLine := strings.TrimSpace(strings.ToUpper(line))

		// Check for block start
		if strings.HasPrefix(upperLine, "$"+blockType) {
			inBlock = true

			continue
		}

		// Check for block end
		if inBlock && strings.HasPrefix(upperLine, "$") {
			inBlock = false

			continue
		}

		if !inBlock {
			continue
		}

		// Skip comments, empty lines, and structure declarations
		if isCommentOrEmpty(line) {
			continue
		}
		if strings.HasPrefix(upperLine, "BLOCK") || strings.HasPrefix(upperLine, "DIAGONAL") {
			continue
		}

		param := parseParameterLine(line, len(params))
		params = append(params, param)
	}

	return params
}

// parseParameterLine extracts parameter info from a single line.
func parseParameterLine(line string, index int) TargetParameter {
	param := TargetParameter{
		Index: index,
		Line:  line,
	}

	// Extract label from comment if present
	if idx := strings.Index(line, ";"); idx >= 0 {
		comment := strings.TrimSpace(line[idx+1:])
		// First word of comment is often the label
		if words := strings.Fields(comment); len(words) > 0 {
			param.Label = words[0]
		}
	}

	// Extract estimate value
	parts := strings.SplitN(strings.TrimSpace(line), ";", 2)
	paramPart := strings.TrimSpace(parts[0])

	// Remove FIX
	fixPattern := regexp.MustCompile(`(?i)\s+FIX(ED)?(\s|$)`)
	paramPart = fixPattern.ReplaceAllString(paramPart, "")
	paramPart = strings.TrimSpace(paramPart)

	// Check for bounded format
	boundedPattern := regexp.MustCompile(`^\(([^,]*),\s*([^,]*),\s*([^)]*)\)$`)
	if matches := boundedPattern.FindStringSubmatch(paramPart); matches != nil {
		if val, err := strconv.ParseFloat(strings.TrimSpace(matches[2]), 64); err == nil {
			param.Estimate = val
		}
	} else {
		// Simple format
		if val, err := strconv.ParseFloat(paramPart, 64); err == nil {
			param.Estimate = val
		}
	}

	return param
}

// TargetParameter represents a parameter parsed from the target control stream.
type TargetParameter struct {
	Index    int
	Label    string
	Estimate float64
	Line     string
}
