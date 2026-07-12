package validation

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

// ValidationCategory represents different categories of validation tests.
type ValidationCategory string

const (
	CategoryNONMEMExecution  ValidationCategory = "NONMEM_EXECUTION"
	CategoryPSNIntegration   ValidationCategory = "PSN_INTEGRATION"
	CategoryBBIIntegration   ValidationCategory = "BBI_INTEGRATION"
	CategoryGridSystem       ValidationCategory = "GRID_SYSTEM"
	CategoryGridSystems      ValidationCategory = "GRID_SYSTEMS"
	CategoryConfiguration    ValidationCategory = "CONFIGURATION"
	CategoryUserInterface    ValidationCategory = "USER_INTERFACE"
	CategoryFileManagement   ValidationCategory = "FILE_MANAGEMENT"
	CategoryAuditCompliance  ValidationCategory = "AUDIT_COMPLIANCE"
	CategoryErrorHandling    ValidationCategory = "ERROR_HANDLING"
	CategoryVersionBuild     ValidationCategory = "VERSION_BUILD"
	CategoryTestFramework    ValidationCategory = "TEST_FRAMEWORK"
	CategorySLURMIntegration ValidationCategory = "SLURM_INTEGRATION"
)

// ValidationResult represents the result of a validation test.
type ValidationResult struct {
	Requirement string             `json:"requirement"`
	Description string             `json:"description"`
	Category    ValidationCategory `json:"category"`
	Status      string             `json:"status"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	Duration    time.Duration      `json:"duration"`
	ErrorMsg    string             `json:"error_msg,omitempty"`
}

// ValidationTest represents a single validation test case.
type ValidationTest struct {
	Requirement string
	Description string
	Category    ValidationCategory
	TestFunc    func(t *testing.T)
}

// ValidationReporter collects and reports validation results.
type ValidationReporter struct {
	results []ValidationResult
}

var globalReporter = &ValidationReporter{}

// Run executes a validation test with proper logging and result collection.
func (vt ValidationTest) Run(t *testing.T) {
	t.Helper()

	startTime := time.Now()

	// Log test start
	t.Logf("VALIDATION_START: REQ=%s DESC=%s CATEGORY=%s",
		vt.Requirement, vt.Description, vt.Category)

	result := ValidationResult{
		Requirement: vt.Requirement,
		Description: vt.Description,
		Category:    vt.Category,
		StartTime:   startTime,
		Status:      "RUNNING",
	}

	// Capture any panics or test failures
	defer func() {
		if r := recover(); r != nil {
			result.Status = "PANIC"
			result.ErrorMsg = fmt.Sprintf("Test panicked: %v", r)
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			globalReporter.addResult(result)
			t.Errorf("VALIDATION_PANIC: REQ=%s ERROR=%s", vt.Requirement, result.ErrorMsg)
			panic(r)
		}
	}()

	// Run the actual test
	vt.TestFunc(t)

	// Check if test failed by examining the testing state
	testPassed := !t.Failed()

	// Record results
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if testPassed {
		result.Status = "PASS"
		t.Logf("VALIDATION_PASS: REQ=%s DURATION=%v", vt.Requirement, result.Duration)
	} else {
		result.Status = "FAIL"
		result.ErrorMsg = "Test assertions failed"
		t.Logf("VALIDATION_FAIL: REQ=%s DURATION=%v", vt.Requirement, result.Duration)
	}

	globalReporter.addResult(result)
}

// addResult adds a validation result to the reporter.
func (vr *ValidationReporter) addResult(result ValidationResult) {
	vr.results = append(vr.results, result)
}

// GetResults returns all collected validation results.
func (vr *ValidationReporter) GetResults() []ValidationResult {
	return vr.results
}

// GenerateReport generates a validation report in JSON format.
func (vr *ValidationReporter) GenerateReport(filename string) error {
	data, err := json.MarshalIndent(vr.results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal validation results: %w", err)
	}

	return os.WriteFile(filename, data, 0600)
}

// PrintSummary prints a summary of validation results.
func (vr *ValidationReporter) PrintSummary() {
	totalTests := len(vr.results)
	passedTests := 0
	failedTests := 0

	categoryStats := make(map[ValidationCategory]map[string]int)

	for _, result := range vr.results {
		if categoryStats[result.Category] == nil {
			categoryStats[result.Category] = make(map[string]int)
		}

		categoryStats[result.Category][result.Status]++

		switch result.Status {
		case "PASS":
			passedTests++
		case "FAIL", "PANIC":
			failedTests++
		}
	}

	log.Printf("\n=== VALIDATION SUMMARY ===\n")
	log.Printf("Total Tests: %d\n", totalTests)
	log.Printf("Passed: %d\n", passedTests)
	log.Printf("Failed: %d\n", failedTests)
	log.Printf("Success Rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)

	log.Printf("\n=== BY CATEGORY ===\n")
	for category, stats := range categoryStats {
		log.Printf("%s:\n", category)
		for status, count := range stats {
			log.Printf("  %s: %d\n", status, count)
		}
	}

	if failedTests > 0 {
		log.Printf("\n=== FAILED TESTS ===\n")
		for _, result := range vr.results {
			if result.Status == "FAIL" || result.Status == "PANIC" {
				log.Printf("  %s: %s - %s\n", result.Requirement, result.Description, result.ErrorMsg)
			}
		}
	}
}

// GetGlobalReporter returns the global validation reporter instance.
func GetGlobalReporter() *ValidationReporter {
	return globalReporter
}

// ResetGlobalReporter clears all results from the global reporter.
func ResetGlobalReporter() {
	globalReporter.results = nil
}

// GenerateAuditorReport generates a comprehensive report suitable for regulatory auditors.
func (vr *ValidationReporter) GenerateAuditorReport(filename, janusVersion, operator string) error {
	enhancedReporter := &EnhancedValidationReporter{
		ValidationReporter: vr,
	}

	enhancedReporter.metadata = ReportMetadata{
		GeneratedAt:        time.Now(),
		JanusVersion:       janusVersion,
		ValidationScope:    "NONMEM Execution and Audit Compliance Validation",
		RegulatoryStandard: "CFR 21 Part 11 - Electronic Records; Electronic Signatures",
		TestEnvironment:    "Automated Test Environment with Mock Binaries",
		Operator:           operator,
	}

	return enhancedReporter.GenerateComprehensiveReport(filename)
}
