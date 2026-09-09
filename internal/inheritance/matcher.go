package inheritance

import (
	"strconv"
	"strings"

	"github.com/shairozan/janus/internal/model"
)

// CorrelationStrategy defines how parameters are matched between source and target.
type CorrelationStrategy int

const (
	// StrategyConservative only matches parameters with identical labels.
	// This is the safest option and the default.
	StrategyConservative CorrelationStrategy = iota

	// StrategyInferred uses fuzzy label matching and positional hints.
	// Matches parameters if labels are similar (case-insensitive, ignoring common prefixes).
	StrategyInferred

	// StrategyPositional matches parameters purely by position.
	// THETA1 -> THETA1, OMEGA(1,1) -> OMEGA(1,1), etc.
	// Use with caution as model structure may have changed.
	StrategyPositional
)

// String returns the string representation of the strategy.
func (s CorrelationStrategy) String() string {
	switch s {
	case StrategyConservative:
		return "conservative"
	case StrategyInferred:
		return "inferred"
	case StrategyPositional:
		return "positional"
	default:
		return "unknown"
	}
}

// ParseCorrelationStrategy parses a string into a CorrelationStrategy.
func ParseCorrelationStrategy(s string) CorrelationStrategy {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "conservative":
		return StrategyConservative
	case "inferred":
		return StrategyInferred
	case "positional":
		return StrategyPositional
	default:
		return StrategyConservative // Default to safest option
	}
}

// Matcher correlates parameters between source and target models.
type Matcher struct {
	strategy CorrelationStrategy
}

// NewMatcher creates a new Matcher with the specified strategy.
func NewMatcher(strategy CorrelationStrategy) *Matcher {
	return &Matcher{strategy: strategy}
}

// ThetaMapping records how a source THETA maps to a target THETA.
type ThetaMapping struct {
	SourceIndex int
	TargetIndex int
	SourceLabel string
	TargetLabel string
	MatchedBy   string // "label", "position", "inferred"
}

// MatrixMapping records how a source OMEGA/SIGMA element maps to a target.
type MatrixMapping struct {
	SourceIndex int
	TargetIndex int
	SourceRow   int
	SourceCol   int
	TargetRow   int
	TargetCol   int
	SourceLabel string
	TargetLabel string
	MatchedBy   string
}

// MatchThetas matches source THETAs to target THETAs based on the strategy.
func (m *Matcher) MatchThetas(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
) []ThetaMapping {
	switch m.strategy {
	case StrategyPositional:
		return m.matchThetasPositional(source, target)
	case StrategyInferred:
		return m.matchThetasInferred(source, target, targetLabels)
	case StrategyConservative:
		fallthrough
	default:
		return m.matchThetasConservative(source, target, targetLabels)
	}
}

// matchThetasConservative matches only by exact label match.
func (m *Matcher) matchThetasConservative(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
) []ThetaMapping {
	mappings := make([]ThetaMapping, 0)
	usedTarget := make(map[int]bool)

	// Build target label lookup
	targetLabelMap := buildTargetLabelMap(target, targetLabels, "THETA")

	for srcIdx, srcParam := range source {
		srcLabel := normalizeLabel(srcParam.Label)
		if srcLabel == "" {
			continue // No label, can't match conservatively
		}

		// Find exact match in target
		for tgtIdx, tgtLabel := range targetLabelMap {
			if usedTarget[tgtIdx] {
				continue
			}

			if srcLabel == tgtLabel {
				mappings = append(mappings, ThetaMapping{
					SourceIndex: srcIdx,
					TargetIndex: tgtIdx,
					SourceLabel: srcParam.Label,
					TargetLabel: target[tgtIdx].Label,
					MatchedBy:   "label",
				})
				usedTarget[tgtIdx] = true

				break
			}
		}
	}

	return mappings
}

// matchThetasInferred uses fuzzy matching for labels.
func (m *Matcher) matchThetasInferred(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
) []ThetaMapping {
	mappings := make([]ThetaMapping, 0)
	usedTarget := make(map[int]bool)

	// Build target label lookup
	targetLabelMap := buildTargetLabelMap(target, targetLabels, "THETA")

	for srcIdx, srcParam := range source {
		srcLabel := normalizeLabel(srcParam.Label)

		// Try exact match first
		for tgtIdx, tgtLabel := range targetLabelMap {
			if usedTarget[tgtIdx] {
				continue
			}

			if srcLabel != "" && srcLabel == tgtLabel {
				mappings = append(mappings, ThetaMapping{
					SourceIndex: srcIdx,
					TargetIndex: tgtIdx,
					SourceLabel: srcParam.Label,
					TargetLabel: target[tgtIdx].Label,
					MatchedBy:   "label",
				})
				usedTarget[tgtIdx] = true

				break
			}
		}
	}

	// Second pass: fuzzy matching for unmatched source params
	for srcIdx, srcParam := range source {
		// Check if already matched
		alreadyMatched := false
		for _, mapping := range mappings {
			if mapping.SourceIndex == srcIdx {
				alreadyMatched = true

				break
			}
		}
		if alreadyMatched {
			continue
		}

		srcLabel := normalizeLabel(srcParam.Label)
		if srcLabel == "" {
			// No label - fall back to position for inferred strategy
			if srcIdx < len(target) && !usedTarget[srcIdx] {
				mappings = append(mappings, ThetaMapping{
					SourceIndex: srcIdx,
					TargetIndex: srcIdx,
					SourceLabel: srcParam.Label,
					TargetLabel: target[srcIdx].Label,
					MatchedBy:   "position",
				})
				usedTarget[srcIdx] = true
			}

			continue
		}

		// Try fuzzy match
		bestMatch := -1
		bestScore := 0.0
		for tgtIdx, tgtLabel := range targetLabelMap {
			if usedTarget[tgtIdx] {
				continue
			}

			score := fuzzyLabelScore(srcLabel, tgtLabel)
			if score > 0.7 && score > bestScore { // 70% similarity threshold
				bestScore = score
				bestMatch = tgtIdx
			}
		}

		if bestMatch >= 0 {
			mappings = append(mappings, ThetaMapping{
				SourceIndex: srcIdx,
				TargetIndex: bestMatch,
				SourceLabel: srcParam.Label,
				TargetLabel: target[bestMatch].Label,
				MatchedBy:   "inferred",
			})
			usedTarget[bestMatch] = true
		}
	}

	return mappings
}

// matchThetasPositional matches purely by position.
func (m *Matcher) matchThetasPositional(
	source []model.ParameterEstimate,
	target []TargetParameter,
) []ThetaMapping {
	mappings := make([]ThetaMapping, 0)

	maxIdx := len(source)
	if len(target) < maxIdx {
		maxIdx = len(target)
	}

	for i := 0; i < maxIdx; i++ {
		mappings = append(mappings, ThetaMapping{
			SourceIndex: i,
			TargetIndex: i,
			SourceLabel: source[i].Label,
			TargetLabel: target[i].Label,
			MatchedBy:   "position",
		})
	}

	return mappings
}

// MatchOmegas matches source OMEGAs to target OMEGAs.
func (m *Matcher) MatchOmegas(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
) []MatrixMapping {
	return m.matchMatrixParams(source, target, targetLabels, "OMEGA")
}

// MatchSigmas matches source SIGMAs to target SIGMAs.
func (m *Matcher) MatchSigmas(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
) []MatrixMapping {
	return m.matchMatrixParams(source, target, targetLabels, "SIGMA")
}

// matchMatrixParams is the generic implementation for OMEGA/SIGMA matching.
func (m *Matcher) matchMatrixParams(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
	paramType string,
) []MatrixMapping {
	switch m.strategy {
	case StrategyPositional:
		return m.matchMatrixPositional(source, target, paramType)
	case StrategyInferred:
		return m.matchMatrixInferred(source, target, targetLabels, paramType)
	case StrategyConservative:
		fallthrough
	default:
		return m.matchMatrixConservative(source, target, targetLabels, paramType)
	}
}

// matchMatrixConservative matches only by exact label match.
func (m *Matcher) matchMatrixConservative(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
	paramType string,
) []MatrixMapping {
	mappings := make([]MatrixMapping, 0)
	usedTarget := make(map[int]bool)

	targetLabelMap := buildTargetLabelMap(target, targetLabels, paramType)

	for srcIdx, srcParam := range source {
		srcLabel := normalizeLabel(srcParam.Label)
		if srcLabel == "" {
			continue
		}

		row, col := parseMatrixIndex(srcParam.Name)

		for tgtIdx, tgtLabel := range targetLabelMap {
			if usedTarget[tgtIdx] {
				continue
			}

			if srcLabel == tgtLabel {
				tgtRow, tgtCol := tgtIdx, tgtIdx // Simplified - diagonal

				mappings = append(mappings, MatrixMapping{
					SourceIndex: srcIdx,
					TargetIndex: tgtIdx,
					SourceRow:   row,
					SourceCol:   col,
					TargetRow:   tgtRow,
					TargetCol:   tgtCol,
					SourceLabel: srcParam.Label,
					TargetLabel: target[tgtIdx].Label,
					MatchedBy:   "label",
				})
				usedTarget[tgtIdx] = true

				break
			}
		}
	}

	return mappings
}

// matchMatrixInferred uses fuzzy matching.
func (m *Matcher) matchMatrixInferred(
	source []model.ParameterEstimate,
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
	paramType string,
) []MatrixMapping {
	// Start with conservative matches
	mappings := m.matchMatrixConservative(source, target, targetLabels, paramType)

	usedTarget := make(map[int]bool)
	for _, mapping := range mappings {
		usedTarget[mapping.TargetIndex] = true
	}

	// Try to match remaining by position
	for srcIdx, srcParam := range source {
		alreadyMatched := false
		for _, mapping := range mappings {
			if mapping.SourceIndex == srcIdx {
				alreadyMatched = true

				break
			}
		}
		if alreadyMatched {
			continue
		}

		row, col := parseMatrixIndex(srcParam.Name)

		// Try positional match for unmatched
		if srcIdx < len(target) && !usedTarget[srcIdx] {
			mappings = append(mappings, MatrixMapping{
				SourceIndex: srcIdx,
				TargetIndex: srcIdx,
				SourceRow:   row,
				SourceCol:   col,
				TargetRow:   srcIdx,
				TargetCol:   srcIdx,
				SourceLabel: srcParam.Label,
				TargetLabel: target[srcIdx].Label,
				MatchedBy:   "position",
			})
			usedTarget[srcIdx] = true
		}
	}

	return mappings
}

// matchMatrixPositional matches purely by position.
func (m *Matcher) matchMatrixPositional(
	source []model.ParameterEstimate,
	target []TargetParameter,
	_ string,
) []MatrixMapping {
	mappings := make([]MatrixMapping, 0)

	maxIdx := len(source)
	if len(target) < maxIdx {
		maxIdx = len(target)
	}

	for i := 0; i < maxIdx; i++ {
		row, col := parseMatrixIndex(source[i].Name)

		mappings = append(mappings, MatrixMapping{
			SourceIndex: i,
			TargetIndex: i,
			SourceRow:   row,
			SourceCol:   col,
			TargetRow:   i,
			TargetCol:   i,
			SourceLabel: source[i].Label,
			TargetLabel: target[i].Label,
			MatchedBy:   "position",
		})
	}

	return mappings
}

// buildTargetLabelMap creates a map of target index to normalized label.
func buildTargetLabelMap(
	target []TargetParameter,
	targetLabels *model.ParameterLabels,
	paramType string,
) map[int]string {
	result := make(map[int]string)

	// First, use labels from parsed control stream
	for i, param := range target {
		if param.Label != "" {
			result[i] = normalizeLabel(param.Label)
		}
	}

	// Override with explicitly parsed labels if available
	if targetLabels != nil {
		switch paramType {
		case "THETA":
			// Thetas use map[int]string with 1-based indexing
			for pos, label := range targetLabels.Thetas {
				if label != "" && pos-1 < len(target) {
					result[pos-1] = normalizeLabel(label) // Convert to 0-based
				}
			}
		case "OMEGA":
			// Omegas use map[string]string with "row,col" keys
			for i := range target {
				posKey := formatPosition(i + 1) // 1-based diagonal position
				if label, ok := targetLabels.Omegas[posKey]; ok && label != "" {
					result[i] = normalizeLabel(label)
				}
			}
		case "SIGMA":
			// Sigmas use map[string]string with "row,col" keys
			for i := range target {
				posKey := formatPosition(i + 1) // 1-based diagonal position
				if label, ok := targetLabels.Sigmas[posKey]; ok && label != "" {
					result[i] = normalizeLabel(label)
				}
			}
		}
	}

	return result
}

// formatPosition creates a diagonal position key like "1,1" for index 1.
func formatPosition(index int) string {
	return strconv.Itoa(index) + "," + strconv.Itoa(index)
}

// normalizeLabel normalizes a label for comparison.
func normalizeLabel(label string) string {
	if label == "" {
		return ""
	}

	// Convert to uppercase
	normalized := strings.ToUpper(strings.TrimSpace(label))

	// Remove common prefixes
	prefixes := []string{"TV", "LOG_", "LN_", "ETA_", "EPS_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)

			break
		}
	}

	return normalized
}

// fuzzyLabelScore returns a similarity score between two labels (0-1).
func fuzzyLabelScore(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	a = strings.ToUpper(a)
	b = strings.ToUpper(b)

	if a == b {
		return 1.0
	}

	// Simple character overlap scoring
	// Could be enhanced with Levenshtein distance
	aChars := make(map[rune]int)
	for _, c := range a {
		aChars[c]++
	}

	matches := 0
	for _, c := range b {
		if aChars[c] > 0 {
			matches++
			aChars[c]--
		}
	}

	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}

	return float64(matches) / float64(maxLen)
}

// parseMatrixIndex extracts row and column from a matrix parameter name like "OMEGA(1,2)".
func parseMatrixIndex(name string) (row, col int) {
	// Default to diagonal
	row, col = 0, 0

	// Try to parse from name like "OMEGA(1,2)" or "SIGMA(1,1)"
	start := strings.Index(name, "(")
	end := strings.Index(name, ")")
	if start < 0 || end < 0 || end <= start {
		return
	}

	indices := name[start+1 : end]
	parts := strings.Split(indices, ",")
	if len(parts) >= 1 {
		if _, err := parseToInt(parts[0]); err == nil {
			row = mustParseInt(parts[0]) - 1 // Convert to 0-based
		}
	}
	if len(parts) >= 2 {
		if _, err := parseToInt(parts[1]); err == nil {
			col = mustParseInt(parts[1]) - 1
		}
	} else {
		col = row // Diagonal
	}

	return
}

// parseToInt parses a string to int, returning error if invalid.
func parseToInt(s string) (int, error) {
	s = strings.TrimSpace(s)
	var result int
	_, err := strings.NewReader(s).Read([]byte{})
	if err != nil {
		return 0, err
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, &invalidIntError{s}
		}
		result = result*10 + int(c-'0')
	}

	return result, nil
}

type invalidIntError struct {
	s string
}

func (e *invalidIntError) Error() string {
	return "invalid integer: " + e.s
}

// mustParseInt parses a string to int, returning 0 on error.
func mustParseInt(s string) int {
	result, _ := parseToInt(s)

	return result
}
