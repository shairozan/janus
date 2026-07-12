package qa

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// normalizeEOL strips carriage returns so the comparison guards against content
// drift rather than line-ending differences, which vary by platform/checkout
// (git's autocrlf may materialize the fixture as CRLF while the embedded copy is
// LF). NONMEM treats these files as text, so EOL is not meaningful content.
func normalizeEOL(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// TestEmbeddedACOPMatchesFixtures guards that the embedded ACOP files stay
// byte-identical to the canonical testdata fixtures. go:embed cannot reach
// testdata/, so the package keeps its own copies; this test fails CI the moment
// they drift apart.
func TestEmbeddedACOPMatchesFixtures(t *testing.T) {
	cases := []struct {
		name     string
		embedded func() ([]byte, error)
		fixture  string
	}{
		{"acop.mod", acopModel, filepath.Join("..", "..", "testdata", "mock-nonmem", "acop.mod")},
		{"acop.csv", acopData, filepath.Join("..", "..", "testdata", "mock-nonmem", "acop.csv")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.embedded()
			if err != nil {
				t.Fatalf("reading embedded %s: %v", tc.name, err)
			}

			want, err := os.ReadFile(tc.fixture)
			if err != nil {
				t.Fatalf("reading fixture %s: %v", tc.fixture, err)
			}

			if !bytes.Equal(normalizeEOL(got), normalizeEOL(want)) {
				t.Fatalf("embedded %s (%d bytes) differs in content from fixture %s (%d bytes)",
					tc.name, len(got), tc.fixture, len(want))
			}
		})
	}
}
