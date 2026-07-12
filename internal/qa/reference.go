package qa

// Reference values for the OQ phase-2 functional run. The embedded ACOP model
// (acopfiles/acop.mod + acop.csv) has a known-good objective function value;
// OQ passes only when the user's configured execution path reproduces it within
// tolerance.
//
// ReferenceOFV is the "OBJECTIVE FUNCTION VALUE WITHOUT CONSTANT" from the
// canonical run in testdata/mock-nonmem/acop.lst.
const (
	ReferenceOFV = 2636.8457689964607

	// AbsTolerance and RelTolerance bound acceptable drift from ReferenceOFV.
	// The relative band (0.1% ≈ ±2.6 OFV units) absorbs cross-version
	// trailing-digit differences while still catching a genuinely wrong or
	// broken result; the absolute band covers the case where the reference is
	// near zero.
	AbsTolerance = 1.0
	RelTolerance = 1e-3
)
