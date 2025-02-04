package utils

import (
	"testing"
)

func TestGetDomainFromURL(t *testing.T) {
    inputURL := "https://example.com/test"
    domain, err := GetDomainFromURL(inputURL)
    if err != nil {
        t.Fatalf("getDomainFromURL returned error: %v", err)
    }
    if domain != "example.com" {
        t.Errorf("Expected domain 'example.com', got '%s'", domain)
    }
}

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
