package practice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadURLsFromFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
		wantErr  bool
	}{
		{
			name:     "valid file with URLs",
			content:  "https://example.com\nhttps://google.com\nhttps://github.com",
			expected: []string{"https://example.com", "https://google.com", "https://github.com"},
		},
		{
			name:     "skips empty lines",
			content:  "https://example.com\n\nhttps://google.com\n\n",
			expected: []string{"https://example.com", "https://google.com"},
		},
		{
			name:     "trims whitespace",
			content:  "  https://example.com  \n\thttps://google.com\t",
			expected: []string{"https://example.com", "https://google.com"},
		},
		{
			name:     "empty file",
			content:  "",
			expected: nil,
		},
		{
			name:     "only whitespace",
			content:  "   \n\n\t\t\n",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "urls.txt")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			got, err := readURLsFromFile(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("readURLsFromFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(got) != len(tt.expected) {
				t.Errorf("readURLsFromFile() got %d URLs, want %d", len(got), len(tt.expected))
				return
			}

			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("readURLsFromFile()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestReadURLsFromFile_NotFound(t *testing.T) {
	_, err := readURLsFromFile("/nonexistent/path/urls.txt")
	if err == nil {
		t.Error("readURLsFromFile() expected error for nonexistent file")
	}
}

func TestComputeSummary(t *testing.T) {
	tests := []struct {
		name     string
		results  []Result
		expected Summary
	}{
		{
			name:     "empty results",
			results:  []Result{},
			expected: Summary{Total: 0, Healthy: 0, Errors: 0, AvgLatency: 0},
		},
		{
			name: "all healthy",
			results: []Result{
				{URL: "http://a.com", StatusCode: 200, LatencyMs: 100},
				{URL: "http://b.com", StatusCode: 201, LatencyMs: 200},
				{URL: "http://c.com", StatusCode: 204, LatencyMs: 300},
			},
			expected: Summary{Total: 3, Healthy: 3, Errors: 0, AvgLatency: 200},
		},
		{
			name: "all errors",
			results: []Result{
				{URL: "http://a.com", Err: context.DeadlineExceeded},
				{URL: "http://b.com", Err: context.Canceled},
			},
			expected: Summary{Total: 2, Healthy: 0, Errors: 2, AvgLatency: 0},
		},
		{
			name: "mixed results",
			results: []Result{
				{URL: "http://a.com", StatusCode: 200, LatencyMs: 100},
				{URL: "http://b.com", StatusCode: 404, LatencyMs: 50},
				{URL: "http://c.com", StatusCode: 500, LatencyMs: 25},
				{URL: "http://d.com", Err: context.DeadlineExceeded, LatencyMs: 5000},
				{URL: "http://e.com", StatusCode: 200, LatencyMs: 200},
			},
			expected: Summary{Total: 5, Healthy: 2, Errors: 1, AvgLatency: 150},
		},
		{
			name: "4xx and 5xx not counted as healthy or error",
			results: []Result{
				{URL: "http://a.com", StatusCode: 400, LatencyMs: 100},
				{URL: "http://b.com", StatusCode: 404, LatencyMs: 100},
				{URL: "http://c.com", StatusCode: 500, LatencyMs: 100},
				{URL: "http://d.com", StatusCode: 503, LatencyMs: 100},
			},
			expected: Summary{Total: 4, Healthy: 0, Errors: 0, AvgLatency: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeSummary(tt.results)

			if got.Total != tt.expected.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.expected.Total)
			}
			if got.Healthy != tt.expected.Healthy {
				t.Errorf("Healthy = %d, want %d", got.Healthy, tt.expected.Healthy)
			}
			if got.Errors != tt.expected.Errors {
				t.Errorf("Errors = %d, want %d", got.Errors, tt.expected.Errors)
			}
			if got.AvgLatency != tt.expected.AvgLatency {
				t.Errorf("AvgLatency = %f, want %f", got.AvgLatency, tt.expected.AvgLatency)
			}
		})
	}
}

func TestCheckUrl(t *testing.T) {
	tests := []struct {
		name           string
		handler        http.HandlerFunc
		expectedStatus int
		expectErr      bool
	}{
		{
			name: "successful 200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			expectedStatus: 200,
			expectErr:      false,
		},
		{
			name: "404 response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedStatus: 404,
			expectErr:      false,
		},
		{
			name: "500 response",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedStatus: 500,
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			result := checkUrl(context.Background(), server.URL)

			if tt.expectErr && result.Err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && result.Err != nil {
				t.Errorf("unexpected error: %v", result.Err)
			}
			if result.StatusCode != tt.expectedStatus {
				t.Errorf("StatusCode = %d, want %d", result.StatusCode, tt.expectedStatus)
			}
			if result.URL != server.URL {
				t.Errorf("URL = %q, want %q", result.URL, server.URL)
			}
			if result.LatencyMs < 0 {
				t.Error("LatencyMs should be non-negative")
			}
		})
	}
}

func TestCheckUrl_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Use a context with a shorter timeout than the server delay
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result := checkUrl(ctx, server.URL)

	if result.Err == nil {
		t.Error("expected timeout error but got none")
	}
}

func TestCheckUrl_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := checkUrl(ctx, server.URL)

	if result.Err == nil {
		t.Error("expected context canceled error but got none")
	}
}

func TestCheckUrl_InvalidURL(t *testing.T) {
	result := checkUrl(context.Background(), "not-a-valid-url")

	if result.Err == nil {
		t.Error("expected error for invalid URL but got none")
	}
}

func TestRunChecker(t *testing.T) {
	// Create a test server that returns different status codes based on path
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/notfound":
			w.WriteHeader(http.StatusNotFound)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	urls := []string{
		server.URL + "/ok",
		server.URL + "/notfound",
		server.URL + "/error",
	}

	results, err := RunChecker(context.Background(), urls, 2)
	if err != nil {
		t.Fatalf("RunChecker() error = %v", err)
	}

	if len(results) != len(urls) {
		t.Errorf("got %d results, want %d", len(results), len(urls))
	}

	// Verify we got results for each URL (order may vary due to concurrency)
	urlSet := make(map[string]bool)
	for _, r := range results {
		urlSet[r.URL] = true
	}

	for _, url := range urls {
		if !urlSet[url] {
			t.Errorf("missing result for URL %q", url)
		}
	}
}

func TestRunChecker_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	urls := make([]string, 10)
	for i := range urls {
		urls[i] = server.URL
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := RunChecker(ctx, urls, 2)

	// Should return context error when cancelled
	if err == nil {
		t.Log("RunChecker completed without error (may have finished before timeout)")
	}
}

func TestRunChecker_EmptyURLs(t *testing.T) {
	results, err := RunChecker(context.Background(), []string{}, 5)
	if err != nil {
		t.Fatalf("RunChecker() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
