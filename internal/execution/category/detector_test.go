package category

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data for various model types.
const (
	nonmemModelValid = `$PROBLEM Warfarin PK model
$DATA warfarin.csv IGNORE=@
$INPUT ID TIME AMT DV EVID MDV WT AGE SEX
$SUBROUTINE ADVAN2 TRANS2

$PK
  TVCL = THETA(1)

$ERROR
  Y = F

$THETA
  (0, 3.0)

$ESTIMATION METHOD=1
`

	nonmemModelMissingData = `$PROBLEM Warfarin PK model
; No dollar DATA or dollar INPUT directive

TVCL = THETA(1)
`

	monolixModelValid = `<DATAFILE>
[FILEINFO]
file = 'data.csv'
delimiter = comma

<MODEL>
[INDIVIDUAL]
input = {ka_pop, V_pop}

<FIT>
data = {y1}

<PARAMETER>
ka_pop = {value=1, method=MLE}
`

	monolixModelPartial = `<DATAFILE>
[FILEINFO]
file = 'data.csv'

Some other content here
`

	stanModelValid = `data {
  int<lower=0> N;
  vector[N] y;
}

parameters {
  real mu;
  real<lower=0> sigma;
}

model {
  y ~ normal(mu, sigma);
}
`

	stanModelMissingBlocks = `data {
  int<lower=0> N;
}

parameters {
  real mu;
}

// Missing model block
`

	torstenModelValid = `data {
  int<lower=1> nt;
  real<lower=0> amt[nt];
  real<lower=0> time[nt];
}

parameters {
  real<lower=0> CL;
  real<lower=0> V;
  real<lower=0> ka;
}

transformed parameters {
  real<lower=0> theta[3];
  theta[1] = CL;
  theta[2] = V;
  theta[3] = ka;

  // Torsten-specific function
  matrix[nt, 2] x = PKModelOneCpt(time, amt, theta);
}

model {
  CL ~ lognormal(log(10), 0.25);
}
`

	torstenModelWithPMX = `data {
  int N;
}

parameters {
  real CL;
}

model {
  // Using Torsten ODE solver
  array[N] vector[2] y = pmx_solve_bdf(ode_system, y0, t0, ts, theta);
}
`

	unknownModel = `This is not a valid model file
Just some random text
No recognizable markers
`
)

func TestNewDetector(t *testing.T) {
	t.Run("creates detector with rules", func(t *testing.T) {
		detector := NewDetector()

		assert.NotNil(t, detector)
		assert.Len(t, detector.rules, 4, "Should have 4 detection rules")

		// Verify rules are in correct priority order
		assert.Equal(t, 1, detector.rules[0].Priority, "Monolix should be priority 1")
		assert.Equal(t, 2, detector.rules[1].Priority, "NONMEM should be priority 2")
		assert.Equal(t, 3, detector.rules[2].Priority, "Torsten should be priority 3")
		assert.Equal(t, 4, detector.rules[3].Priority, "Stan should be priority 4")

		// Verify categories
		assert.Equal(t, CategoryMonolix, detector.rules[0].Category)
		assert.Equal(t, CategoryNONMEM, detector.rules[1].Category)
		assert.Equal(t, CategoryTorsten, detector.rules[2].Category)
		assert.Equal(t, CategoryStan, detector.rules[3].Category)
	})
}

func TestDetectNONMEM(t *testing.T) {
	detector := NewDetector()

	t.Run("detects NONMEM .mod file with valid content", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		err := os.WriteFile(modelPath, []byte(nonmemModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryNONMEM, category)
	})

	t.Run("detects NONMEM .ctl file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.ctl")
		err := os.WriteFile(modelPath, []byte(nonmemModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryNONMEM, category)
	})

	t.Run("detects NONMEM .nmctl file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.nmctl")
		err := os.WriteFile(modelPath, []byte(nonmemModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryNONMEM, category)
	})

	t.Run("returns Unknown for NONMEM file missing $DATA", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		err := os.WriteFile(modelPath, []byte(nonmemModelMissingData), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryUnknown, category)
	})

	t.Run("case insensitive detection", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.mod")
		content := `$problem test model
$data data.csv
$input ID TIME`
		err := os.WriteFile(modelPath, []byte(content), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryNONMEM, category)
	})
}

func TestDetectMonolix(t *testing.T) {
	detector := NewDetector()

	t.Run("detects Monolix .mlxtran file with valid content", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "project.mlxtran")
		err := os.WriteFile(modelPath, []byte(monolixModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryMonolix, category)
	})

	t.Run("returns Unknown for .mlxtran with insufficient markers", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "project.mlxtran")
		err := os.WriteFile(modelPath, []byte(monolixModelPartial), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryUnknown, category)
	})
}

func TestDetectStan(t *testing.T) {
	detector := NewDetector()

	t.Run("detects Stan .stan file with all required blocks", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.stan")
		err := os.WriteFile(modelPath, []byte(stanModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryStan, category)
	})

	t.Run("returns Unknown for Stan file missing blocks", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.stan")
		err := os.WriteFile(modelPath, []byte(stanModelMissingBlocks), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryUnknown, category)
	})
}

func TestDetectTorsten(t *testing.T) {
	detector := NewDetector()

	t.Run("detects Torsten with PKModelOneCpt", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "pk_model.stan")
		err := os.WriteFile(modelPath, []byte(torstenModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryTorsten, category)
	})

	t.Run("detects Torsten with pmx_solve_bdf", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "pk_model.stan")
		err := os.WriteFile(modelPath, []byte(torstenModelWithPMX), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryTorsten, category)
	})

	t.Run("Torsten takes priority over Stan", func(t *testing.T) {
		// This test verifies that Torsten is detected before Stan
		// even though the file has valid Stan blocks
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.stan")
		err := os.WriteFile(modelPath, []byte(torstenModelValid), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryTorsten, category, "Torsten should be detected before Stan")
	})
}

func TestDetectUnknown(t *testing.T) {
	detector := NewDetector()

	t.Run("returns Unknown for unrecognized file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "model.xyz")
		err := os.WriteFile(modelPath, []byte(unknownModel), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryUnknown, category)
	})

	t.Run("returns Unknown for empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "empty.txt")
		err := os.WriteFile(modelPath, []byte(""), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryUnknown, category)
	})

	t.Run("returns Unknown for malformed NONMEM file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "malformed.mod")
		content := `Some text
$PROBLEM but no data directive
Just incomplete
`
		err := os.WriteFile(modelPath, []byte(content), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryUnknown, category)
	})
}

func TestDetectErrors(t *testing.T) {
	detector := NewDetector()

	t.Run("returns error for non-existent file", func(t *testing.T) {
		category, err := detector.Detect("/path/to/nonexistent/file.mod")

		assert.Error(t, err)
		assert.Equal(t, CategoryUnknown, category)
		assert.Contains(t, err.Error(), "failed to read model file")
	})
}

func TestDetectionFunctions(t *testing.T) {
	t.Run("isMonolixFile", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected bool
		}{
			{
				name:     "valid with 3 markers",
				content:  monolixModelValid,
				expected: true,
			},
			{
				name:     "valid with exactly 2 markers",
				content:  "<DATAFILE>\nsome content\n<MODEL>\nmore content",
				expected: true,
			},
			{
				name:     "invalid with only 1 marker",
				content:  monolixModelPartial,
				expected: false,
			},
			{
				name:     "invalid with no markers",
				content:  "random content",
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isMonolixFile("test.mlxtran", []byte(tt.content))
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("isNONMEMFile", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected bool
		}{
			{
				name:     "valid with $PROBLEM and $DATA",
				content:  nonmemModelValid,
				expected: true,
			},
			{
				name:     "valid with $PROB and $INPUT",
				content:  "$PROB test\n$INPUT ID TIME",
				expected: true,
			},
			{
				name:     "valid case insensitive",
				content:  "$problem test\n$data data.csv",
				expected: true,
			},
			{
				name:     "invalid missing $DATA",
				content:  nonmemModelMissingData,
				expected: false,
			},
			{
				name:     "invalid missing $PROBLEM",
				content:  "$DATA data.csv\n$INPUT ID",
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isNONMEMFile("test.mod", []byte(tt.content))
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("isStanFile", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected bool
		}{
			{
				name:     "valid with all blocks",
				content:  stanModelValid,
				expected: true,
			},
			{
				name:     "invalid missing model block",
				content:  stanModelMissingBlocks,
				expected: false,
			},
			{
				name:     "invalid missing data block",
				content:  "parameters { real x; }\nmodel { x ~ normal(0,1); }",
				expected: false,
			},
			{
				name:     "invalid missing parameters block",
				content:  "data { int N; }\nmodel { }",
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isStanFile("test.stan", []byte(tt.content))
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("isTorstenFile", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected bool
		}{
			{
				name:     "valid with PKModelOneCpt",
				content:  torstenModelValid,
				expected: true,
			},
			{
				name:     "valid with pmx_solve_bdf",
				content:  torstenModelWithPMX,
				expected: true,
			},
			{
				name:     "invalid - stan without torsten functions",
				content:  stanModelValid,
				expected: false,
			},
			{
				name:     "invalid - not even stan",
				content:  "random content",
				expected: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := isTorstenFile("test.stan", []byte(tt.content))
				assert.Equal(t, tt.expected, result)
			})
		}
	})
}

func TestEdgeCases(t *testing.T) {
	detector := NewDetector()

	t.Run("comments in NONMEM file", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "commented.mod")
		content := `; This is a comment
$PROBLEM Test model ; inline comment
$DATA data.csv IGNORE=@ ; another comment
$INPUT ID TIME
`
		err := os.WriteFile(modelPath, []byte(content), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryNONMEM, category)
	})

	t.Run("whitespace variations in Stan", func(t *testing.T) {
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "whitespace.stan")
		// Note: Using string concatenation to avoid tabs from code formatting
		content := "data   {\n" +
			"  int N;\n" +
			"}\n" +
			"\n" +
			"parameters {\n" +
			"  real x;\n" +
			"}\n" +
			"\n" +
			"model    {\n" +
			"  x ~ normal(0, 1);\n" +
			"}\n"
		err := os.WriteFile(modelPath, []byte(content), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		assert.NoError(t, err)
		assert.Equal(t, CategoryStan, category, "Should detect Stan with variable whitespace")
	})

	t.Run("mixed case in Monolix tags", func(t *testing.T) {
		// Note: Our current implementation is case-sensitive
		// This test documents that behavior
		tmpDir := t.TempDir()
		modelPath := filepath.Join(tmpDir, "mixedcase.mlxtran")
		content := `<datafile>
content
<Model>
more content
<FIT>
even more
`
		err := os.WriteFile(modelPath, []byte(content), 0644)
		require.NoError(t, err)

		category, err := detector.Detect(modelPath)

		// Current implementation is case-sensitive for Monolix
		// This would return Unknown unless we add case-insensitive matching
		assert.NoError(t, err)
		// Documenting current behavior - adjust if we add case-insensitive support
		assert.Equal(t, CategoryUnknown, category)
	})
}
