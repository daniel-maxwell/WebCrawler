package fetcher

import (
	"log"
	"testing"
)

func TestFetch(t *testing.T) {
	// Test cases
	tests := []struct {
		shortUrl string
	}{
		{"google.com"}, // Add more URLs to expand test
	}

	for _, tt := range tests {
		t.Run(tt.shortUrl, func(t *testing.T) {
			htmlContent := Fetch(tt.shortUrl)
			log.Printf("HTML content: fetched from URL: %v", htmlContent)
			if len(htmlContent) == 0 {
				t.Errorf("Failed to fetch HTML content from URL: %v", tt.shortUrl)
			}
		})
	}
}