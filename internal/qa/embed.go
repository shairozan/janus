package qa

import "embed"

// acopFS holds the embedded ACOP model and its data, materialized into a run
// directory for the OQ phase-2 functional run. The files are byte-identical
// copies of testdata/mock-nonmem/acop.{mod,csv} (go:embed cannot reach
// testdata/); a CI test guards that they stay in sync.
//
//go:embed acopfiles/acop.mod acopfiles/acop.csv
var acopFS embed.FS

// Embedded ACOP file paths within acopFS.
const (
	acopModelPath = "acopfiles/acop.mod"
	acopDataPath  = "acopfiles/acop.csv"
)

// acopModel returns the embedded ACOP control stream.
func acopModel() ([]byte, error) {
	return acopFS.ReadFile(acopModelPath)
}

// acopData returns the embedded ACOP dataset.
func acopData() ([]byte, error) {
	return acopFS.ReadFile(acopDataPath)
}
