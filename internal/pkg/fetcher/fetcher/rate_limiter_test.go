package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"
)

func setup() {
	userAgentData = append(userAgentData, UserAgentData{UserAgent: "test-agent"})
}

// TestInvalidURL tests handling of invalid URLs.
func TestInvalidURL(t *testing.T) {
	err := waitForPermission(context.Background(), "invalid url")
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

// TestNewDomainAddedToCache tests if a new domain is added to the cache.
func TestNewDomainAddedToCache(t *testing.T) {
	setup()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer testServer.Close()

	// Reset cache and HTTP client
	defer func() {
		robotsCacheMutex.Lock()
		robotsCache = make(map[string]*RobotsData)
		robotsCacheMutex.Unlock()
		httpClient = &http.Client{}
	}()

	testURL := testServer.URL + "/path"
	parsedURL, _ := url.Parse(testURL)
	domain := parsedURL.Hostname()

	err := waitForPermission(context.Background(), testURL)
	if err != nil {
		t.Fatal(err)
	}

	robotsCacheMutex.Lock()
	defer robotsCacheMutex.Unlock()
	if _, exists := robotsCache[domain]; !exists {
		t.Errorf("Domain %s not found in cache", domain)
	}
}

// TestRobotsRefreshAfter24Hours tests robots.txt refresh after 24 hours.
func TestRobotsRefreshAfter24Hours(t *testing.T) {
	setup()
	var requestCount int
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Write([]byte("User-agent: test-agent"))
	}))
	defer testServer.Close()

	httpClient = testServer.Client()
	//getRandomUserAgent = func() string { return "test-agent" }
	defer func() {
		robotsCacheMutex.Lock()
		robotsCache = make(map[string]*RobotsData)
		robotsCacheMutex.Unlock()
	}()

	testURL := testServer.URL + "/path"
	waitForPermission(context.Background(), testURL) // Initial fetch

	// Set robotsFetched to 25 hours ago
	parsedURL, _ := url.Parse(testURL)
	domain := parsedURL.Hostname()
	robotsCacheMutex.Lock()
	rData := robotsCache[domain]
	robotsCacheMutex.Unlock()

	rData.mutex.Lock()
	rData.robotsFetched = time.Now().Add(-25 * time.Hour)
	rData.mutex.Unlock()

	waitForPermission(context.Background(), testURL)
	if requestCount != 2 {
		t.Errorf("Expected 2 robots.txt fetches, got %d", requestCount)
	}
}

// TestRobotsFetchFailureAllowsAccess tests failed robots.txt fetch allows access.
func TestRobotsFetchFailureAllowsAccess(t *testing.T) {

	setup()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer testServer.Close()

	httpClient = testServer.Client()
	defer func() {
		robotsCacheMutex.Lock()
		robotsCache = make(map[string]*RobotsData)
		robotsCacheMutex.Unlock()
	}()

	err := waitForPermission(context.Background(), testServer.URL+"/anypath")
	if err != nil {
		t.Errorf("Expected access to be allowed, got %v", err)
	}
}

// TestConcurrentAccess tests concurrent access to the same domain.
func TestConcurrentAccess(t *testing.T) {

	setup()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *"))
	}))
	defer testServer.Close()

	httpClient = testServer.Client()
	defer func() {
		robotsCacheMutex.Lock()
		robotsCache = make(map[string]*RobotsData)
		robotsCacheMutex.Unlock()
	}()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			waitForPermission(context.Background(), testServer.URL+"/path")
		}()
	}
	wg.Wait()
}
