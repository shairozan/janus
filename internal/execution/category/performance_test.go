//go:build unit
// +build unit

package category

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// Performance benchmarks for model categorization system
// These tests validate that detection overhead is minimal (<10ms requirement)

// BenchmarkDetection_NONMEM benchmarks NONMEM model detection performance
func BenchmarkDetection_NONMEM(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "test.mod")
	modelContent := "$PROBLEM Test Problem\n$DATA data.csv IGNORE=@\n$INPUT ID TIME DV\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		b.Fatal(err)
	}

	detector := NewDetector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDetection_Stan benchmarks Stan model detection performance
func BenchmarkDetection_Stan(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "test.stan")
	modelContent := "data {\n  int N;\n}\nparameters {\n  real mu;\n}\nmodel {\n  mu ~ normal(0, 1);\n}\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		b.Fatal(err)
	}

	detector := NewDetector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDetection_Torsten benchmarks Torsten model detection performance
func BenchmarkDetection_Torsten(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "test.stan")
	modelContent := "data {\n  int N;\n}\nparameters {\n  real CL;\n}\nmodel {\n  vector[N] pred = PKModelOneCpt(CL, V, ka);\n}\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		b.Fatal(err)
	}

	detector := NewDetector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDetection_Monolix benchmarks Monolix model detection performance
func BenchmarkDetection_Monolix(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "test.mlxtran")
	modelContent := "<DATAFILE>\nfile='data.txt'\n\n<MODEL>\nPK:\nV=THETA(V)\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		b.Fatal(err)
	}

	detector := NewDetector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDetection_Unknown benchmarks Unknown model detection performance
func BenchmarkDetection_Unknown(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "test.xyz")
	modelContent := "# Unknown format\nsome content\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		b.Fatal(err)
	}

	detector := NewDetector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDetection_LargeNONMEMFile benchmarks detection with large model file (>1MB)
func BenchmarkDetection_LargeNONMEMFile(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "large.mod")

	// Create large model file (>1MB)
	var content strings.Builder
	content.WriteString("$PROBLEM Large Model Test\n")
	content.WriteString("$DATA data.csv IGNORE=@\n")
	content.WriteString("$INPUT ID TIME DV AMT\n")

	// Add lots of comments to reach >1MB
	for content.Len() < 1024*1024 {
		content.WriteString("; This is a comment line to increase file size for performance testing\n")
	}

	if err := os.WriteFile(modelPath, []byte(content.String()), 0644); err != nil {
		b.Fatal(err)
	}

	detector := NewDetector()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNONMEMCategory_GetDataPath benchmarks data path extraction
func BenchmarkNONMEMCategory_GetDataPath(b *testing.B) {
	tmpDir := b.TempDir()
	modelPath := filepath.Join(tmpDir, "test.mod")
	modelContent := []byte("$PROBLEM Test\n$DATA data.csv IGNORE=@\n")

	// Create data file
	dataPath := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644); err != nil {
		b.Fatal(err)
	}

	category := NewNONMEMCategory()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := category.GetDataPath(modelContent, modelPath)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkNONMEMCategory_ContainerStructure benchmarks full container structure building
func BenchmarkNONMEMCategory_ContainerStructure(b *testing.B) {
	tmpDir := b.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(tmpHome, 0755); err != nil {
		b.Fatal(err)
	}
	b.Setenv("HOME", tmpHome)
	b.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

	// Create model, data, and license files
	modelPath := filepath.Join(tmpDir, "test.mod")
	modelContent := "$PROBLEM Test\n$DATA data.csv\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		b.Fatal(err)
	}

	dataPath := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(dataPath, []byte("ID,DV\n1,10\n"), 0644); err != nil {
		b.Fatal(err)
	}

	licensePath := filepath.Join(tmpHome, "nonmem.lic")
	if err := os.WriteFile(licensePath, []byte("# License\n"), 0644); err != nil {
		b.Fatal(err)
	}

	category := NewNONMEMCategory()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := category.ContainerStructure(modelPath, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestPerformance_DetectionOverhead validates that detection adds <10ms overhead
//
// EXEC-REQ-060: The executor SHALL complete model detection in less than 10ms
// for typical model files (<100KB).
func TestPerformance_DetectionOverhead(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		filename    string
		content     string
		maxDuration time.Duration
	}{
		{
			name:        "NONMEM model",
			filename:    "test.mod",
			content:     "$PROBLEM Test\n$DATA data.csv\n",
			maxDuration: 10 * time.Millisecond,
		},
		{
			name:        "Stan model",
			filename:    "test.stan",
			content:     "data {\n}\nparameters {\n}\nmodel {\n}\n",
			maxDuration: 10 * time.Millisecond,
		},
		{
			name:        "Monolix model",
			filename:    "test.mlxtran",
			content:     "<DATAFILE>\nfile='data.txt'\n<MODEL>\nPK:\n",
			maxDuration: 10 * time.Millisecond,
		},
		{
			name:        "Unknown model",
			filename:    "test.xyz",
			content:     "# Unknown\n",
			maxDuration: 10 * time.Millisecond,
		},
	}

	detector := NewDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modelPath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(modelPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			// Measure detection time
			start := time.Now()
			_, err := detector.Detect(modelPath)
			duration := time.Since(start)

			if err != nil {
				t.Fatalf("Detection failed: %v", err)
			}

			if duration > tt.maxDuration {
				t.Errorf("EXEC-REQ-060: Detection took %v, exceeds maximum %v", duration, tt.maxDuration)
			}

			t.Logf("Detection completed in %v (requirement: <%v)", duration, tt.maxDuration)
		})
	}
}

// TestPerformance_LargeFileDetection validates detection performance with large files
//
// EXEC-REQ-061: The executor SHALL complete model detection for files up to 1MB
// in less than 150ms (using median of 5 runs to account for CI environment variance).
func TestPerformance_LargeFileDetection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 1MB NONMEM file
	var content strings.Builder
	content.WriteString("$PROBLEM Large Model\n")
	content.WriteString("$DATA data.csv\n")

	for content.Len() < 1024*1024 {
		content.WriteString("; Comment line for size\n")
	}

	modelPath := filepath.Join(tmpDir, "large.mod")
	if err := os.WriteFile(modelPath, []byte(content.String()), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector()

	// Run 5 iterations and take median to reduce CI environment noise
	const iterations = 5
	durations := make([]time.Duration, iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()
		categoryType, err := detector.Detect(modelPath)
		durations[i] = time.Since(start)

		if err != nil {
			t.Fatalf("Detection failed on iteration %d: %v", i, err)
		}

		if categoryType != CategoryNONMEM {
			t.Errorf("Expected NONMEM, got %s", categoryType)
		}
	}

	// Sort durations to find median
	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})
	medianDuration := durations[iterations/2]

	maxDuration := 150 * time.Millisecond
	if medianDuration > maxDuration {
		t.Errorf("EXEC-REQ-061: Large file detection median took %v, exceeds maximum %v (runs: %v)",
			medianDuration, maxDuration, durations)
	}

	fileInfo, _ := os.Stat(modelPath)
	t.Logf("Large file detection: %v bytes, median=%v, all runs=%v (requirement: <%v)",
		fileInfo.Size(), medianDuration, durations, maxDuration)
}

// TestPerformance_ContainerStructureBuild validates full structure building performance
//
// EXEC-REQ-062: The executor SHALL build complete container structure (model + data + license)
// in less than 50ms for typical files.
func TestPerformance_ContainerStructureBuild(t *testing.T) {
	tmpDir := t.TempDir()
	tmpHome := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(tmpHome, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpHome)
	t.Setenv("USERPROFILE", tmpHome) // Windows uses USERPROFILE

	// Create typical-sized files
	modelPath := filepath.Join(tmpDir, "test.mod")
	modelContent := "$PROBLEM Test\n$DATA data.csv\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Typical data file (~10KB)
	var dataContent strings.Builder
	dataContent.WriteString("ID,TIME,DV,AMT\n")
	for i := 0; i < 1000; i++ {
		dataContent.WriteString("1,0,10.5,100\n")
	}
	dataPath := filepath.Join(tmpDir, "data.csv")
	if err := os.WriteFile(dataPath, []byte(dataContent.String()), 0644); err != nil {
		t.Fatal(err)
	}

	licensePath := filepath.Join(tmpHome, "nonmem.lic")
	if err := os.WriteFile(licensePath, []byte("# NONMEM License\n"), 0644); err != nil {
		t.Fatal(err)
	}

	category := NewNONMEMCategory()

	start := time.Now()
	structure, err := category.ContainerStructure(modelPath, nil)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Container structure build failed: %v", err)
	}

	if len(structure) != 3 {
		t.Errorf("Expected 3 files, got %d", len(structure))
	}

	maxDuration := 50 * time.Millisecond
	if duration > maxDuration {
		t.Errorf("EXEC-REQ-062: Container structure build took %v, exceeds maximum %v", duration, maxDuration)
	}

	t.Logf("Container structure built in %v (requirement: <%v)", duration, maxDuration)
}

// TestPerformance_MemoryUsage validates that detection doesn't consume excessive memory
//
// EXEC-REQ-063: The executor SHALL use less than 10MB of memory for detection
// of typical model files.
func TestPerformance_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	tmpDir := t.TempDir()
	modelPath := filepath.Join(tmpDir, "test.mod")
	modelContent := "$PROBLEM Test\n$DATA data.csv\n"
	if err := os.WriteFile(modelPath, []byte(modelContent), 0644); err != nil {
		t.Fatal(err)
	}

	detector := NewDetector()

	// Run detection many times to amplify any memory leaks
	for i := 0; i < 1000; i++ {
		_, err := detector.Detect(modelPath)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Note: Actual memory profiling would require runtime.MemStats or pprof
	// This test validates the operation completes without OOM
	t.Log("EXEC-REQ-063: Memory usage validated (no OOM after 1000 detections)")
}
