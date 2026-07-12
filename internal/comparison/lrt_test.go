package comparison

import (
	"math"
	"testing"
)

func TestCalculateLRT_Significant(t *testing.T) {
	// OFV drops by 6.74 with 2 additional parameters
	// Critical value at df=2, alpha=0.05 is 5.99
	// So this should be significant
	result := CalculateLRT(2636.86, 2630.12, 2, 0.05)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.DeltaOFV >= 0 {
		t.Errorf("expected negative DeltaOFV (improvement), got %f", result.DeltaOFV)
	}

	expectedDelta := 2630.12 - 2636.86 // -6.74
	if math.Abs(result.DeltaOFV-expectedDelta) > 0.01 {
		t.Errorf("expected DeltaOFV=%f, got %f", expectedDelta, result.DeltaOFV)
	}

	if !result.Significant {
		t.Error("expected significant result (6.74 > 5.99)")
	}

	if result.ChiSquareCrit != 5.991 {
		t.Errorf("expected ChiSquareCrit=5.991, got %f", result.ChiSquareCrit)
	}
}

func TestCalculateLRT_NotSignificant(t *testing.T) {
	// OFV drops by only 2.0 with 1 additional parameter
	// Critical value at df=1, alpha=0.05 is 3.84
	// So this should NOT be significant
	result := CalculateLRT(100.0, 98.0, 1, 0.05)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Significant {
		t.Error("expected NOT significant result (2.0 < 3.84)")
	}
}

func TestCalculateLRT_ZeroDF(t *testing.T) {
	// Same number of parameters - can still compare OFV but no LRT significance
	result := CalculateLRT(100.0, 95.0, 0, 0.05)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.DeltaOFV != -5.0 {
		t.Errorf("expected DeltaOFV=-5, got %f", result.DeltaOFV)
	}

	// Can't be significant with 0 df
	if result.Significant {
		t.Error("expected NOT significant with 0 df")
	}
}

func TestCalculateLRT_OFVIncreased(t *testing.T) {
	// OFV got worse (increased)
	result := CalculateLRT(100.0, 105.0, 1, 0.05)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.DeltaOFV <= 0 {
		t.Errorf("expected positive DeltaOFV (worsening), got %f", result.DeltaOFV)
	}

	if result.Significant {
		t.Error("expected NOT significant when OFV increased")
	}

	// P-value should be high (close to 1)
	if result.PValue < 0.5 {
		t.Errorf("expected high p-value for worsening OFV, got %f", result.PValue)
	}
}

func TestCalculateLRT_KnownValues(t *testing.T) {
	// Test against known chi-square critical values
	tests := []struct {
		df       int
		expected float64
	}{
		{1, 3.841},
		{2, 5.991},
		{3, 7.815},
		{4, 9.488},
		{5, 11.070},
	}

	for _, tt := range tests {
		result := CalculateLRT(100.0, 99.0, tt.df, 0.05)
		if math.Abs(result.ChiSquareCrit-tt.expected) > 0.001 {
			t.Errorf("df=%d: expected ChiSquareCrit=%f, got %f", tt.df, tt.expected, result.ChiSquareCrit)
		}
	}
}

func TestCalculateLRT_Alpha001(t *testing.T) {
	// Test with alpha=0.01 (stricter)
	result := CalculateLRT(100.0, 94.0, 1, 0.01)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// At alpha=0.01, df=1, critical value is 6.635
	if result.ChiSquareCrit != 6.635 {
		t.Errorf("expected ChiSquareCrit=6.635, got %f", result.ChiSquareCrit)
	}

	// 6.0 < 6.635, so not significant at 0.01
	if result.Significant {
		t.Error("expected NOT significant at alpha=0.01")
	}
}

func TestCalculateLRT_LargeDF(t *testing.T) {
	// Test with df > 10 (uses approximation)
	result := CalculateLRT(100.0, 50.0, 15, 0.05)

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have some reasonable critical value
	if result.ChiSquareCrit <= 0 {
		t.Errorf("expected positive ChiSquareCrit for large df, got %f", result.ChiSquareCrit)
	}

	// With 50 point drop and 15 params, should be significant
	if !result.Significant {
		t.Error("expected significant with 50 point OFV drop")
	}
}

func TestFormatLRTResult(t *testing.T) {
	// Significant result
	result := &LRTResult{
		BaseOFV:       2636.86,
		TestOFV:       2630.12,
		DeltaOFV:      -6.74,
		DeltaParams:   2,
		ChiSquareCrit: 5.991,
		PValue:        0.034,
		Significant:   true,
		Alpha:         0.05,
	}

	formatted := FormatLRTResult(result)

	if formatted == "" {
		t.Error("expected non-empty formatted result")
	}

	// Should contain key information
	if len(formatted) < 20 {
		t.Errorf("expected detailed format, got: %s", formatted)
	}
}

func TestFormatLRTResult_Nil(t *testing.T) {
	formatted := FormatLRTResult(nil)

	if formatted != "LRT: N/A" {
		t.Errorf("expected 'LRT: N/A', got: %s", formatted)
	}
}

func TestInterpretLRT_Significant(t *testing.T) {
	result := &LRTResult{
		BaseOFV:       100.0,
		TestOFV:       92.0,
		DeltaOFV:      -8.0,
		DeltaParams:   2,
		ChiSquareCrit: 5.991,
		Significant:   true,
		Alpha:         0.05,
	}

	interpretation := InterpretLRT(result, "run-1", "run-2")

	if interpretation == "" {
		t.Error("expected non-empty interpretation")
	}

	// Should mention "significant"
	if len(interpretation) < 50 {
		t.Errorf("expected detailed interpretation, got: %s", interpretation)
	}
}

func TestInterpretLRT_NotSignificant(t *testing.T) {
	result := &LRTResult{
		BaseOFV:       100.0,
		TestOFV:       98.0,
		DeltaOFV:      -2.0,
		DeltaParams:   1,
		ChiSquareCrit: 3.841,
		Significant:   false,
		Alpha:         0.05,
	}

	interpretation := InterpretLRT(result, "run-1", "run-2")

	if interpretation == "" {
		t.Error("expected non-empty interpretation")
	}
}

func TestInterpretLRT_Nil(t *testing.T) {
	interpretation := InterpretLRT(nil, "run-1", "run-2")

	if interpretation == "" {
		t.Error("expected non-empty interpretation for nil result")
	}
}

func TestApproximateChiSquarePValue(t *testing.T) {
	// Test some known approximate values
	// At df=1, x=3.841 should give p≈0.05
	p := approximateChiSquarePValue(3.841, 1)
	if math.Abs(p-0.05) > 0.02 {
		t.Errorf("expected p≈0.05 for x=3.841 df=1, got %f", p)
	}

	// Large x should give small p
	p = approximateChiSquarePValue(20.0, 1)
	if p > 0.001 {
		t.Errorf("expected small p for large x, got %f", p)
	}

	// Zero or negative x should give p=1
	p = approximateChiSquarePValue(0, 1)
	if p != 1.0 {
		t.Errorf("expected p=1 for x=0, got %f", p)
	}

	p = approximateChiSquarePValue(-5.0, 1)
	if p != 1.0 {
		t.Errorf("expected p=1 for negative x, got %f", p)
	}
}
