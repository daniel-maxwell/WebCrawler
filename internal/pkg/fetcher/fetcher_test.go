package fetcher

import (
	"io"
	"time"
	"net/http"
	"net/http/httptest"
	"log"
	"testing"
)

func TestFetch(t *testing.T) {
	// Test cases
	tests := []struct {
		shortUrl string
	}{
		{"google.com"}, // Use a domain suitable for testing
	}

	for _, tt := range tests {
		t.Run(tt.shortUrl, func(t *testing.T) {
			htmlContent, err := Fetch(tt.shortUrl)
			if err != nil {
				t.Errorf("Failed to fetch HTML content from URL: %v", tt.shortUrl)
			}
			log.Printf("HTML content fetched from URL: %v", htmlContent)
			if len(htmlContent) == 0 {
				t.Errorf("Failed to fetch HTML content from URL: %v", tt.shortUrl)
			}
		})
	}
}

func TestWaitForPermission(t *testing.T) {
    testCases := []struct {
        name            string
        robotsTxt       string
        robotsTxtStatus int
        urlPath         string
        expectedError   error
    }{
        {
            name: "Allow all",
            robotsTxt: `User-agent: *
Allow: /`,
            robotsTxtStatus: http.StatusOK,
            urlPath:         "/test",
            expectedError:   nil,
        },
        {
            name: "Disallow all",
            robotsTxt: `User-agent: *
Disallow: /`,
            robotsTxtStatus: http.StatusOK,
            urlPath:         "/test",
            expectedError:   ErrCrawlingDisallowed,
        },
        {
            name: "Disallow Crawlerbot",
            robotsTxt: `User-agent: Crawlerbot
Disallow: /`,
            robotsTxtStatus: http.StatusOK,
            urlPath:         "/test",
            expectedError:   ErrCrawlingDisallowed,
        },
        {
            name: "Allow Crawlerbot",
            robotsTxt: `User-agent: Crawlerbot
Allow: /`,
            robotsTxtStatus: http.StatusOK,
            urlPath:         "/test",
            expectedError:   nil,
        },
        {
            name:            "No robots.txt",
            robotsTxt:       "",
            robotsTxtStatus: http.StatusNotFound,
            urlPath:         "/test",
            expectedError:   nil, // Should allow crawling if robots.txt is not found
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Override sleepFunc to avoid actual sleep during tests
            originalSleepFunc := sleepFunc
            defer func() { sleepFunc = originalSleepFunc }()
            sleepFunc = func(d time.Duration) {
                // No-op during tests
            }

            // Set up a test server
            testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
                if r.URL.Path == "/robots.txt" {
                    w.Header().Set("Content-Type", "text/plain")
                    w.WriteHeader(tc.robotsTxtStatus)
                    io.WriteString(w, tc.robotsTxt)
                    return
                }
                // For other paths, just respond with 200 OK
                w.WriteHeader(http.StatusOK)
                io.WriteString(w, "OK")
            }))
            defer testServer.Close()

            // Build the URL to test
            testURL := testServer.URL + tc.urlPath

            // Clear the robotsCache to avoid interference between tests
            robotsCacheMutex.Lock()
            robotsCache = make(map[string]*RobotsData)
            robotsCacheMutex.Unlock()

            // Call waitForPermission
            err := waitForPermission(testURL)

            if err != tc.expectedError {
                t.Errorf("Expected error %v, got %v", tc.expectedError, err)
            }
        })
    }
}

func TestFetchWithCrawlDelay(t *testing.T) {
    var requestTimes []time.Time
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/robots.txt" {
            w.Write([]byte(`User-agent: Crawlerbot
Crawl-delay: 2
Allow: /`))
            return
        }
        requestTimes = append(requestTimes, time.Now())
        w.Write([]byte("OK"))
    }))
    defer server.Close()

    testURL := server.URL + "/test"

    for i := 0; i < 3; i++ {
        Fetch(testURL)
    }

    if len(requestTimes) >= 2 {
        elapsed := requestTimes[1].Sub(requestTimes[0])
        if elapsed < 2*time.Second {
            t.Errorf("Expected at least 2 seconds between requests, got %v", elapsed)
        }
    }
}
