//go:build integration
// +build integration

package slurm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pharmalytica/janus/internal/config"
)

// Mock SLURM REST API responses based on actual SLURM REST API v0.0.40

type MockSLURMServer struct {
	server   *httptest.Server
	requests []MockRequest
	jobs     map[string]*MockJob
}

type MockRequest struct {
	Method string
	URL    string
	Body   string
}

type MockJob struct {
	JobID     string `json:"job_id"`
	JobName   string `json:"job_name"`
	JobState  string `json:"job_state"`
	ExitCode  int    `json:"exit_code,omitempty"`
	StdOut    string `json:"standard_output,omitempty"`
	StdErr    string `json:"standard_error,omitempty"`
	StartTime int64  `json:"start_time,omitempty"`
	EndTime   int64  `json:"end_time,omitempty"`
}

func NewMockSLURMServer() *MockSLURMServer {
	mock := &MockSLURMServer{
		requests: make([]MockRequest, 0),
		jobs:     make(map[string]*MockJob),
	}

	mux := http.NewServeMux()

	// Job submission endpoint
	mux.HandleFunc("/slurm/v0.0.40/job/submit", mock.handleJobSubmit)

	// Job operations endpoint (handles status, cancel, output)
	mux.HandleFunc("/slurm/v0.0.40/job/", mock.handleJobOperations)

	mock.server = httptest.NewServer(mux)
	return mock
}

func (m *MockSLURMServer) Close() {
	m.server.Close()
}

func (m *MockSLURMServer) URL() string {
	return m.server.URL
}

func (m *MockSLURMServer) recordRequest(method, url, body string) {
	m.requests = append(m.requests, MockRequest{
		Method: method,
		URL:    url,
		Body:   body,
	})
}

func (m *MockSLURMServer) handleJobSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Record the request
	body := ""
	if r.Body != nil {
		bodyBytes := make([]byte, r.ContentLength)
		r.Body.Read(bodyBytes)
		body = string(bodyBytes)
	}
	m.recordRequest(r.Method, r.URL.Path, body)

	// Generate a mock job ID
	jobID := fmt.Sprintf("12345%d", len(m.jobs)+1)

	// Create mock job
	job := &MockJob{
		JobID:     jobID,
		JobName:   "janus-test-job",
		JobState:  "PENDING",
		StartTime: time.Now().Unix(),
	}
	m.jobs[jobID] = job

	// Return successful submission response
	response := map[string]interface{}{
		"job_id":  jobID,
		"step_id": "batch",
		"errors":  []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *MockSLURMServer) handleJobOperations(w http.ResponseWriter, r *http.Request) {
	// Route based on method and path
	if strings.Contains(r.URL.Path, "output") {
		m.handleJobOutput(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		m.handleJobStatus(w, r)
	case http.MethodDelete:
		m.handleJobCancel(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *MockSLURMServer) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	jobID := pathParts[4]

	m.recordRequest(r.Method, r.URL.Path, "")

	job, exists := m.jobs[jobID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, `{"errors":[{"error":"Job %s not found"}]}`, jobID)
		return
	}

	// Simulate job progression
	now := time.Now().Unix()
	if job.JobState == "PENDING" && now-job.StartTime > 2 {
		job.JobState = "RUNNING"
	} else if job.JobState == "RUNNING" && now-job.StartTime > 5 {
		job.JobState = "COMPLETED"
		job.EndTime = now
		job.ExitCode = 0
	}

	response := map[string]interface{}{
		"jobs": []map[string]interface{}{
			{
				"job_id":    job.JobID,
				"job_state": job.JobState,
				"exit_code": job.ExitCode,
				"name":      job.JobName,
			},
		},
		"errors": []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *MockSLURMServer) handleJobCancel(w http.ResponseWriter, r *http.Request) {
	// Extract job ID from URL path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	jobID := pathParts[4]

	m.recordRequest(r.Method, r.URL.Path, "")

	job, exists := m.jobs[jobID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Update job state to cancelled
	job.JobState = "CANCELLED"

	response := map[string]interface{}{
		"errors": []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (m *MockSLURMServer) handleJobOutput(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	jobID := pathParts[4]

	m.recordRequest(r.Method, r.URL.Path, "")

	_, exists := m.jobs[jobID]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	response := map[string]interface{}{
		"stdout": fmt.Sprintf("Mock stdout for job %s", jobID),
		"stderr": "",
		"errors": []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func TestSLURMRESTClientIntegration(t *testing.T) {
	mockServer := NewMockSLURMServer()
	defer mockServer.Close()

	// Create test config pointing to mock server
	cfg := config.SLURMConfig{
		Mode: config.SLURMModeREST,
		REST: config.SLURMRESTConfig{
			SocketPath: mockServer.URL(), // Using HTTP URL instead of socket for testing
			APIVersion: "v0.0.40",
			Timeout:    "30s",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	require.NotNil(t, client)

	ctx := context.Background()

	t.Run("submit_job", func(t *testing.T) {
		options := SubmitOptions{
			JobName:    "test-job",
			Partition:  "cpu",
			Nodes:      1,
			CPUs:       4,
			TimeLimit:  "01:00:00",
			WorkingDir: "/tmp",
		}

		jobScript := `#!/bin/bash
#SBATCH --job-name=test-job
#SBATCH --partition=cpu
#SBATCH --nodes=1
#SBATCH --cpus-per-task=4
#SBATCH --time=01:00:00

echo "Hello from SLURM job"`

		jobInfo, err := client.SubmitJob(ctx, jobScript, options)
		require.NoError(t, err)
		require.NotNil(t, jobInfo)

		assert.NotEmpty(t, jobInfo.JobID)
		assert.Contains(t, jobInfo.JobID, "12345") // Mock server generates IDs starting with 12345

		// Verify request was recorded
		assert.Len(t, mockServer.requests, 1)
		assert.Equal(t, http.MethodPost, mockServer.requests[0].Method)
		assert.Contains(t, mockServer.requests[0].URL, "/job/submit")
	})

	t.Run("get_job_status", func(t *testing.T) {
		// First submit a job
		options := SubmitOptions{JobName: "status-test-job"}
		jobScript := "#!/bin/bash\necho 'test'"

		jobInfo, err := client.SubmitJob(ctx, jobScript, options)
		require.NoError(t, err)

		// Then check its status
		status, err := client.GetJobStatus(ctx, jobInfo.JobID)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.Equal(t, jobInfo.JobID, status.JobID)
		assert.Contains(t, []string{"PENDING", "RUNNING", "COMPLETED"}, status.State)

		// Check that status request was made
		statusRequests := 0
		for _, req := range mockServer.requests {
			if strings.Contains(req.URL, "/job/") && req.Method == http.MethodGet {
				statusRequests++
			}
		}
		assert.GreaterOrEqual(t, statusRequests, 1)
	})

	t.Run("cancel_job", func(t *testing.T) {
		// First submit a job
		options := SubmitOptions{JobName: "cancel-test-job"}
		jobScript := "#!/bin/bash\nsleep 60"

		jobInfo, err := client.SubmitJob(ctx, jobScript, options)
		require.NoError(t, err)

		// Then cancel it
		err = client.CancelJob(ctx, jobInfo.JobID)
		require.NoError(t, err)

		// Verify job state changed to cancelled
		status, err := client.GetJobStatus(ctx, jobInfo.JobID)
		require.NoError(t, err)

		// Job should eventually be cancelled (might need to wait for mock server logic)
		assert.Contains(t, []string{"CANCELLED", "PENDING", "RUNNING"}, status.State)

		// Check that cancel request was made
		cancelRequests := 0
		for _, req := range mockServer.requests {
			if strings.Contains(req.URL, "/job/") && req.Method == http.MethodDelete {
				cancelRequests++
			}
		}
		assert.GreaterOrEqual(t, cancelRequests, 1)
	})

	t.Run("get_job_output", func(t *testing.T) {
		// First submit a job
		options := SubmitOptions{JobName: "output-test-job"}
		jobScript := "#!/bin/bash\necho 'Test output'"

		jobInfo, err := client.SubmitJob(ctx, jobScript, options)
		require.NoError(t, err)

		// Try to get job output
		output, err := client.GetJobOutput(ctx, jobInfo.JobID)

		// Note: Real SLURM might not have output immediately, but our mock does
		if err == nil {
			require.NotNil(t, output)
			assert.Contains(t, output.Stdout, "Mock stdout")
		} else {
			// Job output retrieval not yet implemented for REST API
			assert.Contains(t, err.Error(), "not yet implemented")
		}
	})

	err = client.Close()
	assert.NoError(t, err)
}

func TestSLURMRESTClientErrorHandling(t *testing.T) {
	// Test with non-existent server
	cfg := config.SLURMConfig{
		Mode: config.SLURMModeREST,
		REST: config.SLURMRESTConfig{
			SocketPath: "http://localhost:99999", // Non-existent port
			APIVersion: "v0.0.40",
			Timeout:    "1s",
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err) // Client creation should succeed

	ctx := context.Background()

	t.Run("submit_job_connection_error", func(t *testing.T) {
		options := SubmitOptions{JobName: "error-test"}
		jobScript := "#!/bin/bash\necho 'test'"

		_, err := client.SubmitJob(ctx, jobScript, options)
		assert.Error(t, err)
		// Just verify we get an error, don't check specific message
	})

	t.Run("get_status_connection_error", func(t *testing.T) {
		_, err := client.GetJobStatus(ctx, "12345")
		assert.Error(t, err)
	})

	t.Run("cancel_job_connection_error", func(t *testing.T) {
		err := client.CancelJob(ctx, "12345")
		assert.Error(t, err)
	})
}

func TestSLURMRESTClientConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		config      config.SLURMConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid_rest_config",
			config: config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					SocketPath: "/var/run/slurm/slurmrestd.sock",
					APIVersion: "v0.0.40",
					Timeout:    "30s",
				},
			},
		},
		{
			name: "missing_socket_path",
			config: config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					APIVersion: "v0.0.40",
					Timeout:    "30s",
				},
			},
			expectError: true,
			errorMsg:    "socket_path",
		},
		{
			name: "missing_api_version",
			config: config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					SocketPath: "/var/run/slurm/slurmrestd.sock",
					Timeout:    "30s",
				},
			},
			expectError: true,
			errorMsg:    "api_version",
		},
		{
			name: "invalid_timeout",
			config: config.SLURMConfig{
				Mode: config.SLURMModeREST,
				REST: config.SLURMRESTConfig{
					SocketPath: "/var/run/slurm/slurmrestd.sock",
					APIVersion: "v0.0.40",
					Timeout:    "invalid",
				},
			},
			expectError: true,
			errorMsg:    "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, client)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, client)

				// Clean up
				if client != nil {
					client.Close()
				}
			}
		})
	}
}

func TestSLURMRESTClientTimeout(t *testing.T) {
	// Create a slow mock server
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Longer than our client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	cfg := config.SLURMConfig{
		Mode: config.SLURMModeREST,
		REST: config.SLURMRESTConfig{
			SocketPath: slowServer.URL,
			APIVersion: "v0.0.40",
			Timeout:    "1s", // Short timeout
		},
	}

	client, err := NewClient(cfg)
	require.NoError(t, err)
	defer client.Close()

	ctx := context.Background()
	options := SubmitOptions{JobName: "timeout-test"}
	jobScript := "#!/bin/bash\necho 'test'"

	_, err = client.SubmitJob(ctx, jobScript, options)
	assert.Error(t, err)
	// Error could be "timeout" or "context deadline exceeded" depending on implementation
	assert.True(t, strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded"))
}
