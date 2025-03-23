package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
	"webcrawler/internal/pkg/types"
	"webcrawler/internal/pkg/utils"
	"golang.org/x/net/html"
)

type UserAgentData struct {
	UserAgent string `json:"userAgent"`
}

const (
	maxBodySize  = 2 * 1024 * 1024 // 2 MB
	maxParseTime = 5 * time.Second
	defaultMaxRedirects = 3
)

var (
	// Shared Chrome instance variables
	browserCancel context.CancelFunc
	allocCancel   context.CancelFunc
	userAgentData []UserAgentData

	// HTTP client with custom settings
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 5 * time.Second,
			IdleConnTimeout:       5 * time.Second,
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   10,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Check for redirect loops
			newURL := req.URL.String()
			for _, prevReq := range via {
				if prevReq.URL.String() == newURL {
					return fmt.Errorf("redirect loop detected: %s", newURL)
				}
			}
			
			// Check redirect limit
			if len(via) >= defaultMaxRedirects {
				return fmt.Errorf("reached maximum of %d redirects", defaultMaxRedirects)
			}
			
			return nil
		},
	}
)

// Initialize the fetcher module by loading prerequisites.
func Init() error {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get this file's path at runtime")
	}

	projectRoot := filepath.Dir(filename)
	userAgentsPath := filepath.Join(projectRoot, "data", "userAgents.json")
	
	// Load user agents
	jsonFile, err := os.Open(userAgentsPath)
	if err != nil {
		return fmt.Errorf("failed to read user agents file: %v", err)
	}
	defer jsonFile.Close()
	if err := json.NewDecoder(jsonFile).Decode(&userAgentData); err != nil {
		return fmt.Errorf("error decoding user agents JSON: %v", err)
	}
	return nil
}

// Fetch orchestrates the fetching process.
// It tries using the HTTP client and falls back to chromedp if necessary.
func Fetch(context context.Context, shortUrl string) (types.PageData, error) {

	fullURL, err := utils.BuildFullUrl(shortUrl)

	if err != nil {
		fmt.Printf("Failed to build full URL from short URL %v: %v\n", shortUrl, err)
		return types.PageData{}, fmt.Errorf("failed to build full URL from short URL %v: %v", shortUrl, err)
	}

	// Initialize PageData
	var pageData types.PageData
	pageData.URL = fullURL

	// Wait for permission from the rate limiter
	err = waitForPermission(context, fullURL)
	if err != nil {
		fmt.Printf("Error in rate limiter for URL %s: %v\n", fullURL, err)
		if errors.Is(err, ErrCrawlingDisallowed) {
			log.Printf("Crawling disallowed for URL: %s", fullURL)
			return types.PageData{}, err
		}
		return types.PageData{}, fmt.Errorf("error in rate limiter for URL [%s] Cause: [%v]", fullURL, err)
	}

	// Attempt to fetch content using HTTP client
	startTime := time.Now()
	content, err := fetchContent(context, fullURL)
	pageData.LoadTime = time.Since(startTime)
	if err != nil {
		log.Printf("HTTP fetch failed for URL [%s] Cause: [%v]", fullURL, err)
		return types.PageData{}, err
	}

	// Extract data from content
	pd, err := extractPageData(content, fullURL)
	if err != nil {
		return types.PageData{}, errors.New(err.Error())
	}
	pd.URL = fullURL
	pageData = pd

	return pageData, nil
}

// Fetches the page content using the HTTP client.
func fetchContent(context context.Context, fullURL string) (string, error) {
	req, err := http.NewRequestWithContext(context, "GET", fullURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %v", err)
	}
	req.Header.Set("User-Agent", getRandomUserAgent())

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL %s: %v", fullURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, maxBodySize) // Limit body size

	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	// Check if we hit the limit and log a warning if so
	if len(bodyBytes) == int(maxBodySize) {
		log.Printf("Warning: response for %s was truncated to %d bytes", fullURL, maxBodySize)
	}

	content := string(bodyBytes)
	// Validate that the content is valid UTF-8.
	if !utf8.ValidString(content) {
		return "", fmt.Errorf("invalid UTF-8 content for URL %s", fullURL)
	}

	return content, nil
}

// Tries to parse HTML, returns an error if parsing fails or takes too long.
func parseHTMLWithTimeout(content string, maxDuration time.Duration) (*html.Node, error) {
	done := make(chan struct{})
	var doc *html.Node
	var err error

	go func() {
		doc, err = html.Parse(strings.NewReader(content))
		close(done)
	}()

	select {
	case <-done:
		// Finished parsing
		return doc, err
	case <-time.After(maxDuration):
		return nil, fmt.Errorf("HTML parsing took longer than %v", maxDuration)
	}
}

// Extracts data from HTML content and populates PageData.
func extractPageData(content, baseURL string) (types.PageData, error) {
	return traverseAndExtractPageContent(content, baseURL)
}

// Gets a random user agent from the user agents list.
func getRandomUserAgent() string {
	if len(userAgentData) == 0 {
		log.Fatalf("No user agents loaded")
	}
	return userAgentData[rand.Intn(len(userAgentData))].UserAgent
}

// Gracefully closes the browser instance and releases resources.
func Shutdown() {
	if browserCancel != nil {
		browserCancel()
		browserCancel = nil
	}
	if allocCancel != nil {
		allocCancel()
		allocCancel = nil
	}
}
