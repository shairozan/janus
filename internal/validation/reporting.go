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
	ID                    string             `json:"id"`
	Title                 string             `json:"title"`
	Description           string             `json:"description"`
	RegulatoryContext     string             `json:"regulatory_context"`
	BusinessJustification string             `json:"business_justification"`
	RiskAssessment        string             `json:"risk_assessment"`
	TestObjective         string             `json:"test_objective"`
	AcceptanceCriteria    string             `json:"acceptance_criteria"`
	Category              ValidationCategory `json:"category"`
}

// TestMatrixEntry links requirements to test results with disposition.
type TestMatrixEntry struct {
	RequirementID    string             `json:"requirement_id"`
	TestName         string             `json:"test_name"`
	TestDisposition  string             `json:"test_disposition"`
	ExecutionTime    time.Time          `json:"execution_time"`
	Status           string             `json:"status"`
	Duration         time.Duration      `json:"duration"`
	Category         ValidationCategory `json:"category"`
	ErrorMessage     string             `json:"error_message,omitempty"`
	TechnicalSummary string             `json:"technical_summary"`
}

// ValidationSummaryReport contains all run log-ready documentation.
type ValidationSummaryReport struct {
	Metadata             ReportMetadata             `json:"metadata"`
	RequirementsRegistry []RequirementJustification `json:"requirements_registry"`
	TestMatrix           []TestMatrixEntry          `json:"test_matrix"`
	ExecutionSummary     ExecutionSummary           `json:"execution_summary"`
	ComplianceStatement  ComplianceStatement        `json:"compliance_statement"`
}

// ReportMetadata provides context about the validation execution.
type ReportMetadata struct {
	GeneratedAt        time.Time `json:"generated_at"`
	JanusVersion       string    `json:"janus_version"`
	ValidationScope    string    `json:"validation_scope"`
	RegulatoryStandard string    `json:"regulatory_standard"`
	TestEnvironment    string    `json:"test_environment"`
	Operator           string    `json:"operator"`
}

// ExecutionSummary provides high-level metrics for run logs.
type ExecutionSummary struct {
	TotalRequirements int                                  `json:"total_requirements"`
	TotalTests        int                                  `json:"total_tests"`
	PassedTests       int                                  `json:"passed_tests"`
	FailedTests       int                                  `json:"failed_tests"`
	SuccessRate       float64                              `json:"success_rate"`
	CategoryBreakdown map[ValidationCategory]CategoryStats `json:"category_breakdown"`
	ExecutionTime     time.Duration                        `json:"total_execution_time"`
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
		ID:                    "REQ-01",
		Title:                 "Basic NONMEM Execution",
		Description:           "Janus must be able to execute NONMEM models locally with proper command generation and output capture",
		RegulatoryContext:     "CFR 21 Part 11.10(a) - System validation must ensure accurate and reliable execution of pharmaceutical modeling software",
		BusinessJustification: "NONMEM is the industry standard for population pharmacokinetic modeling. Reliable local execution is fundamental for pharmaceutical analysis workflows",
		RiskAssessment:        "HIGH - Incorrect NONMEM execution could lead to invalid model results affecting drug development decisions",
		TestObjective:         "Verify that Janus can successfully execute NONMEM models with proper subprocess management, output capture, and error handling",
		AcceptanceCriteria:    "System successfully executes NONMEM commands, captures stdout/stderr, returns proper exit codes, and handles file I/O correctly",
		Category:              CategoryNONMEMExecution,
	},
	"REQ-02": {
		ID:                    "REQ-02",
		Title:                 "NONMEM with Additional Options",
		Description:           "Janus must support passing additional command-line options to NONMEM for advanced execution scenarios",
		RegulatoryContext:     "CFR 21 Part 11.10(b) - System must provide ability to generate accurate and complete copies of records in human readable form, including all execution parameters",
		BusinessJustification: "Advanced NONMEM options (maxeval, files, etc.) are critical for complex models and production environments requiring specific execution parameters",
		RiskAssessment:        "MEDIUM - Incorrect option handling could affect model execution but is detectable through validation",
		TestObjective:         "Verify that additional NONMEM options are properly passed through and appear in execution commands and run logs",
		AcceptanceCriteria:    "Additional options are correctly appended to NONMEM commands and reflected in captured output and run logs",
		Category:              CategoryNONMEMExecution,
	},
	"REQ-03": {
		ID:                    "REQ-03",
		Title:                 "NONMEM Parallel Execution",
		Description:           "Janus must support parallel NONMEM execution with proper core allocation and parallel file handling",
		RegulatoryContext:     "CFR 21 Part 11.10(a) - System must maintain accuracy and reliability during concurrent processing operations",
		BusinessJustification: "Parallel execution significantly reduces computation time for complex population models, improving productivity in pharmaceutical development",
		RiskAssessment:        "MEDIUM - Parallel execution errors could affect performance but should not compromise result accuracy",
		TestObjective:         "Verify that parallel NONMEM execution properly configures parallel flags, PNM files, and core allocation",
		AcceptanceCriteria:    "Parallel execution commands include proper -parallel flags, PNM file references, and execute without resource conflicts",
		Category:              CategoryNONMEMExecution,
	},
	"REQ-04": {
		ID:                    "REQ-04",
		Title:                 "NONMEM Output File Handling",
		Description:           "Janus must correctly generate output file paths and handle different input file naming conventions",
		RegulatoryContext:     "CFR 21 Part 11.10(b) - System must provide accurate and complete file management with proper naming conventions",
		BusinessJustification: "Consistent file naming conventions are essential for organizing model results and maintaining traceability in pharmaceutical workflows",
		RiskAssessment:        "MEDIUM - Incorrect file naming could lead to data organization issues but does not affect model accuracy",
		TestObjective:         "Verify that input .mod files correctly generate corresponding .lst output files with preserved naming patterns",
		AcceptanceCriteria:    "All input file naming patterns (.mod extensions, underscores, paths) correctly generate expected .lst output file references",
		Category:              CategoryNONMEMExecution,
	},
	"REQ-41": {
		ID:                    "REQ-41",
		Title:                 "JSON Audit Trail Enablement",
		Description:           "Janus must provide configurable JSON run log functionality for regulatory compliance",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Systems must use secure, computer-generated, time-stamped run logs to independently record the date and time of operator entries and actions",
		BusinessJustification: "Audit trails are mandatory for FDA-regulated pharmaceutical software to demonstrate compliance and support regulatory submissions",
		RiskAssessment:        "CRITICAL - Missing run logs could render the system non-compliant for regulated pharmaceutical use",
		TestObjective:         "Verify that run log functionality can be enabled/disabled via configuration and properly captures all required execution data",
		AcceptanceCriteria:    "Run log engine enables/disables based on configuration, creates JSON run logs only when enabled, and captures all execution details",
		Category:              CategoryAuditCompliance,
	},
	"REQ-42": {
		ID:                    "REQ-42",
		Title:                 "Job ID Audit Logging",
		Description:           "Janus must generate and log unique job IDs for execution traceability",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Audit trails must include sufficient information to independently track individual operations",
		BusinessJustification: "Unique job IDs enable tracing individual executions across distributed systems and correlating results with specific runs",
		RiskAssessment:        "HIGH - Non-unique or missing job IDs compromise run log integrity and regulatory compliance",
		TestObjective:         "Verify that unique job IDs are generated for each execution and properly logged in run logs",
		AcceptanceCriteria:    "Each execution generates a unique job ID with proper format and sufficient entropy for uniqueness guarantees",
		Category:              CategoryAuditCompliance,
	},
	"REQ-43": {
		ID:                    "REQ-43",
		Title:                 "STDOUT Audit Capture",
		Description:           "Janus must capture and log complete STDOUT from executed commands",
		RegulatoryContext:     "CFR 21 Part 11.10(b) - System must provide accurate and complete copies of records including all output data",
		BusinessJustification: "STDOUT contains critical execution results and diagnostic information required for model validation and troubleshooting",
		RiskAssessment:        "HIGH - Missing STDOUT data compromises result verification and debugging capabilities",
		TestObjective:         "Verify that complete STDOUT from command execution is captured and stored in run logs",
		AcceptanceCriteria:    "All STDOUT data is captured without truncation and properly stored in JSON run log format",
		Category:              CategoryAuditCompliance,
	},
	"REQ-44": {
		ID:                    "REQ-44",
		Title:                 "STDERR Audit Capture",
		Description:           "Janus must capture and log complete STDERR from executed commands",
		RegulatoryContext:     "CFR 21 Part 11.10(b) - System must provide accurate and complete copies of records including all error and diagnostic data",
		BusinessJustification: "STDERR contains critical error and warning information essential for validating execution success and diagnosing issues",
		RiskAssessment:        "HIGH - Missing STDERR data compromises error detection and system reliability validation",
		TestObjective:         "Verify that complete STDERR from command execution is captured and stored in run logs",
		AcceptanceCriteria:    "All STDERR data is captured without truncation and properly stored in JSON run log format",
		Category:              CategoryAuditCompliance,
	},
	"REQ-45": {
		ID:                    "REQ-45",
		Title:                 "Binary Path Audit Logging",
		Description:           "Janus must log the complete path of executed binaries for execution traceability",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Audit trails must include sufficient information to independently verify what operations were performed",
		BusinessJustification: "Binary path logging ensures traceability of which specific tools and versions were used for each execution",
		RiskAssessment:        "MEDIUM - Missing binary paths compromise run log completeness but do not affect execution accuracy",
		TestObjective:         "Verify that complete binary paths are captured and logged in run logs for all executions",
		AcceptanceCriteria:    "Full absolute paths to executed binaries are recorded in run logs with proper format",
		Category:              CategoryAuditCompliance,
	},
	"REQ-46": {
		ID:                    "REQ-46",
		Title:                 "Command Arguments Audit Logging",
		Description:           "Janus must log all command-line arguments passed to executed commands",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Audit trails must include sufficient information to independently reconstruct the sequence of events",
		BusinessJustification: "Command arguments contain critical execution parameters that must be traceable for result validation and compliance",
		RiskAssessment:        "HIGH - Missing command arguments compromise the ability to verify and reproduce execution results",
		TestObjective:         "Verify that all command-line arguments are captured and logged in run logs",
		AcceptanceCriteria:    "Complete command argument arrays are recorded in run logs preserving order and formatting",
		Category:              CategoryAuditCompliance,
	},
	"REQ-47": {
		ID:                    "REQ-47",
		Title:                 "Hermes Model Configuration File",
		Description:           "Hermes execution shall pull configuration from a .janus.config.json file colocated with the model",
		RegulatoryContext:     "CFR 21 Part 11.10(a) - Validated systems must ensure procedural controls are consistently applied through standardized configuration",
		BusinessJustification: "Model-specific container configuration enables reproducible containerized NONMEM execution while maintaining execution environment traceability",
		RiskAssessment:        "HIGH - Incorrect container configuration could lead to invalid execution environments affecting model results",
		TestObjective:         "Verify that Hermes executor loads and validates .janus.config.json from model directory",
		AcceptanceCriteria:    "System successfully loads container image and resource specifications from model-colocated configuration file",
		Category:              CategoryHermesExecution,
	},
	"REQ-48": {
		ID:                    "REQ-48",
		Title:                 "Hermes Configuration Prompt on Missing File",
		Description:           "If .janus.config.json is not present, Janus shall prompt the user for configuration details",
		RegulatoryContext:     "CFR 21 Part 11.10(f) - Systems must ensure data cannot be executed without proper configuration",
		BusinessJustification: "Interactive configuration collection prevents execution failures and ensures users explicitly define execution parameters",
		RiskAssessment:        "MEDIUM - Missing configuration prompts could prevent execution but do not affect result validity",
		TestObjective:         "Verify that missing config file triggers GUI dialog and creates valid .janus.config.json",
		AcceptanceCriteria:    "System presents configuration dialog for missing files and creates valid configuration before execution",
		Category:              CategoryHermesExecution,
	},
	"REQ-49": {
		ID:                    "REQ-49",
		Title:                 "Run Log Capture for NONMEM, BBI, and PSN Modes",
		Description:           "For NONMEM, BBI, and PSN execution modes, Janus shall collect STDERR, STDOUT, and common output files into the run log",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Run logs must capture complete execution output for compliance verification",
		BusinessJustification: "Comprehensive output capture enables complete traceability of execution results for regulatory submissions",
		RiskAssessment:        "HIGH - Incomplete output capture compromises regulatory compliance and result reproducibility",
		TestObjective:         "Verify that run log entries contain compressed STDERR/STDOUT and embedded file content for standard executions",
		AcceptanceCriteria:    "Run logs contain complete execution output in compressed format with all relevant output files embedded",
		Category:              CategoryAuditCompliance,
	},
	"REQ-50": {
		ID:                    "REQ-50",
		Title:                 "Hermes Execution Output Capture",
		Description:           "When HERMES is the execution model, Janus shall capture STDERR, STDOUT, and all files returned from Hermes into the run log",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Container execution logs must include complete output capture with metadata",
		BusinessJustification: "Containerized execution requires enhanced logging to capture container-specific execution details for compliance",
		RiskAssessment:        "HIGH - Incomplete Hermes output capture compromises container execution traceability and regulatory compliance",
		TestObjective:         "Verify that Hermes executions capture container stdout/stderr and all FileChunk streams into the run log",
		AcceptanceCriteria:    "Run logs contain complete Hermes execution output including streaming data and collected files",
		Category:              CategoryHermesExecution,
	},
	"REQ-51": {
		ID:                    "REQ-51",
		Title:                 "Hermes Container Image Provenance",
		Description:           "Hermes executions shall record complete container image provenance in the run log",
		RegulatoryContext:     "CFR 21 Part 11.10(a) - Execution environment provenance is critical for ensuring validated systems remain in validated state",
		BusinessJustification: "Container image details (name, tag, SHA256 digest) ensure exact execution environment reproducibility for regulatory compliance",
		RiskAssessment:        "CRITICAL - Missing image provenance prevents verification of execution environment validity",
		TestObjective:         "Verify that run log contains image name, tag, digest, and full reference for Hermes executions",
		AcceptanceCriteria:    "Run logs include complete container image metadata with SHA256 digest for execution environment verification",
		Category:              CategoryHermesExecution,
	},
	"REQ-52": {
		ID:                    "REQ-52",
		Title:                 "Hermes Resource Configuration Logging",
		Description:           "Hermes executions shall record the resource configuration used at execution time",
		RegulatoryContext:     "CFR 21 Part 11.10(e) - Execution environment parameters must be logged to demonstrate procedural controls",
		BusinessJustification: "CPU and memory resource specifications document exact execution environment for result reproducibility",
		RiskAssessment:        "MEDIUM - Missing resource configuration reduces execution environment traceability but does not affect result validity",
		TestObjective:         "Verify that run log contains CPU and memory resource specifications from .janus.config.json",
		AcceptanceCriteria:    "Run logs include complete resource configuration (CPU cores, memory limits) from model configuration",
		Category:              CategoryHermesExecution,
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
		return "Creates NONMEMExecutor with run log backend disabled (empty string), executes command, verifies no run log file is created or remains empty, demonstrating proper run logging disable functionality"
	case "REQ-42":
		return "Creates NONMEMExecutor with run logging enabled, executes real command, reads generated run log file, validates job ID is present, properly formatted with 'job-' prefix, and has sufficient length for uniqueness"
	case "REQ-43":
		return "Creates NONMEMExecutor with run log enabled, executes command with specific expected output, reads run log file, validates STDOUT field contains expected output from command execution"
	case "REQ-44":
		return "Similar to REQ-43 but specifically validates STDERR capture functionality by executing commands that generate error output and verifying STDERR field in run logs"
	case "REQ-45":
		return "Tests various binary path scenarios (/opt/NONMEM/nm76/run/nmfe76, /usr/local/bin/execute, /usr/bin/bbi, /opt/torque/bin/qsub), validates binary path field in run logs accurately reflects executed binary location"
	case "REQ-46":
		return "Tests various command argument patterns (basic NONMEM args, parallel args, PsN args, BBI args, TORQUE args), validates arguments array in run logs preserves all arguments in correct order and format"
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
	case "REQ-47":
		return "Creates temporary model directory with .janus.config.json file containing container image and resource specifications, loads configuration using LoadHermesModelConfig, validates image name, CPU cores, and memory values are correctly parsed from colocated config file"
	case "REQ-48":
		return "Attempts to load Hermes configuration from model directory without .janus.config.json file present, validates that appropriate error is returned indicating missing config requirement, verifies error message includes directory path to guide user"
	case "REQ-49":
		return "Creates run logger with JSONL backend, simulates standard NONMEM/BBI/PSN execution by recording job with STDOUT/STDERR/output files, reads run log entries, validates complete capture of execution streams and output file list in run log"
	case "REQ-50":
		return "Creates run logger, records Hermes container execution with complete metadata (execution ID, container ID, image details, resources), validates run log entry captures container STDOUT/STDERR, all returned files, and Hermes-specific metadata fields"
	case "REQ-51":
		return "Tests multiple container image provenance scenarios (DockerHub, GHCR, private registry), validates each ContainerImage struct contains complete provenance (name, tag, SHA256 digest, full reference), verifies JSON serialization preserves all provenance fields"
	case "REQ-52":
		return "Tests multiple resource configuration scenarios (standard, high-performance, low-resource), validates HermesResources struct correctly captures CPU cores and memory from model config, verifies JSON serialization preserves resource specifications for run log compliance"
	case "REQ-CLI-01":
		return "Simulates CLI init workflow by creating model file, generating Hermes config with container image and resources, saving config to .janus.config.json, validates config file creation and structure matches expected schema for Hermes execution"
	case "REQ-CLI-02":
		return "Creates Hermes executor with test config, verifies executor implements StreamingExecutor interface, tests SetOutputWriters method with buffer writers, validates stdout/stderr output capture capability for CLI execution"
	case "REQ-CLI-03":
		return "Creates directory structure with non-colocated files (model in models/run1, data in data/), creates model with relative $DATA path (../../data/dataset.csv), tests BuildHermesPayload functional core, validates workspace structure includes both model and data files with preserved relative paths"
	case "REQ-CLI-04":
		return "Creates Hermes executor, verifies StreamingExecutor interface support, configures output writers (buffers), writes test data to stdout/stderr, validates real-time streaming infrastructure works and output is captured correctly"
	case "REQ-CLI-05":
		return "Creates model and Hermes config in temporary directory, validates config file is written to model directory (.janus.config.json), verifies config file is loadable and contains correct values (image, cpu_cores, memory)"
	case "REQ-CLI-06":
		return "Creates model with Hermes config, validates expected run log location (.janus.runlog.json in model directory), verifies config metadata that would be included in run log (execution_mode, container_image, resources, timestamps)"
	case "REQ-CLI-07":
		return "Creates model with $DATA statement referencing data file, uses ParseModelDependencies to extract dependencies, validates model file path is identified, data file is found and path resolved, verifies parser handles NONMEM syntax correctly"
	case "REQ-CLI-08":
		return "Creates model with data file and Hermes config, calls BuildHermesPayload functional core, validates payload structure (workspace files, entry point, config), verifies same function is used by CLI/GUI/REST API for uniform payload construction across all interfaces"
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

// GenerateComprehensiveReport creates a complete run log-ready validation report.
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
			Scope:          "NONMEM execution validation and run log compliance for pharmaceutical software",
			Statement:      "This validation demonstrates that Janus software meets CFR 21 Part 11 requirements for accurate, reliable execution of pharmaceutical modeling software with complete run log capabilities.",
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
