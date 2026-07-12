package runlog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompressAndEncode(t *testing.T) {
	testData := []byte("This is test data that should compress well because it has repetition repetition repetition")

	encoded, err := CompressAndEncode(testData)
	if err != nil {
		t.Fatalf("CompressAndEncode failed: %v", err)
	}

	if encoded == "" {
		t.Fatal("Encoded string is empty")
	}

	// Verify we can decode it back
	decoded, err := DecodeAndDecompress(encoded)
	if err != nil {
		t.Fatalf("DecodeAndDecompress failed: %v", err)
	}

	if string(decoded) != string(testData) {
		t.Errorf("Decoded data doesn't match original.\nExpected: %s\nGot: %s", testData, decoded)
	}
}

func TestCompressFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	testContent := "NONMEM output with lots of repeated text text text\n"
	err := os.WriteFile(testFile, []byte(testContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Compress the file
	encoded, err := CompressFile(testFile)
	if err != nil {
		t.Fatalf("CompressFile failed: %v", err)
	}

	// Verify we can decompress it
	decoded, err := DecodeAndDecompress(encoded)
	if err != nil {
		t.Fatalf("DecodeAndDecompress failed: %v", err)
	}

	if string(decoded) != testContent {
		t.Errorf("Decoded content doesn't match original.\nExpected: %s\nGot: %s", testContent, decoded)
	}
}

func TestShouldEmbed(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		expected bool
	}{
		{"Small file (1KB)", 1024, true},
		{"Medium file (25KB)", 25 * 1024, true},
		{"Threshold file (49KB)", 49 * 1024, true},
		{"Large file (51KB)", 51 * 1024, false},
		{"Very large file (1MB)", 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShouldEmbed(tt.size)
			if result != tt.expected {
				t.Errorf("ShouldEmbed(%d) = %v, expected %v", tt.size, result, tt.expected)
			}
		})
	}
}

func TestCompressionRatio(t *testing.T) {
	// Simulate typical NONMEM .lst file content with repetitive structure
	lstContent := `ITERATION NO.:    0    OBJECTIVE VALUE:   15294.0021442315
ITERATION NO.:    5    OBJECTIVE VALUE:   3740.54112996543
ITERATION NO.:   10    OBJECTIVE VALUE:   3133.05185122856
`
	// Repeat to simulate a real file
	fullContent := ""
	for i := 0; i < 100; i++ {
		fullContent += lstContent
	}

	original := []byte(fullContent)
	compressed, err := CompressAndEncode(original)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}

	compressionRatio := float64(len(compressed)) / float64(len(original))

	t.Logf("Original size: %d bytes", len(original))
	t.Logf("Compressed size: %d bytes", len(compressed))
	t.Logf("Compression ratio: %.2f%%", compressionRatio*100)

	// We expect at least 50% reduction for repetitive text
	if compressionRatio > 0.5 {
		t.Errorf("Compression ratio too high: %.2f%% (expected < 50%%)", compressionRatio*100)
	}
}
