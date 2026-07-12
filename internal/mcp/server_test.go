package mcp

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testToken = "test-token-abc123"

// bearerRoundTripper injects an Authorization header into every request.
type bearerRoundTripper struct {
	token string
	base  http.RoundTripper
}

func (b *bearerRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if b.token != "" {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+b.token)
	}

	return b.base.RoundTrip(r)
}

// startTestServer starts an MCP server on a random loopback port and returns it
// along with its base "http://host:port/mcp" endpoint. It registers cleanup.
func startTestServer(t *testing.T, allowExecute bool) (*Server, string) {
	t.Helper()

	srv, err := NewServer(Config{
		Host:         "127.0.0.1",
		Port:         0,
		AuthToken:    testToken,
		AllowExecute: allowExecute,
		Version:      "test",
	}, &fakeBridge{})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	require.NoError(t, srv.Start(ctx))
	t.Cleanup(func() { _ = srv.Stop() })

	endpoint := "http://" + srv.Addr() + "/mcp"

	return srv, endpoint
}

// connect builds an SDK client session against endpoint using token for auth.
func connect(t *testing.T, endpoint, token string) *mcpsdk.ClientSession {
	t.Helper()

	httpClient := &http.Client{
		Transport: &bearerRoundTripper{token: token, base: http.DefaultTransport},
	}

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "1"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	session, err := client.Connect(ctx, &mcpsdk.StreamableClientTransport{
		Endpoint:   endpoint,
		HTTPClient: httpClient,
	}, nil)
	require.NoError(t, err)

	t.Cleanup(func() { _ = session.Close() })

	return session
}

func toolNames(result *mcpsdk.ListToolsResult) map[string]bool {
	names := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}

	return names
}

func TestServerToolDiscoveryReadOnly(t *testing.T) {
	_, endpoint := startTestServer(t, false)
	session := connect(t, endpoint, testToken)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := toolNames(result)
	for _, want := range []string{"get_run_log_status", "list_runs", "get_run", "list_run_files", "get_run_file"} {
		assert.True(t, names[want], "expected tool %q", want)
	}

	assert.False(t, names["execute_run"], "execute_run must be absent when allow_execute is false")
}

func TestServerToolDiscoveryWithExecute(t *testing.T) {
	_, endpoint := startTestServer(t, true)
	session := connect(t, endpoint, testToken)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	assert.True(t, toolNames(result)["execute_run"], "execute_run must be present when allow_execute is true")
}

func TestServerAuth(t *testing.T) {
	_, endpoint := startTestServer(t, false)

	tests := []struct {
		name       string
		authHeader string
		wantStatus int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"wrong token", "Bearer nope", http.StatusUnauthorized},
		{"malformed header", "Basic " + testToken, http.StatusUnauthorized},
		// A correct token gets past auth; the body is not a valid MCP request, so the
		// MCP handler responds with something other than 401 (typically 400). The point
		// here is only that auth no longer blocks it.
		{"correct token", "Bearer " + testToken, http.StatusBadRequest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(),
				http.MethodPost, endpoint, strings.NewReader("{}"))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			assert.Equal(t, tc.wantStatus, resp.StatusCode)
		})
	}
}

func TestServerStartIdempotentAndStop(t *testing.T) {
	srv, _ := startTestServer(t, false)

	// Second Start is a no-op and must not error or rebind.
	require.NoError(t, srv.Start(context.Background()))
	assert.True(t, srv.Running())

	require.NoError(t, srv.Stop())
	assert.False(t, srv.Running())

	// Stop is idempotent.
	require.NoError(t, srv.Stop())
}

func TestNewServerRequiresToken(t *testing.T) {
	_, err := NewServer(Config{Host: "127.0.0.1", Port: 0}, &fakeBridge{})
	require.Error(t, err)
}

func TestNewServerRequiresBridge(t *testing.T) {
	_, err := NewServer(Config{AuthToken: "x"}, nil)
	require.Error(t, err)
}
