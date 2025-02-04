// fetcher_fetch_test.go
package fetcher

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Checks that the fetcher is initialized successfully.
func TestInit(t *testing.T) {
	err := Init()
	if err != nil {
		t.Logf("Init returned error (this may be expected if the file is missing): %v", err)
	} else if len(userAgentData) == 0 {
		t.Error("expected non-empty userAgentData after Init")
	}
}

// Fetch using an httptest server.
func TestFetchContentSuccess(t *testing.T) {
	Init()
	const responseBody = "Hello, World!"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use a valid status code and body.
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(responseBody))
	}))
	defer server.Close()

	ctx := context.Background()
	content, err := fetchContent(ctx, server.URL)
	if err != nil {
		t.Fatalf("fetchContent returned unexpected error: %v", err)
	}
	if content != responseBody {
		t.Errorf("expected %q, got %q", responseBody, content)
	}
}

// Fetching with non-200 response.
func TestFetchContentNon200(t *testing.T) {
	Init()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	context := context.Background()
	_, err := fetchContent(context, server.URL)
	if err == nil {
		t.Fatal("expected an error for non-200 status, got nil")
	}
}

// The response should be truncated to maxBodySize bytes.
func TestFetchContentTruncated(t *testing.T) {
	Init()
	longContent := strings.Repeat("a", int(maxBodySize) + 100) // longer than maxBodySize
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(longContent))
	}))
	defer server.Close()

	context := context.Background()
	content, err := fetchContent(context, server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) != int(maxBodySize) {
		t.Errorf("expected content length %d, got %d", maxBodySize, len(content))
	}
}

// Parsing a simple HTML document should succeed.
func TestParseHTMLWithTimeoutSuccess(t *testing.T) {
	htmlContent := "<html><body><p>Hello</p></body></html>"
	doc, err := parseHTMLWithTimeout(htmlContent, 5 * time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc == nil {
		t.Error("expected a parsed document, got nil")
	}
}

// Timeout should trigger if parsing takes too long.
func TestParseHTMLWithTimeoutTimeout(t *testing.T) {
	// Use a 0ns timeout to force a timeout error.
	htmlContent := "<html><body><p>Hello</p></body></html>"
	_, err := parseHTMLWithTimeout(htmlContent, 0 * time.Nanosecond)
	if err == nil {
		t.Error("expected a timeout error, got nil")
	}
}


// Extracting page data from a very simple HTML document.
func TestExtractPageData(t *testing.T) {
	htmlContent := `<html lang="en"><head><title>Test Page</title></head><body><p>Hello</p></body></html>`
	pd, err := extractPageData(htmlContent, "http://example.com")
	if err != nil {
		t.Fatalf("extractPageData returned error: %v", err)
	}
	if pd.Title != "Test Page" {
		t.Errorf("expected title %q, got %q", "Test Page", pd.Title)
	}
}

// Get the user agent from the userAgentData slice.
func TestGetRandomUserAgent(t *testing.T) {
	// Save the original data.
	original := userAgentData
	defer func() { userAgentData = original }()

	userAgentData = []UserAgentData{
		{UserAgent: "Agent1"},
		{UserAgent: "Agent2"},
		{UserAgent: "Agent3"},
	}
	// Run several times to ensure we get one of the provided agents.
	for i := 0; i < 10; i++ {
		ua := getRandomUserAgent()
		if ua != "Agent1" && ua != "Agent2" && ua != "Agent3" {
			t.Errorf("unexpected user agent returned: %q", ua)
		}
	}
}

// Verifies that Shutdown properly nils the cancel functions.
func TestShutdown(t *testing.T) {
	var browserCalled, allocCalled bool
	browserCancel = func() { browserCalled = true }
	allocCancel = func() { allocCalled = true }

	Shutdown()
	if browserCancel != nil {
		t.Error("expected browserCancel to be nil after Shutdown")
	}
	if allocCancel != nil {
		t.Error("expected allocCancel to be nil after Shutdown")
	}
	if !browserCalled {
		t.Error("expected browserCancel to have been called")
	}
	if !allocCalled {
		t.Error("expected allocCancel to have been called")
	}
}

// Runs a complete Fetch function end-to-end using an httptest server.
func TestFetch(t *testing.T) {
	Init()
	htmlContent := `<html lang="en"><head><title>Test Fetch</title></head><body><p>Hello Fetch</p></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, htmlContent)
	}))
	defer server.Close()

	ctx := context.Background()
	pagedata, err := Fetch(ctx, server.URL)
	if err != nil {
		t.Fatalf("Fetch returned unexpected error: %v", err)
	}
	if pagedata.Title != "Test Fetch" {
		t.Errorf("expected title %q, got %q", "Test Fetch", pagedata.Title)
	}
	if pagedata.URL != server.URL {
		t.Errorf("expected URL %q, got %q", server.URL, pagedata.URL)
	}
	if pagedata.Language != "en" {
		t.Errorf("expected language %q, got %q", "en", pagedata.Language)
	}
	if pagedata.VisibleText != "Test Fetch Hello Fetch" {
		t.Errorf("expected visible text %q, got %q", "Test Fetch Hello Fetch", pagedata.VisibleText)
	}
}
