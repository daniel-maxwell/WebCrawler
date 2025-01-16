package utils

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Extracts the host domain from a URL.
func GetDomainFromURL(inputURL string) (string, error) {
	if !strings.HasPrefix(inputURL, "http://") && !strings.HasPrefix(inputURL, "https://") {
        inputURL = "https://" + inputURL
    }
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", errors.New("error parsing URL")
	}
	return strings.TrimPrefix(parsedURL.Hostname(), "www."), nil
}

// Constructs the full URL from a short URL.
func BuildFullUrl(shortUrl string) (string, error) {
    // Prepend scheme if missing
    if !strings.HasPrefix(shortUrl, "http://") && !strings.HasPrefix(shortUrl, "https://") {
        shortUrl = "https://" + shortUrl
    }
    parsedURL, err := url.Parse(shortUrl)
    if err != nil {
        return "", fmt.Errorf("invalid URL %v: %v", shortUrl, err)
    }
    return parsedURL.String(), nil
}