package comparison

import (
	"fmt"
	"math"
)

// LRTResult contains likelihood ratio test results.
type LRTResult struct {
	BaseOFV       float64 // OFV of base/reference model
	TestOFV       float64 // OFV of test model
	DeltaOFV      float64 // Difference (TestOFV - BaseOFV), negative = improvement
	DeltaParams   int     // Difference in parameter count (test - base)
	ChiSquareCrit float64 // Critical value at alpha
	PValue        float64 // Approximate p-value (0 if can't be calculated)
	Significant   bool    // True if improvement is significant
	Alpha         float64 // Significance level used
}

// Chi-square critical values at alpha=0.05 for degrees of freedom 1-10.
// These are the values that DeltaOFV must exceed for significance.
var chiSquareCritical005 = map[int]float64{
	1:  3.841,
	2:  5.991,
	3:  7.815,
	4:  9.488,
	5:  11.070,
	6:  12.592,
	7:  14.067,
	8:  15.507,
	9:  16.919,
	10: 18.307,
}

// Chi-square critical values at alpha=0.01 for degrees of freedom 1-10.
var chiSquareCritical001 = map[int]float64{
	1:  6.635,
	2:  9.210,
	3:  11.345,
	4:  13.277,
	5:  15.086,
	6:  16.812,
	7:  18.475,
	8:  20.090,
	9:  21.666,
	10: 23.209,
}

// CalculateLRT performs likelihood ratio test between two runs.
// baseOFV: OFV of the simpler/base model
// testOFV: OFV of the more complex/test model
// deltaParams: number of additional parameters in test model (must be >= 0)
// alpha: significance level (0.05 or 0.01 supported)
func CalculateLRT(baseOFV, testOFV float64, deltaParams int, alpha float64) *LRTResult {
	result := &LRTResult{
		BaseOFV:     baseOFV,
		TestOFV:     testOFV,
		DeltaOFV:    testOFV - baseOFV, // Negative = improvement
		DeltaParams: deltaParams,
		Alpha:       alpha,
	}

	// Get critical value based on alpha
	var critTable map[int]float64
	switch alpha {
	case 0.01:
		critTable = chiSquareCritical001
	default:
		critTable = chiSquareCritical005
		result.Alpha = 0.05
	}

	// Look up critical value
	if deltaParams > 0 && deltaParams <= 10 {
		result.ChiSquareCrit = critTable[deltaParams]
	} else if deltaParams > 10 {
		// Approximate for df > 10 using Wilson-Hilferty approximation
		// For simplicity, use a conservative estimate
		result.ChiSquareCrit = float64(deltaParams) + 2*math.Sqrt(float64(2*deltaParams))
	}

	// Determine significance
	// The test statistic is -DeltaOFV (we want the magnitude of improvement)
	// Model is significantly better if -DeltaOFV > ChiSquareCrit
	if deltaParams > 0 {
		result.Significant = -result.DeltaOFV > result.ChiSquareCrit
	}

	// Approximate p-value using chi-square distribution
	// This is a rough approximation using the survival function
	if deltaParams > 0 && result.DeltaOFV < 0 {
		result.PValue = approximateChiSquarePValue(-result.DeltaOFV, deltaParams)
	} else {
		result.PValue = 1.0 // No improvement or invalid
	}

	return result
}

// approximateChiSquarePValue approximates the p-value for a chi-square test.
// Uses Wilson-Hilferty transformation for approximation.
func approximateChiSquarePValue(x float64, df int) float64 {
	if x <= 0 || df <= 0 {
		return 1.0
	}

	// Wilson-Hilferty transformation
	// Transforms chi-square to approximately normal
	k := float64(df)
	z := math.Pow(x/k, 1.0/3.0) - (1.0 - 2.0/(9.0*k))
	z /= math.Sqrt(2.0 / (9.0 * k))

	// Standard normal CDF approximation (upper tail)
	// Using error function approximation
	p := 0.5 * (1.0 - erf(z/math.Sqrt2))

	if p < 0 {
		p = 0
	}

	if p > 1 {
		p = 1
	}

	return p
}

// erf is the error function approximation.
func erf(x float64) float64 {
	// Approximation using Horner's method
	// Accurate to about 1.5e-7
	sign := 1.0
	if x < 0 {
		sign = -1.0
		x = -x
	}

	// Constants
	a1 := 0.254829592
	a2 := -0.284496736
	a3 := 1.421413741
	a4 := -1.453152027
	a5 := 1.061405429
	p := 0.3275911

	t := 1.0 / (1.0 + p*x)
	y := 1.0 - (((((a5*t+a4)*t)+a3)*t+a2)*t+a1)*t*math.Exp(-x*x)

	return sign * y
}

// FormatLRTResult formats the LRT result for display.
func FormatLRTResult(result *LRTResult) string {
	if result == nil {
		return "LRT: N/A"
	}

	var sigText string
	switch {
	case result.DeltaParams <= 0:
		sigText = "(same parameters)"
	case result.Significant:
		sigText = fmt.Sprintf("✓ Significant at α=%.2f", result.Alpha)
	default:
		sigText = fmt.Sprintf("✗ Not significant at α=%.2f", result.Alpha)
	}

	pText := ""
	if result.PValue > 0 && result.PValue < 1 {
		if result.PValue < 0.001 {
			pText = "p < 0.001"
		} else {
			pText = fmt.Sprintf("p ≈ %.3f", result.PValue)
		}
	}

	return fmt.Sprintf("ΔOFV: %.2f  Δparams: %d  %s  %s",
		result.DeltaOFV, result.DeltaParams, pText, sigText)
}

// InterpretLRT provides a human-readable interpretation of the LRT result.
func InterpretLRT(result *LRTResult, baseID, testID string) string {
	if result == nil {
		return "Cannot perform likelihood ratio test: missing data."
	}

	if result.DeltaParams <= 0 {
		return fmt.Sprintf(
			"Runs %s and %s have the same number of parameters. "+
				"The OFV changed by %.2f (%.2f → %.2f).",
			baseID, testID, result.DeltaOFV, result.BaseOFV, result.TestOFV)
	}

	if result.DeltaOFV >= 0 {
		return fmt.Sprintf(
			"Run %s (OFV=%.2f) shows no improvement over %s (OFV=%.2f). "+
				"The more complex model with %d additional parameters did not improve fit.",
			testID, result.TestOFV, baseID, result.BaseOFV, result.DeltaParams)
	}

	if result.Significant {
		return fmt.Sprintf(
			"Run %s shows a SIGNIFICANT improvement over %s. "+
				"The OFV decreased by %.2f with %d additional parameters "+
				"(critical value: %.2f at α=%.2f).",
			testID, baseID, -result.DeltaOFV, result.DeltaParams,
			result.ChiSquareCrit, result.Alpha)
	}

	return fmt.Sprintf(
		"Run %s shows improvement but NOT statistically significant. "+
			"The OFV decreased by %.2f with %d additional parameters, "+
			"but this does not exceed the critical value of %.2f (α=%.2f).",
		testID, -result.DeltaOFV, result.DeltaParams,
		result.ChiSquareCrit, result.Alpha)
}
