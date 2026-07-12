package summary

import (
	"bufio"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// NONMEM output file parsing implementation

// parseEstimationSummary extracts estimation method and run characteristics from NONMEM output.
func (s *NONMEMSummarizer) parseEstimationSummary(outputFile string, estimation *EstimationSummary) error {
	if !fileExists(outputFile) {
		return SummaryError{Type: "missing_file", Message: "output file not found", File: outputFile}
	}

	file, err := os.Open(outputFile)
	if err != nil {
		return SummaryError{Type: "file_error", Message: err.Error(), File: outputFile}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Regex patterns for parsing
	estimationMethodPattern := regexp.MustCompile(`ESTIMATION METHOD:\s*(.+)`)
	estimationMethodPattern2 := regexp.MustCompile(`(FIRST ORDER CONDITIONAL ESTIMATION WITH INTERACTION|FIRST ORDER CONDITIONAL ESTIMATION|FOCE WITH INTERACTION|FOCE)`)
	subjectsPattern := regexp.MustCompile(`(?:TOTAL NO\. OF INDIVIDUALS|TOT\. NO\. OF INDIVIDUALS):\s*(\d+)`)
	observationsPattern := regexp.MustCompile(`(?:TOTAL NO\. OF OBSERVATIONS|TOT\. NO\. OF OBS RECS):\s*(\d+)`)
	sigDigitsPattern := regexp.MustCompile(`(?:SIGNIFICANT DIGITS IN FINAL RESULTS|NO\. OF SIG\. DIGITS IN FINAL EST\.?):\s*(\d+)`)
	terminationPattern := regexp.MustCompile(`(MINIMIZATION SUCCESSFUL|MINIMIZATION TERMINATED|OPTIMIZATION TERMINATED)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse estimation method
		if match := estimationMethodPattern.FindStringSubmatch(line); match != nil {
			estimation.Method = strings.TrimSpace(match[1])
		} else if match := estimationMethodPattern2.FindStringSubmatch(line); match != nil {
			estimation.Method = strings.TrimSpace(match[1])
		}

		// Parse number of subjects
		if match := subjectsPattern.FindStringSubmatch(line); match != nil {
			if subjects, err := strconv.Atoi(match[1]); err == nil {
				estimation.Subjects = subjects
			}
		}

		// Parse number of observations
		if match := observationsPattern.FindStringSubmatch(line); match != nil {
			if obs, err := strconv.Atoi(match[1]); err == nil {
				estimation.Observations = obs
			}
		}

		// Parse significant digits
		if match := sigDigitsPattern.FindStringSubmatch(line); match != nil {
			if sigDigits, err := strconv.Atoi(match[1]); err == nil {
				estimation.SignificantDigits = sigDigits
			}
		}

		// Parse termination reason
		if match := terminationPattern.FindStringSubmatch(line); match != nil {
			estimation.TerminationReason = match[1]
			estimation.Minimized = strings.Contains(match[1], "SUCCESSFUL")
		}
	}

	return scanner.Err()
}

// parseGoodnessOfFit extracts OFV, AIC, BIC from NONMEM output.
func (s *NONMEMSummarizer) parseGoodnessOfFit(outputFile string, gof *GoodnessOfFitSummary) error {
	if !fileExists(outputFile) {
		return SummaryError{Type: "missing_file", Message: "output file not found", File: outputFile}
	}

	file, err := os.Open(outputFile)
	if err != nil {
		return SummaryError{Type: "file_error", Message: err.Error(), File: outputFile}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Regex patterns for fit statistics
	ofvPattern := regexp.MustCompile(`#OBJV:\*{9,}\s*([-+]?\d*\.?\d+([eE][-+]?\d+)?)`)
	aicPattern := regexp.MustCompile(`AIC\s*=\s*([-+]?\d*\.?\d+([eE][-+]?\d+)?)`)
	bicPattern := regexp.MustCompile(`BIC\s*=\s*([-+]?\d*\.?\d+([eE][-+]?\d+)?)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse objective function value
		if match := ofvPattern.FindStringSubmatch(line); match != nil {
			if ofv, err := strconv.ParseFloat(match[1], 64); err == nil {
				gof.ObjectiveFunctionValue = &ofv
				// Calculate -2 log-likelihood approximation
				logLikelihood := -ofv / 2
				gof.LogLikelihood = &logLikelihood
			}
		}

		// Parse AIC
		if match := aicPattern.FindStringSubmatch(line); match != nil {
			if aic, err := strconv.ParseFloat(match[1], 64); err == nil {
				gof.AIC = &aic
			}
		}

		// Parse BIC
		if match := bicPattern.FindStringSubmatch(line); match != nil {
			if bic, err := strconv.ParseFloat(match[1], 64); err == nil {
				gof.BIC = &bic
			}
		}
	}

	return scanner.Err()
}

// parseDiagnosticSummary extracts convergence and stability indicators.
func (s *NONMEMSummarizer) parseDiagnosticSummary(outputFile string, diagnostics *DiagnosticSummary) error {
	if !fileExists(outputFile) {
		return SummaryError{Type: "missing_file", Message: "output file not found", File: outputFile}
	}

	file, err := os.Open(outputFile)
	if err != nil {
		return SummaryError{Type: "file_error", Message: err.Error(), File: outputFile}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Initialize diagnostics
	diagnostics.HeuristicFlags = make(map[string]bool)

	// Patterns for diagnostic indicators
	conditionNumberPattern := regexp.MustCompile(`CONDITION NUMBER\s*=\s*([-+]?\d*\.?\d+([eE][-+]?\d+)?)`)
	covarianceStepPattern := regexp.MustCompile(`COVARIANCE STEP`)
	covarianceAbortedPattern := regexp.MustCompile(`COVARIANCE STEP ABORTED`)
	warningPattern := regexp.MustCompile(`WARNING`)
	errorPattern := regexp.MustCompile(`ERROR`)
	boundaryPattern := regexp.MustCompile(`PARAMETER NEAR BOUNDARY`)
	hessianResetPattern := regexp.MustCompile(`HESSIAN RESET`)
	eigenvaluePattern := regexp.MustCompile(`EIGENVALUE`)

	var warnings []string
	var errors []string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Parse condition number
		if match := conditionNumberPattern.FindStringSubmatch(line); match != nil {
			if condNum, err := strconv.ParseFloat(match[1], 64); err == nil {
				diagnostics.ConditionNumber = &condNum
				// Flag large condition numbers (>1000 is often problematic)
				diagnostics.LargeConditionNumber = condNum > 1000
				diagnostics.HeuristicFlags["large_condition_number"] = condNum > 1000
			}
		}

		// Check for covariance step indicators
		if covarianceAbortedPattern.MatchString(line) {
			diagnostics.CovarianceStepAborted = true
			diagnostics.CovarianceStepSuccess = false
			diagnostics.HeuristicFlags["covariance_step_aborted"] = true
		} else if covarianceStepPattern.MatchString(line) {
			diagnostics.CovarianceStepSuccess = true
		}

		// Track if we've already added this line to warnings
		lineAdded := false

		// Check for parameter boundary issues
		if boundaryPattern.MatchString(line) {
			diagnostics.ParametersNearBoundary = true
			diagnostics.HeuristicFlags["parameters_near_boundary"] = true
			warnings = append(warnings, line)
			lineAdded = true
		}

		// Check for Hessian reset
		if hessianResetPattern.MatchString(line) {
			diagnostics.HessianReset = true
			diagnostics.HeuristicFlags["hessian_reset"] = true
			if !lineAdded {
				warnings = append(warnings, line)
				lineAdded = true
			}
		}

		// Check for eigenvalue issues
		if eigenvaluePattern.MatchString(line) && (strings.Contains(line, "NEGATIVE") || strings.Contains(line, "PROBLEM")) {
			diagnostics.EigenvalueIssues = true
			diagnostics.HeuristicFlags["eigenvalue_issues"] = true
			// Only add to warnings if it's not an error
			if !lineAdded && !errorPattern.MatchString(line) {
				warnings = append(warnings, line)
				lineAdded = true
			}
		}

		// Collect warnings and errors (avoid duplicates)
		if warningPattern.MatchString(line) && !lineAdded {
			warnings = append(warnings, line)
		}
		if errorPattern.MatchString(line) {
			errors = append(errors, line)
		}
	}

	diagnostics.Warnings = warnings
	diagnostics.Errors = errors

	return scanner.Err()
}

// parseParameterSummary extracts parameter estimates from the .ext file.
func (s *NONMEMSummarizer) parseParameterSummary(extFile string, parameters *ParameterSummary) error {
	if !fileExists(extFile) {
		return SummaryError{Type: "missing_file", Message: "ext file not found", File: extFile}
	}

	file, err := os.Open(extFile)
	if err != nil {
		return SummaryError{Type: "file_error", Message: err.Error(), File: extFile}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var headerLine string
	var finalEstimates []string

	// Read through the file to find the final estimates
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if strings.HasPrefix(line, ";") || line == "" {
			continue
		}

		// Check if this is a header line (contains THETA, OMEGA, SIGMA)
		if strings.Contains(line, "THETA") || strings.Contains(line, "OMEGA") || strings.Contains(line, "SIGMA") {
			headerLine = line

			continue
		}

		// Look for the final estimates line (usually the last non-header line)
		if headerLine != "" && !strings.HasPrefix(line, "TABLE") {
			finalEstimates = strings.Fields(line)
		}
	}

	if headerLine == "" || len(finalEstimates) == 0 {
		return SummaryError{Type: "parse_error", Message: "could not find parameter estimates", File: extFile}
	}

	// Parse parameter names from header
	headers := strings.Fields(headerLine)

	// Skip the first column (usually ITERATION or similar)
	paramHeaders := headers[1:]
	paramValues := finalEstimates[1:]

	if len(paramHeaders) != len(paramValues) {
		return SummaryError{Type: "parse_error", Message: "parameter count mismatch", File: extFile}
	}

	// Categorize and create parameter estimates
	var thetas, omegas, sigmas []ParameterEstimate

	for i, header := range paramHeaders {
		if i >= len(paramValues) {
			break
		}

		// Skip OBJ column and other non-parameter columns
		if header == "OBJ" || strings.HasPrefix(header, "ITERATION") {
			continue
		}

		param := ParameterEstimate{
			Name:  header,
			Fixed: false, // We'll determine this from the value
		}

		// Parse the parameter value
		if value, err := strconv.ParseFloat(paramValues[i], 64); err == nil {
			param.Estimate = &value
		}

		// Categorize parameter by name and only include meaningful parameters
		switch {
		case strings.HasPrefix(header, "THETA"):
			thetas = append(thetas, param)
		case strings.HasPrefix(header, "OMEGA"):
			// Skip OMEGA(2,1) since it's zero (off-diagonal)
			if header != "OMEGA(2,1)" {
				omegas = append(omegas, param)
			}
		case strings.HasPrefix(header, "SIGMA") && param.Estimate != nil && *param.Estimate != 1.0:
			// Only include SIGMA if it's not the fixed value of 1.0
			sigmas = append(sigmas, param)
		}
	}

	// Update parameter summary
	totalMeaningfulParams := len(thetas) + len(omegas) + len(sigmas)
	parameters.TotalParameters = totalMeaningfulParams
	parameters.EstimatedParameters = totalMeaningfulParams // All meaningful params are estimated
	parameters.FixedParameters = 0
	parameters.Thetas = thetas
	parameters.Omegas = omegas
	parameters.Sigmas = sigmas

	return scanner.Err()
}

// calculateRunTime extracts runtime information from the output file.
func (s *NONMEMSummarizer) calculateRunTime(outputFile string) *int64 {
	if !fileExists(outputFile) {
		return nil
	}

	file, err := os.Open(outputFile)
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Look for timing information in NONMEM output
	timePattern := regexp.MustCompile(`TOTAL NO\. OF ELAPSED SECONDS:\s*([-+]?\d*\.?\d+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if match := timePattern.FindStringSubmatch(line); match != nil {
			if seconds, err := strconv.ParseFloat(match[1], 64); err == nil {
				milliseconds := int64(seconds * 1000)

				return &milliseconds
			}
		}
	}

	return nil
}

// collectSourceFiles gathers file metadata for audit trail.
//
//nolint:unparam // error return reserved for future checksum calculation errors
func (s *NONMEMSummarizer) collectSourceFiles(files ModelFiles, computeChecksum bool) ([]SourceFile, error) {
	var sourceFiles []SourceFile

	// Helper function to process a single file
	processFile := func(filePath, fileType string) {
		if filePath == "" || !fileExists(filePath) {
			return
		}

		info, err := os.Stat(filePath)
		if err != nil {
			return
		}

		sourceFile := SourceFile{
			Path:         filePath,
			Type:         fileType,
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
		}

		if computeChecksum {
			// TODO: Implement checksum calculation
			sourceFile.Checksum = "sha256:placeholder"
		}

		sourceFiles = append(sourceFiles, sourceFile)
	}

	// Process all file types
	processFile(files.ControlFile, "control")
	processFile(files.OutputFile, "output")
	processFile(files.ExtFile, "ext")
	processFile(files.CovFile, "cov")
	processFile(files.CorFile, "cor")
	processFile(files.PhiFile, "phi")

	for _, dataFile := range files.DataFiles {
		processFile(dataFile, "data")
	}

	return sourceFiles, nil
}

// findFile looks for a file with any of the given extensions.
func (s *NONMEMSummarizer) findFile(basePath string, extensions []string) string {
	for _, ext := range extensions {
		candidate := basePath + ext
		if fileExists(candidate) {
			return candidate
		}
	}
	// If no files exist, return the first candidate for expected path
	if len(extensions) > 0 {
		return basePath + extensions[0]
	}

	return ""
}

// extractDataFiles parses the control file to find $DATA statements.
func (s *NONMEMSummarizer) extractDataFiles(controlFile string) ([]string, error) {
	if !fileExists(controlFile) {
		return nil, SummaryError{Type: "missing_file", Message: "control file not found", File: controlFile}
	}

	file, err := os.Open(controlFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var dataFiles []string

	// Look for $DATA statements
	dataPattern := regexp.MustCompile(`^\s*\$DATA\s+(.+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if match := dataPattern.FindStringSubmatch(line); match != nil {
			// Extract the data file path (first word after $DATA)
			dataFilePath := strings.Fields(match[1])[0]
			// Remove any quotes
			dataFilePath = strings.Trim(dataFilePath, `"'`)
			dataFiles = append(dataFiles, dataFilePath)
		}
	}

	return dataFiles, scanner.Err()
}

// fileExists is a helper function to check if file exists.
func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}

	return !info.IsDir()
}
