package fetcher

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetch(t *testing.T) {
	// Test cases
	tests := []struct {
		shortUrl string
	}{
		{"example.com"}, // Use a domain suitable for testing
	}

	for _, tt := range tests {
		t.Run(tt.shortUrl, func(t *testing.T) {
			htmlContent := Fetch(tt.shortUrl)
			log.Printf("HTML content fetched from URL: %v", htmlContent)
			if len(htmlContent) == 0 {
				t.Errorf("Failed to fetch HTML content from URL: %v", tt.shortUrl)
			}
		})
	}
}

func TestIsCrawlingPermitted(t *testing.T) {
	testCases := []struct {
		name            string
		robotsTxt       string
		robotsTxtStatus int
		urlPath         string
		expectedResult  bool
		expectedError   bool
	}{
		{
			name: "Allow all",
			robotsTxt: `User-agent: *
Allow: /`,
			robotsTxtStatus: http.StatusOK,
			urlPath:         "/test",
			expectedResult:  true,
			expectedError:   false,
		},
		{
			name: "Disallow all",
			robotsTxt: `User-agent: *
Disallow: /`,
			robotsTxtStatus: http.StatusOK,
			urlPath:         "/test",
			expectedResult:  false,
			expectedError:   false,
		},
		{
			name: "Disallow Crawlerbot",
			robotsTxt: `User-agent: Crawlerbot
Disallow: /`,
			robotsTxtStatus: http.StatusOK,
			urlPath:         "/test",
			expectedResult:  false,
			expectedError:   false,
		},
		{
			name: "Allow Crawlerbot",
			robotsTxt: `User-agent: Crawlerbot
Allow: /`,
			robotsTxtStatus: http.StatusOK,
			urlPath:         "/test",
			expectedResult:  true,
			expectedError:   false,
		},
		{
			name:            "No robots.txt",
			robotsTxt:       "",
			robotsTxtStatus: http.StatusNotFound,
			urlPath:         "/test",
			expectedResult:  true, // Should return true if robots.txt is not found
			expectedError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up a test server for this test case
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

			// Call the function being tested
			result, err := isCrawlingPermitted(testURL)

			if tc.expectedError && err == nil {
				t.Errorf("Expected error but got nil")
			} else if !tc.expectedError && err != nil {
				t.Errorf("Did not expect error but got %v", err)
			}

			if result != tc.expectedResult {
				t.Errorf("Expected result %v but got %v", tc.expectedResult, result)
			}
		})
	}
}
