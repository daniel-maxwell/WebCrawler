package utils

import (
	"testing"
)

func TestBuildFullUrl(t *testing.T) {
    shortUrl := "example.com/test"
    fullUrl, err := BuildFullUrl(shortUrl)
    if err != nil {
        t.Fatalf("buildFullUrl returned error: %v", err)
    }
    if fullUrl != "https://example.com/test" {
        t.Errorf("Expected full URL 'https://example.com/test', got '%s'", fullUrl)
    }
}