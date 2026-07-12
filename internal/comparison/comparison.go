// Package comparison provides functionality for comparing NONMEM run results,
// calculating parameter deltas, and determining statistical significance.
package comparison

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/pharmalytica/janus/internal/model"
	"github.com/pharmalytica/janus/internal/runlog"
)

// HighlightLevel indicates the severity of a parameter change.
type HighlightLevel int

const (
	HighlightNone    HighlightLevel = iota // No significant change
	HighlightMinor                         // < 5% change
	HighlightMajor                         // 5-10% change
	HighlightWarning                       // > 10% change
)

// CorrelationStatus indicates how a parameter was correlated across runs.
type CorrelationStatus int

const (
	CorrelationUnknown   CorrelationStatus = iota // No correlation info available
	CorrelationKnown                              // Matched by label (e.g., both have "CL")
	CorrelationInferred                           // Positional match (same index, no labels)
	CorrelationUnrelated                          // New param or no match in other runs
)

// String returns a human-readable representation of the correlation status.
func (c CorrelationStatus) String() string {
	switch c {
	case CorrelationUnknown:
		return "unknown"
	case CorrelationKnown:
		return "known"
	case CorrelationInferred:
		return "inferred"
	case CorrelationUnrelated:
		return "unrelated"
	}

	return "unknown"
}

// CorrelationStrategy defines how parameters are matched across runs.
type CorrelationStrategy int

const (
	// StrategyConservative only correlates parameters with matching labels.
	// Unlabeled parameters are treated as unrelated, even if they share the same position.
	// This is the safest option when label consistency is uncertain.
	StrategyConservative CorrelationStrategy = iota

	// StrategyInferred correlates by label first, then attempts positional matching
	// for unlabeled parameters. Parameters at the same position (e.g., THETA1) across
	// runs are inferred to be the same parameter if neither has a label.
	StrategyInferred

	// StrategyPositional ignores labels entirely and correlates purely by position.
	// All parameters at the same index are assumed to be the same parameter.
	// Use only when labels are known to be unreliable or absent.
	StrategyPositional
)

// String returns a human-readable name for the strategy.
func (s CorrelationStrategy) String() string {
	switch s {
	case StrategyConservative:
		return "conservative"
	case StrategyInferred:
		return "inferred"
	case StrategyPositional:
		return "positional"
	}

	return "unknown"
}

// ParseCorrelationStrategy parses a string into a CorrelationStrategy.
// Returns StrategyConservative as the default for unrecognized values.
func ParseCorrelationStrategy(s string) CorrelationStrategy {
	switch s {
	case "conservative":
		return StrategyConservative
	case "inferred":
		return StrategyInferred
	case "positional":
		return StrategyPositional
	default:
		return StrategyConservative
	}
}

// DefaultThresholds for highlighting parameter changes.
var DefaultThresholds = HighlightThresholds{
	MinorPct:   5.0,
	MajorPct:   10.0,
	WarningPct: 10.0, // Same as major for now, but could be different
}

// HighlightThresholds configures the percentage thresholds for highlighting.
type HighlightThresholds struct {
	MinorPct   float64 // Threshold for minor highlight (default 5%)
	MajorPct   float64 // Threshold for major highlight (default 10%)
	WarningPct float64 // Threshold for warning highlight (default 10%)
}

// ComparisonResult holds the result of comparing multiple runs.
type ComparisonResult struct {
	Runs          []*RunSnapshot // Snapshots of compared runs (ordered by timestamp)
	ParameterRows []ParameterRow // Aligned parameter comparisons
	OFVRow        *ParameterRow  // Special row for OFV comparison
	MetadataRows  []MetadataRow  // Non-numeric comparisons (minimized, cov step, etc.)
}

// RunSnapshot contains the relevant data from a RunRecord for comparison.
type RunSnapshot struct {
	ID        string
	Timestamp time.Time
	OFV       *float64
	Minimized bool
	CovStep   bool
	Subjects  int
	Obs       int
	Method    string
	Thetas    []model.ParameterEstimate
	Omegas    []model.ParameterEstimate
	Sigmas    []model.ParameterEstimate
}

// ParameterRow represents one row in the comparison table.
type ParameterRow struct {
	Name        string            // e.g., "THETA1", "OMEGA(1,1)", "OFV"
	Label       string            // Human-readable label from control stream (e.g., "CL", "Ka")
	Type        string            // "theta", "omega", "sigma", "ofv"
	Values      []*float64        // Value for each run (nil if missing)
	Delta       *float64          // Absolute change (last - first)
	DeltaPct    *float64          // Percentage change
	Highlight   HighlightLevel    // Highlight level based on change
	Correlation CorrelationStatus // How this param was correlated across runs
	FirstSeenIn int               // Index of run where param first appears (-1 if present in all)
}

// MetadataRow represents non-numeric comparison data.
type MetadataRow struct {
	Name   string   // e.g., "Minimized", "Cov Step"
	Values []string // String value for each run
}

// CompareRuns compares 2-4 runs and returns structured comparison data.
// Uses the conservative correlation strategy (label-based matching only).
func CompareRuns(records []*runlog.RunRecord) (*ComparisonResult, error) {
	return CompareRunsWithStrategy(records, StrategyConservative)
}

// CompareRunsWithStrategy compares 2-4 runs using the specified correlation strategy.
func CompareRunsWithStrategy(records []*runlog.RunRecord, strategy CorrelationStrategy) (*ComparisonResult, error) {
	if len(records) < 2 {
		return nil, fmt.Errorf("at least 2 runs required for comparison, got %d", len(records))
	}

	if len(records) > 4 {
		return nil, fmt.Errorf("maximum 4 runs can be compared, got %d", len(records))
	}

	// Convert records to snapshots
	snapshots := make([]*RunSnapshot, len(records))
	for i, rec := range records {
		snapshots[i] = recordToSnapshot(rec)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
	})

	result := &ComparisonResult{
		Runs: snapshots,
	}

	// Build OFV row
	result.OFVRow = buildOFVRow(snapshots)

	// Build parameter rows using specified strategy
	result.ParameterRows = alignParametersWithStrategy(snapshots, strategy)

	// Build metadata rows
	result.MetadataRows = buildMetadataRows(snapshots)

	return result, nil
}

// recordToSnapshot extracts comparison-relevant data from a RunRecord.
func recordToSnapshot(rec *runlog.RunRecord) *RunSnapshot {
	snap := &RunSnapshot{
		ID:        rec.ID,
		Timestamp: rec.Timestamp,
	}

	if rec.Summary != nil {
		snap.OFV = rec.Summary.GoodnessOfFit.ObjectiveFunctionValue
		snap.Minimized = rec.Summary.Estimation.Minimized
		snap.CovStep = rec.Summary.Diagnostics.CovarianceStepSuccess
		snap.Subjects = rec.Summary.Estimation.Subjects
		snap.Obs = rec.Summary.Estimation.Observations
		snap.Method = rec.Summary.Estimation.Method
		snap.Thetas = rec.Summary.Parameters.Thetas
		snap.Omegas = rec.Summary.Parameters.Omegas
		snap.Sigmas = rec.Summary.Parameters.Sigmas
	}

	return snap
}

// buildOFVRow creates the OFV comparison row.
func buildOFVRow(snapshots []*RunSnapshot) *ParameterRow {
	values := make([]*float64, len(snapshots))
	for i, snap := range snapshots {
		values[i] = snap.OFV
	}

	delta, deltaPct := CalculateDelta(values[0], values[len(values)-1])
	highlight := DetermineHighlight(deltaPct, DefaultThresholds)

	// For OFV, negative delta (improvement) should not be warning
	if delta != nil && *delta < 0 {
		highlight = HighlightNone // OFV decrease is good
	}

	return &ParameterRow{
		Name:      "OFV",
		Type:      "ofv",
		Values:    values,
		Delta:     delta,
		DeltaPct:  deltaPct,
		Highlight: highlight,
	}
}

// alignParametersWithStrategy aligns parameters using the specified correlation strategy.
func alignParametersWithStrategy(snapshots []*RunSnapshot, strategy CorrelationStrategy) []ParameterRow {
	var rows []ParameterRow

	// Build rows for each parameter type using the specified strategy
	rows = append(rows, alignParametersByStrategy(snapshots, "theta", strategy)...)
	rows = append(rows, alignParametersByStrategy(snapshots, "omega", strategy)...)
	rows = append(rows, alignParametersByStrategy(snapshots, "sigma", strategy)...)

	return rows
}

// paramWithRunInfo holds a parameter with metadata about which run it came from.
type paramWithRunInfo struct {
	param  model.ParameterEstimate
	runIdx int
}

// alignParametersByStrategy aligns parameters using the specified correlation strategy.
func alignParametersByStrategy(snapshots []*RunSnapshot, paramType string, strategy CorrelationStrategy) []ParameterRow {
	// For positional strategy, skip label processing entirely
	if strategy == StrategyPositional {
		return alignParametersByPosition(snapshots, paramType)
	}

	// Collect all parameters with their run info
	var allParams []paramWithRunInfo
	for runIdx, snap := range snapshots {
		params := getParamsForType(snap, paramType)
		for _, p := range params {
			allParams = append(allParams, paramWithRunInfo{
				param:  p,
				runIdx: runIdx,
			})
		}
	}

	// Group parameters by normalized label (for those that have labels)
	labelGroups := make(map[string][]paramWithRunInfo)
	var unlabeledParams []paramWithRunInfo

	for _, pwi := range allParams {
		normalizedLabel := model.NormalizeForComparison(pwi.param.Label)
		if normalizedLabel != "" {
			labelGroups[normalizedLabel] = append(labelGroups[normalizedLabel], pwi)
		} else {
			unlabeledParams = append(unlabeledParams, pwi)
		}
	}

	var rows []ParameterRow

	// Build a map from positional name to label group for merging unlabeled params
	// This handles the case where THETA2 has label "CL" in run 1 but no label in run 2
	positionToLabelGroup := make(map[string]string)
	for _, pwi := range allParams {
		normalizedLabel := model.NormalizeForComparison(pwi.param.Label)
		if normalizedLabel != "" {
			positionToLabelGroup[pwi.param.Name] = normalizedLabel
		}
	}

	// Merge unlabeled params into their corresponding label groups if position matches
	for _, pwi := range unlabeledParams {
		if labelKey, ok := positionToLabelGroup[pwi.param.Name]; ok {
			// This unlabeled param's position matches a labeled param - merge it
			labelGroups[labelKey] = append(labelGroups[labelKey], pwi)
		}
	}

	// Track which positional names have been covered by labeled groups
	coveredPositions := make(map[string]bool)

	// Process labeled groups first (CorrelationKnown)
	processedLabels := make([]string, 0, len(labelGroups))
	for label := range labelGroups {
		processedLabels = append(processedLabels, label)
	}
	// Order labeled groups by the position of their parameters so the logical
	// model order is preserved (e.g. THETA1 "CL" before THETA2 "V") rather than
	// sorting alphabetically by label text.
	sort.SliceStable(processedLabels, func(i, j int) bool {
		return naturalParamLess(groupPositionKey(labelGroups[processedLabels[i]]), groupPositionKey(labelGroups[processedLabels[j]]))
	})

	for _, label := range processedLabels {
		group := labelGroups[label]
		row := buildCorrelatedRow(snapshots, group, paramType, CorrelationKnown)
		rows = append(rows, row)
		// Mark all positional names in this group as covered
		for _, pwi := range group {
			coveredPositions[pwi.param.Name] = true
		}
	}

	// Process unlabeled parameters that don't have a matching labeled position
	unlabeledByName := make(map[string][]paramWithRunInfo)
	for _, pwi := range unlabeledParams {
		if coveredPositions[pwi.param.Name] {
			continue // Already merged into a labeled row
		}
		unlabeledByName[pwi.param.Name] = append(unlabeledByName[pwi.param.Name], pwi)
	}

	unlabeledNames := make([]string, 0, len(unlabeledByName))
	for name := range unlabeledByName {
		unlabeledNames = append(unlabeledNames, name)
	}
	sort.SliceStable(unlabeledNames, func(i, j int) bool {
		return naturalParamLess(unlabeledNames[i], unlabeledNames[j])
	})

	for _, name := range unlabeledNames {
		group := unlabeledByName[name]

		// Determine correlation status based on strategy
		var status CorrelationStatus
		switch strategy {
		case StrategyInferred:
			// If the same positional name appears in multiple runs, infer correlation
			if hasMultipleRuns(group) {
				status = CorrelationInferred
			} else {
				status = CorrelationUnrelated
			}
		case StrategyConservative, StrategyPositional:
			// StrategyPositional is handled by alignParametersByPosition, shouldn't reach here
			status = CorrelationUnrelated
		}

		row := buildCorrelatedRow(snapshots, group, paramType, status)
		rows = append(rows, row)
	}

	return rows
}

// alignParametersByPosition aligns parameters purely by position, ignoring labels.
func alignParametersByPosition(snapshots []*RunSnapshot, paramType string) []ParameterRow {
	// Group all parameters by their positional name (e.g., THETA1, OMEGA(1,1))
	paramsByName := make(map[string][]paramWithRunInfo)

	for runIdx, snap := range snapshots {
		params := getParamsForType(snap, paramType)
		for _, p := range params {
			paramsByName[p.Name] = append(paramsByName[p.Name], paramWithRunInfo{
				param:  p,
				runIdx: runIdx,
			})
		}
	}

	// Sort names for consistent ordering, numerically by position (THETA1,
	// THETA2, ..., THETA10) rather than lexicographically.
	names := make([]string, 0, len(paramsByName))
	for name := range paramsByName {
		names = append(names, name)
	}
	sort.SliceStable(names, func(i, j int) bool {
		return naturalParamLess(names[i], names[j])
	})

	var rows []ParameterRow
	for _, name := range names {
		group := paramsByName[name]

		// Determine correlation status
		var status CorrelationStatus
		if hasMultipleRuns(group) {
			status = CorrelationInferred // Positional match across runs
		} else {
			status = CorrelationUnrelated // Only in one run
		}

		row := buildCorrelatedRow(snapshots, group, paramType, status)
		rows = append(rows, row)
	}

	return rows
}

// hasMultipleRuns checks if a parameter group spans multiple runs.
func hasMultipleRuns(group []paramWithRunInfo) bool {
	if len(group) <= 1 {
		return false
	}

	firstRun := group[0].runIdx
	for _, pwi := range group[1:] {
		if pwi.runIdx != firstRun {
			return true
		}
	}

	return false
}

// getParamsForType returns the parameters for a given type from a snapshot.
func getParamsForType(snap *RunSnapshot, paramType string) []model.ParameterEstimate {
	switch paramType {
	case "theta":
		return snap.Thetas
	case "omega":
		return snap.Omegas
	case "sigma":
		return snap.Sigmas
	default:
		return nil
	}
}

// buildCorrelatedRow builds a ParameterRow from a group of correlated parameters.
func buildCorrelatedRow(snapshots []*RunSnapshot, group []paramWithRunInfo, paramType string, correlation CorrelationStatus) ParameterRow {
	numRuns := len(snapshots)
	values := make([]*float64, numRuns)

	// Determine which run each value came from
	firstSeenIn := -1
	var name, label string

	for _, pwi := range group {
		// Bounds check to prevent out-of-bounds access
		if pwi.runIdx < 0 || pwi.runIdx >= numRuns {
			continue
		}
		values[pwi.runIdx] = pwi.param.Estimate
		if name == "" {
			name = pwi.param.Name
		}
		if label == "" && pwi.param.Label != "" {
			label = pwi.param.Label
		}
	}

	// Find first run where this parameter appears
	for i, v := range values {
		if v != nil {
			if firstSeenIn == -1 {
				firstSeenIn = i
			}
		}
	}

	// If present in all runs, set to -1
	allPresent := true
	for _, v := range values {
		if v == nil {
			allPresent = false

			break
		}
	}
	if allPresent {
		firstSeenIn = -1
	}

	// Calculate delta between first and last available values
	var firstVal, lastVal *float64
	for i := 0; i < numRuns; i++ {
		if values[i] != nil {
			if firstVal == nil {
				firstVal = values[i]
			}
			lastVal = values[i]
		}
	}

	delta, deltaPct := CalculateDelta(firstVal, lastVal)
	highlight := DetermineHighlight(deltaPct, DefaultThresholds)

	return ParameterRow{
		Name:        name,
		Label:       label,
		Type:        paramType,
		Values:      values,
		Delta:       delta,
		DeltaPct:    deltaPct,
		Highlight:   highlight,
		Correlation: correlation,
		FirstSeenIn: firstSeenIn,
	}
}

// buildMetadataRows builds non-numeric comparison rows.
func buildMetadataRows(snapshots []*RunSnapshot) []MetadataRow {
	rows := []MetadataRow{
		{Name: "Minimized", Values: make([]string, len(snapshots))},
		{Name: "Cov Step", Values: make([]string, len(snapshots))},
		{Name: "Method", Values: make([]string, len(snapshots))},
		{Name: "Subjects", Values: make([]string, len(snapshots))},
		{Name: "Observations", Values: make([]string, len(snapshots))},
	}

	for i, snap := range snapshots {
		if snap.Minimized {
			rows[0].Values[i] = "✓"
		} else {
			rows[0].Values[i] = "✗"
		}

		if snap.CovStep {
			rows[1].Values[i] = "✓"
		} else {
			rows[1].Values[i] = "✗"
		}

		if snap.Method != "" {
			rows[2].Values[i] = abbreviateMethod(snap.Method)
		} else {
			rows[2].Values[i] = MissingValueDisplay
		}

		if snap.Subjects > 0 {
			rows[3].Values[i] = fmt.Sprintf("%d", snap.Subjects)
		} else {
			rows[3].Values[i] = MissingValueDisplay
		}

		if snap.Obs > 0 {
			rows[4].Values[i] = fmt.Sprintf("%d", snap.Obs)
		} else {
			rows[4].Values[i] = MissingValueDisplay
		}
	}

	return rows
}

// abbreviateMethod shortens common NONMEM estimation method names for display.
func abbreviateMethod(method string) string {
	// Common NONMEM method abbreviations
	abbreviations := map[string]string{
		"FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION": "FOCE-I",
		"FIRST ORDER CONDITIONAL ESTIMATION":                  "FOCE",
		"FIRST ORDER":                                         "FO",
		"LAPLACIAN CONDITIONAL ESTIMATION WITH INTERACTION":   "LAPLACE-I",
		"LAPLACIAN CONDITIONAL ESTIMATION":                    "LAPLACE",
		"ITERATIVE TWO STAGE":                                 "ITS",
		"IMPORTANCE SAMPLING":                                 "IMP",
		"STOCHASTIC APPROXIMATION EXPECTATION-MAXIMIZATION":   "SAEM",
		"MARKOV CHAIN MONTE CARLO BAYESIAN ANALYSIS":          "MCMC-BAYES",
	}

	// Check for exact match first
	if abbrev, ok := abbreviations[method]; ok {
		return abbrev
	}

	// Check for partial matches (method may have additional suffixes)
	for full, abbrev := range abbreviations {
		if len(method) >= len(full) && method[:len(full)] == full {
			return abbrev
		}
	}

	// If no match and method is too long, truncate with ellipsis
	const maxLen = 15
	if len(method) > maxLen {
		return method[:maxLen-1] + "…"
	}

	return method
}

// CalculateDelta computes absolute and percentage change between two values.
// Returns (nil, nil) if either value is nil.
func CalculateDelta(baseline, comparison *float64) (delta, deltaPct *float64) {
	if baseline == nil || comparison == nil {
		return nil, nil
	}

	d := *comparison - *baseline
	delta = &d

	// Avoid division by zero
	if *baseline == 0 {
		if *comparison == 0 {
			pct := 0.0
			deltaPct = &pct
		}
		// If baseline is 0 but comparison isn't, percentage is undefined

		return delta, deltaPct
	}

	pct := (d / math.Abs(*baseline)) * 100
	deltaPct = &pct

	return delta, deltaPct
}

// DetermineHighlight returns highlight level based on percentage change.
func DetermineHighlight(deltaPct *float64, thresholds HighlightThresholds) HighlightLevel {
	if deltaPct == nil {
		return HighlightNone
	}

	absPct := math.Abs(*deltaPct)

	if absPct >= thresholds.WarningPct {
		return HighlightWarning
	}

	if absPct >= thresholds.MajorPct {
		return HighlightMajor
	}

	if absPct >= thresholds.MinorPct {
		return HighlightMinor
	}

	return HighlightNone
}

// MissingValueDisplay is the string shown when a value is not available.
const MissingValueDisplay = "—"

// FormatDelta formats a delta value for display.
func FormatDelta(delta *float64) string {
	if delta == nil {
		return MissingValueDisplay
	}

	if *delta >= 0 {
		return fmt.Sprintf("+%.4g", *delta)
	}

	return fmt.Sprintf("%.4g", *delta)
}

// FormatDeltaPct formats a percentage delta for display.
func FormatDeltaPct(deltaPct *float64) string {
	if deltaPct == nil {
		return MissingValueDisplay
	}

	if *deltaPct >= 0 {
		return fmt.Sprintf("+%.1f%%", *deltaPct)
	}

	return fmt.Sprintf("%.1f%%", *deltaPct)
}

// FormatValue formats a parameter value for display.
func FormatValue(value *float64) string {
	if value == nil {
		return MissingValueDisplay
	}

	return fmt.Sprintf("%.4g", *value)
}

// FormatCorrelationBadge returns a badge string for the correlation status.
// In a GUI context, these would be rendered with colors:
// KNOWN=green (matched by label), INFER=yellow (positional), NEW=orange (unrelated).
func FormatCorrelationBadge(status CorrelationStatus) string {
	switch status {
	case CorrelationUnknown:
		return ""
	case CorrelationKnown:
		return "[KNOWN]"
	case CorrelationInferred:
		return "[INFER]"
	case CorrelationUnrelated:
		return "[NEW]"
	}

	return ""
}

// FormatParameterWithLabel formats a parameter name with its label for display.
// Example: "THETA1 (CL)" or just "THETA1" if no label.
func FormatParameterWithLabel(name, label string) string {
	if label == "" {
		return name
	}

	return fmt.Sprintf("%s (%s)", name, label)
}

// naturalParamLess compares two parameter identifiers so that embedded numbers
// sort numerically rather than lexicographically. This preserves the logical
// model order modelers expect (THETA1, THETA2, ..., THETA10) and handles matrix
// names like OMEGA(1,1) and OMEGA(2,2) by comparing each numeric run in order.
func naturalParamLess(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		aDigit, bDigit := isDigit(a[0]), isDigit(b[0])

		// When both chunks are numeric, compare them as integers.
		if aDigit && bDigit {
			var aNum, bNum string
			aNum, a = splitDigits(a)
			bNum, b = splitDigits(b)
			if aNum != bNum {
				ai, _ := strconv.Atoi(aNum)
				bi, _ := strconv.Atoi(bNum)

				return ai < bi
			}

			continue
		}

		// Otherwise compare a single byte of non-numeric text.
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}

	// Shorter string sorts first when one is a prefix of the other.
	return len(a) < len(b)
}

// isDigit reports whether b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// splitDigits returns the leading run of digits in s and the remainder.
func splitDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}

	return s[:i], s[i:]
}

// groupPositionKey returns the naturally-smallest positional parameter name in a
// correlated group, used to order labeled groups by model position rather than by
// the (alphabetical) label text.
func groupPositionKey(group []paramWithRunInfo) string {
	key := ""
	for i, pwi := range group {
		if i == 0 || naturalParamLess(pwi.param.Name, key) {
			key = pwi.param.Name
		}
	}

	return key
}
