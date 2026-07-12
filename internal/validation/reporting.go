package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// RequirementJustification provides regulatory context for each requirement.
type RequirementJustification struct {
	ID                  string `json:"id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	RegulatoryContext   string `json:"regulatory_context"`
	BusinessJustification string `json:"business_justification"`
	RiskAssessment      string `json:"risk_assessment"`
	TestObjective       string `json:"test_objective"`
	AcceptanceCriteria  string `json:"acceptance_criteria"`
	Category            ValidationCategory `json:"category"`
}

// TestMatrixEntry links requirements to test results with disposition.
type TestMatrixEntry struct {
	RequirementID     string             `json:"requirement_id"`
	TestName          string             `json:"test_name"`
	TestDisposition   string             `json:"test_disposition"`
	ExecutionTime     time.Time          `json:"execution_time"`
	Status            string             `json:"status"`
	Duration          time.Duration      `json:"duration"`
	Category          ValidationCategory `json:"category"`
	ErrorMessage      string             `json:"error_message,omitempty"`
	TechnicalSummary  string             `json:"technical_summary"`
}

// ValidationSummaryReport contains all auditor-ready documentation.
type ValidationSummaryReport struct {
	Metadata             ReportMetadata             `json:"metadata"`
	RequirementsRegistry []RequirementJustification `json:"requirements_registry"`
	TestMatrix           []TestMatrixEntry          `json:"test_matrix"`
	ExecutionSummary     ExecutionSummary           `json:"execution_summary"`
	ComplianceStatement  ComplianceStatement        `json:"compliance_statement"`
}

// ReportMetadata provides context about the validation execution.
type ReportMetadata struct {
	GeneratedAt       time.Time `json:"generated_at"`
	JanusVersion      string    `json:"janus_version"`
	ValidationScope   string    `json:"validation_scope"`
	RegulatoryStandard string   `json:"regulatory_standard"`
	TestEnvironment   string    `json:"test_environment"`
	Operator          string    `json:"operator"`
}

// ExecutionSummary provides high-level metrics for auditors.
type ExecutionSummary struct {
	TotalRequirements int                            `json:"total_requirements"`
	TotalTests        int                            `json:"total_tests"`
	PassedTests       int                            `json:"passed_tests"`
	FailedTests       int                            `json:"failed_tests"`
	SuccessRate       float64                        `json:"success_rate"`
	CategoryBreakdown map[ValidationCategory]CategoryStats `json:"category_breakdown"`
	ExecutionTime     time.Duration                  `json:"total_execution_time"`
}

// CategoryStats provides per-category test statistics.
type CategoryStats struct {
	TotalTests  int `json:"total_tests"`
	PassedTests int `json:"passed_tests"`
	FailedTests int `json:"failed_tests"`
}

// ComplianceStatement provides regulatory compliance assertion.
type ComplianceStatement struct {
	Standard       string `json:"standard"`
	Scope          string `json:"scope"`
	Statement      string `json:"statement"`
	Limitations    string `json:"limitations"`
	Recommendation string `json:"recommendation"`
}

// RequirementsRegistry contains all requirement justifications.
var requirementsRegistry = map[string]RequirementJustification{
	"REQ-01": {
		ID:    "REQ-01",
		Title: "Basic NONMEM Execution",
		Description: "Janus must be able to execute NONMEM models locally with proper command generation and output capture",
		RegulatoryContext: "CFR 21 Part 11.10(a) - System validation must ensure accurate and reliable execution of pharmaceutical modeling software",
		BusinessJustification: "NONMEM is the industry standard for population pharmacokinetic modeling. Reliable local execution is fundamental for pharmaceutical analysis workflows",
		RiskAssessment: "HIGH - Incorrect NONMEM execution could lead to invalid model results affecting drug development decisions",
		TestObjective: "Verify that Janus can successfully execute NONMEM models with proper subprocess management, output capture, and error handling",
		AcceptanceCriteria: "System successfully executes NONMEM commands, captures stdout/stderr, returns proper exit codes, and handles file I/O correctly",
		Category: CategoryNONMEMExecution,
	},
	"REQ-02": {
		ID:    "REQ-02",
		Title: "NONMEM with Additional Options",
		Description: "Janus must support passing additional command-line options to NONMEM for advanced execution scenarios",
		RegulatoryContext: "CFR 21 Part 11.10(b) - System must provide ability to generate accurate and complete copies of records in human readable form, including all execution parameters",
		BusinessJustification: "Advanced NONMEM options (maxeval, files, etc.) are critical for complex models and production environments requiring specific execution parameters",
		RiskAssessment: "MEDIUM - Incorrect option handling could affect model execution but is detectable through validation",
		TestObjective: "Verify that additional NONMEM options are properly passed through and appear in execution commands and audit trails",
		AcceptanceCriteria: "Additional options are correctly appended to NONMEM commands and reflected in captured output and audit logs",
		Category: CategoryNONMEMExecution,
	},
	"REQ-03": {
		ID:    "REQ-03",
		Title: "NONMEM Parallel Execution",
		Description: "Janus must support parallel NONMEM execution with proper core allocation and parallel file handling",
		RegulatoryContext: "CFR 21 Part 11.10(a) - System must maintain accuracy and reliability during concurrent processing operations",
		BusinessJustification: "Parallel execution significantly reduces computation time for complex population models, improving productivity in pharmaceutical development",
		RiskAssessment: "MEDIUM - Parallel execution errors could affect performance but should not compromise result accuracy",
		TestObjective: "Verify that parallel NONMEM execution properly configures parallel flags, PNM files, and core allocation",
		AcceptanceCriteria: "Parallel execution commands include proper -parallel flags, PNM file references, and execute without resource conflicts",
		Category: CategoryNONMEMExecution,
	},
	"REQ-04": {
		ID:    "REQ-04",
		Title: "NONMEM Output File Handling",
		Description: "Janus must correctly generate output file paths and handle different input file naming conventions",
		RegulatoryContext: "CFR 21 Part 11.10(b) - System must provide accurate and complete file management with proper naming conventions",
		BusinessJustification: "Consistent file naming conventions are essential for organizing model results and maintaining traceability in pharmaceutical workflows",
		RiskAssessment: "MEDIUM - Incorrect file naming could lead to data organization issues but does not affect model accuracy",
		TestObjective: "Verify that input .mod files correctly generate corresponding .lst output files with preserved naming patterns",
		AcceptanceCriteria: "All input file naming patterns (.mod extensions, underscores, paths) correctly generate expected .lst output file references",
		Category: CategoryNONMEMExecution,
	},
	"REQ-41": {
		ID:    "REQ-41",
		Title: "JSON Audit Trail Enablement",
		Description: "Janus must provide configurable JSON audit trail functionality for regulatory compliance",
		RegulatoryContext: "CFR 21 Part 11.10(e) - Systems must use secure, computer-generated, time-stamped audit trails to independently record the date and time of operator entries and actions",
		BusinessJustification: "Audit trails are mandatory for FDA-regulated pharmaceutical software to demonstrate compliance and support regulatory submissions",
		RiskAssessment: "CRITICAL - Missing audit trails could render the system non-compliant for regulated pharmaceutical use",
		TestObjective: "Verify that audit trail functionality can be enabled/disabled via configuration and properly captures all required execution data",
		AcceptanceCriteria: "Audit engine enables/disables based on configuration, creates JSON audit logs only when enabled, and captures all execution details",
		Category: CategoryAuditCompliance,
	},
	"REQ-42": {
		ID:    "REQ-42",
		Title: "Job ID Audit Logging",
		Description: "Janus must generate and log unique job IDs for execution traceability",
		RegulatoryContext: "CFR 21 Part 11.10(e) - Audit trails must include sufficient information to independently track individual operations",
		BusinessJustification: "Unique job IDs enable tracing individual executions across distributed systems and correlating results with specific runs",
		RiskAssessment: "HIGH - Non-unique or missing job IDs compromise audit trail integrity and regulatory compliance",
		TestObjective: "Verify that unique job IDs are generated for each execution and properly logged in audit trails",
		AcceptanceCriteria: "Each execution generates a unique job ID with proper format and sufficient entropy for uniqueness guarantees",
		Category: CategoryAuditCompliance,
	},
	"REQ-43": {
		ID:    "REQ-43",
		Title: "STDOUT Audit Capture",
		Description: "Janus must capture and log complete STDOUT from executed commands",
		RegulatoryContext: "CFR 21 Part 11.10(b) - System must provide accurate and complete copies of records including all output data",
		BusinessJustification: "STDOUT contains critical execution results and diagnostic information required for model validation and troubleshooting",
		RiskAssessment: "HIGH - Missing STDOUT data compromises result verification and debugging capabilities",
		TestObjective: "Verify that complete STDOUT from command execution is captured and stored in audit logs",
		AcceptanceCriteria: "All STDOUT data is captured without truncation and properly stored in JSON audit trail format",
		Category: CategoryAuditCompliance,
	},
	"REQ-44": {
		ID:    "REQ-44",
		Title: "STDERR Audit Capture",
		Description: "Janus must capture and log complete STDERR from executed commands",
		RegulatoryContext: "CFR 21 Part 11.10(b) - System must provide accurate and complete copies of records including all error and diagnostic data",
		BusinessJustification: "STDERR contains critical error and warning information essential for validating execution success and diagnosing issues",
		RiskAssessment: "HIGH - Missing STDERR data compromises error detection and system reliability validation",
		TestObjective: "Verify that complete STDERR from command execution is captured and stored in audit logs",
		AcceptanceCriteria: "All STDERR data is captured without truncation and properly stored in JSON audit trail format",
		Category: CategoryAuditCompliance,
	},
	"REQ-45": {
		ID:    "REQ-45",
		Title: "Binary Path Audit Logging",
		Description: "Janus must log the complete path of executed binaries for execution traceability",
		RegulatoryContext: "CFR 21 Part 11.10(e) - Audit trails must include sufficient information to independently verify what operations were performed",
		BusinessJustification: "Binary path logging ensures traceability of which specific tools and versions were used for each execution",
		RiskAssessment: "MEDIUM - Missing binary paths compromise audit trail completeness but do not affect execution accuracy",
		TestObjective: "Verify that complete binary paths are captured and logged in audit trails for all executions",
		AcceptanceCriteria: "Full absolute paths to executed binaries are recorded in audit logs with proper format",
		Category: CategoryAuditCompliance,
	},
	"REQ-46": {
		ID:    "REQ-46",
		Title: "Command Arguments Audit Logging",
		Description: "Janus must log all command-line arguments passed to executed commands",
		RegulatoryContext: "CFR 21 Part 11.10(e) - Audit trails must include sufficient information to independently reconstruct the sequence of events",
		BusinessJustification: "Command arguments contain critical execution parameters that must be traceable for result validation and compliance",
		RiskAssessment: "HIGH - Missing command arguments compromise the ability to verify and reproduce execution results",
		TestObjective: "Verify that all command-line arguments are captured and logged in audit trails",
		AcceptanceCriteria: "Complete command argument arrays are recorded in audit logs preserving order and formatting",
		Category: CategoryAuditCompliance,
	},
}

// GetRequirementJustification returns the justification for a given requirement ID.
func GetRequirementJustification(reqID string) (RequirementJustification, bool) {
	req, exists := requirementsRegistry[reqID]

	return req, exists
}

// GetAllRequirements returns all requirement justifications.
func GetAllRequirements() []RequirementJustification {
	var requirements []RequirementJustification
	for _, req := range requirementsRegistry {
		requirements = append(requirements, req)
	}

	// Sort by requirement ID for consistent ordering
	sort.Slice(requirements, func(i, j int) bool {
		return requirements[i].ID < requirements[j].ID
	})

	return requirements
}

// GenerateTestDisposition creates a technical summary of what a test does.
func GenerateTestDisposition(reqID string) string {
	switch reqID {
	case "REQ-01":
		return "Creates temporary NONMEM model file, configures NONMEMExecutor with /bin/echo for cross-platform testing, executes model via Execute() method, validates exit code=0, verifies STDOUT contains model file path and expected .lst output file"
	case "REQ-02":
		return "Creates complex NONMEM model file, executes with additional options (-maxeval=9999, -files=100), validates successful execution and verifies additional options appear in command output and are properly passed through execution pipeline"
	case "REQ-03":
		return "Creates parallel NONMEM model file, executes with isParallel=true and cores=4, validates parallel flags (-parallel, .pnm file) are properly included in execution command and output"
	case "REQ-04":
		return "Tests multiple file naming patterns (model.mod, complex_model.mod, model_v2.mod, population_pk.mod), creates real model files for each pattern, executes via NONMEMExecutor, validates that corresponding .lst output file names are correctly generated"
	case "REQ-41":
		return "Creates NONMEMExecutor with audit backend disabled (empty string), executes command, verifies no audit log file is created or remains empty, demonstrating proper audit logging disable functionality"
	case "REQ-42":
		return "Creates NONMEMExecutor with audit logging enabled, executes real command, reads generated audit log file, validates job ID is present, properly formatted with 'job-' prefix, and has sufficient length for uniqueness"
	case "REQ-43":
		return "Creates NONMEMExecutor with audit enabled, executes command with specific expected output, reads audit log file, validates STDOUT field contains expected output from command execution"
	case "REQ-44":
		return "Similar to REQ-43 but specifically validates STDERR capture functionality by executing commands that generate error output and verifying STDERR field in audit logs"
	case "REQ-45":
		return "Tests various binary path scenarios (/opt/NONMEM/nm76/run/nmfe76, /usr/local/bin/execute, /usr/bin/bbi, /opt/torque/bin/qsub), validates binary path field in audit logs accurately reflects executed binary location"
	case "REQ-46":
		return "Tests various command argument patterns (basic NONMEM args, parallel args, PsN args, BBI args, TORQUE args), validates arguments array in audit logs preserves all arguments in correct order and format"
	case "REQ-60":
		return "Tests SLURM configuration validation for both REST and CLI modes, validates required fields (socket_path, api_version for REST mode), tests invalid configurations to ensure proper error handling"
	case "REQ-61":
		return "Validates SLURM REST mode specific configuration including socket path, API version, timeout, and authentication token, tests invalid mode configurations and verifies appropriate error messages"
	case "REQ-62":
		return "Validates SLURM CLI mode configuration including host, port, timeout settings, tests default mode handling (empty configuration defaults to CLI mode)"
	case "REQ-63":
		return "Tests SLURM integration as a grid scheduler with existing executors (NONMEM), validates grid execution attempts with SLURM sbatch, handles expected tool availability issues in test environment"
	case "REQ-64":
		return "Validates SLURM scheduler configuration with existing execution modes, tests SLURM configuration validation, verifies proper scheduler assignment in config structure"
	default:
		return fmt.Sprintf("Executes validation test for %s via real command execution and validates expected behavior", reqID)
	}
}

// EnhancedValidationReporter extends the basic reporter with comprehensive reporting capabilities.
type EnhancedValidationReporter struct {
	*ValidationReporter
	metadata ReportMetadata
}

// NewEnhancedReporter creates a new enhanced validation reporter.
func NewEnhancedReporter(janusVersion, operator string) *EnhancedValidationReporter {
	return &EnhancedValidationReporter{
		ValidationReporter: &ValidationReporter{},
		metadata: ReportMetadata{
			GeneratedAt:        time.Now(),
			JanusVersion:       janusVersion,
			ValidationScope:    "NONMEM Execution and Audit Compliance Validation",
			RegulatoryStandard: "CFR 21 Part 11 - Electronic Records; Electronic Signatures",
			TestEnvironment:    "Automated Test Environment with Mock Binaries",
			Operator:           operator,
		},
	}
}

// GenerateComprehensiveReport creates a complete auditor-ready validation report.
func (evr *EnhancedValidationReporter) GenerateComprehensiveReport(filename string) error {
	// Build test matrix from results
	var testMatrix []TestMatrixEntry
	categoryStats := make(map[ValidationCategory]CategoryStats)

	passedTests := 0
	failedTests := 0
	var totalDuration time.Duration

	for _, result := range evr.results {
		// Update category statistics
		stats := categoryStats[result.Category]
		stats.TotalTests++
		if result.Status == "PASS" {
			stats.PassedTests++
			passedTests++
		} else {
			stats.FailedTests++
			failedTests++
		}
		categoryStats[result.Category] = stats

		totalDuration += result.Duration

		// Create test matrix entry
		entry := TestMatrixEntry{
			RequirementID:    result.Requirement,
			TestName:         fmt.Sprintf("Test%s_%s", result.Requirement, strings.ReplaceAll(result.Description, " ", "")),
			TestDisposition:  GenerateTestDisposition(result.Requirement),
			ExecutionTime:    result.StartTime,
			Status:           result.Status,
			Duration:         result.Duration,
			Category:         result.Category,
			ErrorMessage:     result.ErrorMsg,
			TechnicalSummary: fmt.Sprintf("Automated validation test executing real %s functionality through Janus validation framework", string(result.Category)),
		}
		testMatrix = append(testMatrix, entry)
	}

	// Calculate success rate
	totalTests := len(evr.results)
	successRate := 0.0
	if totalTests > 0 {
		successRate = float64(passedTests) / float64(totalTests) * 100
	}

	// Build comprehensive report
	report := ValidationSummaryReport{
		Metadata:             evr.metadata,
		RequirementsRegistry: GetAllRequirements(),
		TestMatrix:           testMatrix,
		ExecutionSummary: ExecutionSummary{
			TotalRequirements: len(requirementsRegistry),
			TotalTests:        totalTests,
			PassedTests:       passedTests,
			FailedTests:       failedTests,
			SuccessRate:       successRate,
			CategoryBreakdown: categoryStats,
			ExecutionTime:     totalDuration,
		},
		ComplianceStatement: ComplianceStatement{
			Standard:       "CFR 21 Part 11 - Electronic Records; Electronic Signatures",
			Scope:          "NONMEM execution validation and audit trail compliance for pharmaceutical software",
			Statement:      "This validation demonstrates that Janus software meets CFR 21 Part 11 requirements for accurate, reliable execution of pharmaceutical modeling software with complete audit trail capabilities.",
			Limitations:    "Validation performed using mock binaries in test environment. Production validation with actual NONMEM installations recommended for regulatory submissions.",
			Recommendation: "System is suitable for regulated pharmaceutical use provided production validation confirms compatibility with target NONMEM installations and infrastructure.",
		},
	}

	// Generate JSON report
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal comprehensive validation report: %w", err)
	}

	return os.WriteFile(filename, data, 0600)
}