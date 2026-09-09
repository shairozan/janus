package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shairozan/janus/internal/config"
)

// Client represents a SLURM client that can operate in either CLI or REST mode.
type Client interface {
	// SubmitJob submits a job to SLURM and returns the job ID
	SubmitJob(ctx context.Context, jobScript string, options SubmitOptions) (*JobInfo, error)

	// GetJobStatus retrieves the status of a job by ID
	GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error)

	// CancelJob cancels a running job
	CancelJob(ctx context.Context, jobID string) error

	// GetJobOutput retrieves the output of a completed job
	GetJobOutput(ctx context.Context, jobID string) (*JobOutput, error)

	// Close closes the client and cleans up resources
	Close() error
}

// SubmitOptions represents options for job submission.
type SubmitOptions struct {
	JobName     string            `json:"job_name,omitempty"`
	Partition   string            `json:"partition,omitempty"`
	Nodes       int               `json:"nodes,omitempty"`
	CPUs        int               `json:"cpus,omitempty"`
	Memory      string            `json:"memory,omitempty"`
	TimeLimit   string            `json:"time_limit,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	OutputFile  string            `json:"output_file,omitempty"`
	ErrorFile   string            `json:"error_file,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
}

// JobInfo represents information about a submitted job.
type JobInfo struct {
	JobID      string `json:"job_id"`
	JobName    string `json:"job_name,omitempty"`
	State      string `json:"state"`
	SubmitTime string `json:"submit_time,omitempty"`
	StartTime  string `json:"start_time,omitempty"`
	EndTime    string `json:"end_time,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Partition  string `json:"partition,omitempty"`
	WorkingDir string `json:"working_dir,omitempty"`
}

// JobStatus represents the current status of a job.
type JobStatus struct {
	JobID     string `json:"job_id"`
	State     string `json:"state"`
	ExitCode  int    `json:"exit_code,omitempty"`
	StartTime string `json:"start_time,omitempty"`
	EndTime   string `json:"end_time,omitempty"`
	RunTime   string `json:"run_time,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// JobOutput represents the output of a completed job.
type JobOutput struct {
	JobID  string `json:"job_id"`
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// NewClient creates a new SLURM client based on the provided configuration.
func NewClient(cfg config.SLURMConfig) (Client, error) {
	// Default to CLI mode if not specified
	mode := cfg.Mode
	if mode == "" {
		mode = config.SLURMModeCLI
	}

	switch mode {
	case config.SLURMModeREST:
		return NewRESTClient(cfg.REST)
	case config.SLURMModeCLI:
		return NewCLIClient(cfg)
	default:
		return nil, fmt.Errorf("unsupported SLURM mode: %s", mode)
	}
}

// RESTClient implements the Client interface using SLURM REST API via Unix domain socket.
type RESTClient struct {
	config      config.SLURMRESTConfig
	httpClient  *http.Client
	baseURL     string
	lastRequest *RequestDetails // For audit logging
}

// RequestDetails captures REST API request/response for audit logging.
type RequestDetails struct {
	Method       string
	URL          string
	RequestBody  string
	ResponseCode int
	ResponseBody string
}

// NewRESTClient creates a new REST-based SLURM client.
func NewRESTClient(cfg config.SLURMRESTConfig) (*RESTClient, error) {
	if err := config.ValidateSLURMMode(config.SLURMConfig{Mode: config.SLURMModeREST, REST: cfg}); err != nil {
		return nil, fmt.Errorf("invalid SLURM REST configuration: %w", err)
	}

	// Parse timeout
	timeout := 30 * time.Second
	if cfg.Timeout != "" {
		var err error
		timeout, err = time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout format: %w", err)
		}
	}

	// Create HTTP client - handle both Unix domain sockets and HTTP URLs (for testing)
	var transport http.RoundTripper
	var baseURL string

	if strings.HasPrefix(cfg.SocketPath, "http://") || strings.HasPrefix(cfg.SocketPath, "https://") {
		// For testing: Use HTTP URL directly
		transport = http.DefaultTransport
		baseURL = fmt.Sprintf("%s/slurm/%s", cfg.SocketPath, cfg.APIVersion)
	} else {
		// Production: Use Unix domain socket
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", cfg.SocketPath)
			},
		}
		baseURL = fmt.Sprintf("http://unix/slurm/%s", cfg.APIVersion)
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}

	return &RESTClient{
		config:     cfg,
		httpClient: httpClient,
		baseURL:    baseURL,
	}, nil
}

// SubmitJob submits a job via SLURM REST API.
func (c *RESTClient) SubmitJob(ctx context.Context, jobScript string, options SubmitOptions) (*JobInfo, error) {
	// Construct job submission request
	job := map[string]interface{}{
		"name":                      options.JobName,
		"current_working_directory": options.WorkingDir,
		"standard_output":           options.OutputFile,
		"standard_error":            options.ErrorFile,
	}

	// Only add partition if it's not empty
	if options.Partition != "" {
		job["partition"] = options.Partition
	}

	// For SLURM REST API v0.0.36, we can submit either:
	// 1. A "script" field with a bash script (what we had)
	// 2. Set the script directly in the job object
	// The issue is that "script" at top level may not work - let's try both approaches
	jobReq := map[string]interface{}{
		"script": jobScript,
		"job":    job,
	}

	// Add resource requirements if specified
	if job, ok := jobReq["job"].(map[string]interface{}); ok {
		// Set CPU count directly - SLURM REST API v0.0.36 expects integer values
		// Only set if > 0, otherwise let SLURM use cluster defaults
		if options.CPUs > 0 {
			job["cpus_per_task"] = options.CPUs
		}

		// Set node count if specified
		if options.Nodes > 0 {
			job["nodes"] = options.Nodes
		}

		// Set memory per node if specified (convert GB to MB)
		if options.Memory != "" {
			// Parse memory from string format (e.g., "4G" -> 4096 MB)
			memoryMB, err := parseMemoryToMB(options.Memory)
			if err == nil && memoryMB > 0 {
				job["memory_per_node"] = memoryMB
			}
		}

		// Set time limit if specified
		if options.TimeLimit != "" {
			job["time_limit"] = options.TimeLimit
		}
	}

	// Add environment variables - SLURM REST API requires environment field as dictionary
	if job, ok := jobReq["job"].(map[string]interface{}); ok {
		env := make(map[string]string)

		// Start with the user's current environment
		for _, envVar := range os.Environ() {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}

		// Override with any custom environment variables specified
		for k, v := range options.Environment {
			env[k] = v
		}

		job["environment"] = env
	}

	// Make the request
	resp, err := c.makeRequest(ctx, "POST", "/job/submit", jobReq)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode job submission response: %w", err)
	}

	// Extract job ID from response
	jobID, ok := result["job_id"].(string)
	if !ok {
		// Try alternative field names
		if jobIDFloat, ok := result["job_id"].(float64); ok {
			jobID = fmt.Sprintf("%.0f", jobIDFloat)
		} else {
			return nil, fmt.Errorf("job_id not found in response")
		}
	}

	return &JobInfo{
		JobID:      jobID,
		JobName:    options.JobName,
		State:      "PENDING",
		WorkingDir: options.WorkingDir,
	}, nil
}

// GetJobStatus retrieves job status via SLURM REST API.
func (c *RESTClient) GetJobStatus(ctx context.Context, jobID string) (*JobStatus, error) {
	url := fmt.Sprintf("/job/%s", jobID)
	resp, err := c.makeRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get job status: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode job status response: %w", err)
	}

	// Extract job status information
	jobs, ok := result["jobs"].([]interface{})
	if !ok || len(jobs) == 0 {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	job, ok := jobs[0].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid job data format")
	}

	status := &JobStatus{
		JobID: jobID,
	}

	if state, ok := job["job_state"].(string); ok {
		status.State = state
	}
	if exitCode, ok := job["exit_code"].(float64); ok {
		status.ExitCode = int(exitCode)
	}
	if startTime, ok := job["start_time"].(string); ok {
		status.StartTime = startTime
	}
	if endTime, ok := job["end_time"].(string); ok {
		status.EndTime = endTime
	}
	if reason, ok := job["state_reason"].(string); ok {
		status.Reason = reason
	}

	return status, nil
}

// CancelJob cancels a job via SLURM REST API.
func (c *RESTClient) CancelJob(ctx context.Context, jobID string) error {
	url := fmt.Sprintf("/job/%s", jobID)
	req := map[string]interface{}{
		"signal": "TERM",
	}

	resp, err := c.makeRequest(ctx, "DELETE", url, req)
	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to cancel job, status: %d", resp.StatusCode)
	}

	return nil
}

// GetJobOutput retrieves job output via SLURM REST API.
func (c *RESTClient) GetJobOutput(ctx context.Context, jobID string) (*JobOutput, error) {
	// Note: Job output retrieval via REST API might require additional endpoints
	// or file system access depending on the SLURM REST API version
	return nil, fmt.Errorf("job output retrieval not yet implemented for REST API mode")
}

// Close closes the REST client and cleans up resources.
func (c *RESTClient) Close() error {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}

	return nil
}

// makeRequest is a helper method for making HTTP requests to the SLURM REST API.
func (c *RESTClient) makeRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Response, error) {
	url := c.baseURL + endpoint

	var reqBody io.Reader
	var requestBodyStr string
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		requestBodyStr = string(jsonBody)
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication token if configured
	if c.config.AuthToken != "" {
		req.Header.Set("X-SLURM-USER-TOKEN", c.config.AuthToken)
	}

	// Log the REST API request for audit purposes
	if requestBodyStr != "" {
		log.Printf("[SLURM REST API] Request: %s %s\nBody: %s\n", method, url, requestBodyStr)
	} else {
		log.Printf("[SLURM REST API] Request: %s %s\n", method, url)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	// Read response body for logging while preserving it for the caller
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	resp.Body.Close()

	// Log the REST API response for audit purposes
	log.Printf("[SLURM REST API] Response: %d %s\nBody: %s\n", resp.StatusCode, resp.Status, string(responseBody))

	// Store request/response details for audit logging
	c.lastRequest = &RequestDetails{
		Method:       method,
		URL:          url,
		RequestBody:  requestBodyStr,
		ResponseCode: resp.StatusCode,
		ResponseBody: string(responseBody),
	}

	// Create a new reader with the response body for the caller
	resp.Body = io.NopCloser(bytes.NewReader(responseBody))

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(responseBody))
	}

	return resp, nil
}

// GetLastRequestDetails returns the details of the last REST API request for audit logging.
func (c *RESTClient) GetLastRequestDetails() *RequestDetails {
	return c.lastRequest
}

// parseMemoryToMB converts memory string format to megabytes integer.
// Supports formats like "4G", "512M", "1024" (assumes MB if no suffix).
func parseMemoryToMB(memoryStr string) (int, error) {
	memoryStr = strings.TrimSpace(strings.ToUpper(memoryStr))

	if memoryStr == "" {
		return 0, fmt.Errorf("empty memory string")
	}

	// Check for suffix
	if strings.HasSuffix(memoryStr, "G") {
		// Gigabytes to megabytes
		gbStr := strings.TrimSuffix(memoryStr, "G")
		gb, err := strconv.Atoi(gbStr)
		if err != nil {
			return 0, fmt.Errorf("invalid gigabyte value: %s", gbStr)
		}

		return gb * 1024, nil
	}

	if strings.HasSuffix(memoryStr, "M") {
		// Megabytes
		mbStr := strings.TrimSuffix(memoryStr, "M")
		mb, err := strconv.Atoi(mbStr)
		if err != nil {
			return 0, fmt.Errorf("invalid megabyte value: %s", mbStr)
		}

		return mb, nil
	}

	if strings.HasSuffix(memoryStr, "K") {
		// Kilobytes to megabytes
		kbStr := strings.TrimSuffix(memoryStr, "K")
		kb, err := strconv.Atoi(kbStr)
		if err != nil {
			return 0, fmt.Errorf("invalid kilobyte value: %s", kbStr)
		}

		return kb / 1024, nil
	}

	// No suffix - assume megabytes
	mb, err := strconv.Atoi(memoryStr)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", memoryStr)
	}

	return mb, nil
}
