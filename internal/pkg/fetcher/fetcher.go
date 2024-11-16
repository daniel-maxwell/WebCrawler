package fetcher

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "io"
    "log"
    "net/http"
    "net/url"
    "strings"
    "sync"
    "time"
    "github.com/chromedp/chromedp"
    "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const userAgent = "SearchEngineCrawler/1.0"

var (
    // Shared Chrome instance variables
    browserCtx    context.Context
    browserCancel context.CancelFunc
    allocCancel   context.CancelFunc
    chromeOnce    sync.Once

    // HTTP client with custom settings
    httpClient = &http.Client{
        Timeout: 15 * time.Second,
        Transport: &http.Transport{
            MaxIdleConnsPerHost: 10,
        },
    }
)

// Orchestrates the fetching process.
// First tries using the HTTP client and falls back to chromedp if necessary.
func Fetch(shortUrl string) (string, error) {
    fullURL, err := buildFullUrl(shortUrl)
    if err != nil {
        return "", fmt.Errorf("failed to build full URL from short URL %v: %v", shortUrl, err)
    }

    // Wait for permission from the rate limiter
    err = waitForPermission(fullURL)
    if err != nil {
        if errors.Is(err, ErrCrawlingDisallowed) {
            log.Printf("Crawling disallowed for URL: %s", fullURL)
            return "", err
        }
        return "", fmt.Errorf("error in rate limiter for URL %s: %v", fullURL, err)
    }

    // Attempt to fetch content using HTTP client
    content, err := fetchContent(fullURL)
    if err != nil {
        log.Printf("HTTP fetch failed for URL %s: %v", fullURL, err)
        return "", err
    }

    // Check if content is sufficient
    if IsContentSufficient(content) {
        // Extract meaningful text for indexing
        textContent := ExtractVisibleTextFromString(content)
        return textContent, nil
    }

    // Content is insufficient; fallback to rendering with chromedp
    log.Printf("Content insufficient for URL %s, falling back to chromedp", fullURL)
    renderedContent, err := fetchRenderedContent(fullURL)
    if err != nil {
        return "", fmt.Errorf("failed to fetch rendered content from URL %s: %v", fullURL, err)
    }

    // Extract meaningful text from rendered content
    textContent := ExtractVisibleTextFromString(renderedContent)
    return textContent, nil
}

// Helper function to extract text from a string of HTML content
func ExtractVisibleTextFromString(content string) string {
    doc, err := html.Parse(strings.NewReader(content))
    if err != nil {
        return ""
    }
    return ExtractVisibleText(doc)
}

// FetchContent fetches the page content using the HTTP client.
func fetchContent(fullURL string) (string, error) {
    req, err := http.NewRequest("GET", fullURL, nil)
    if err != nil {
        return "", fmt.Errorf("failed to create HTTP request: %v", err)
    }
    req.Header.Set("User-Agent", userAgent)

    resp, err := httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("failed to fetch URL %s: %v", fullURL, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return "", fmt.Errorf("received non-200 response code: %d", resp.StatusCode)
    }

    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response body: %v", err)
    }

    content := string(bodyBytes)
    return content, nil
}

// IsContentSufficient checks if the fetched content is sufficient.
// This can be customized based on specific criteria.
func IsContentSufficient(content string) bool {
    // Parse the HTML content
    doc, err := html.Parse(strings.NewReader(content))
    if err != nil {
        return false
    }

    // Extract visible text
    visibleText := ExtractVisibleText(doc)

    // Define a threshold for minimum content length
    minContentLength := 200 // Adjust this value as needed

    // Check if the visible text meets the threshold
    if len(visibleText) >= minContentLength {
        return true
    }

    return false
}

func ExtractVisibleText(n *html.Node) string {
    var buf bytes.Buffer
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.TextNode && !isNonVisibleParent(n.Parent) {
            text := strings.TrimSpace(n.Data)
            if len(text) > 0 {
                buf.WriteString(text + " ")
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(n)
    return buf.String()
}

// isNonVisibleParent checks if a node or any of its ancestors are non-visible.
func isNonVisibleParent(n *html.Node) bool {
    for ; n != nil; n = n.Parent {
        if n.Type == html.ElementNode {
            switch n.DataAtom {
            case atom.Script, atom.Style, atom.Head, atom.Meta, atom.Link, atom.Noscript:
                return true
            }
            // Add checks for 'display: none' if CSS parsing is implemented
        }
    }
    return false
}

// NodeTextContent extracts the text content from an HTML node.
func nodeTextContent(n *html.Node) string {
    var buf bytes.Buffer
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.TextNode {
            buf.WriteString(n.Data)
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(n)
    return buf.String()
}

// FetchRenderedContent fetches the page content by rendering it using chromedp.
func fetchRenderedContent(fullURL string) (string, error) {
    // Initialize Chrome instance (only once per process)
    chromeOnce.Do(initChrome)

    // Create a new context for this fetch operation derived from the browser context
    taskCtx, taskCancel := chromedp.NewContext(browserCtx)
    defer taskCancel()

    // Add a timeout to the context
    taskCtx, timeoutCancel := context.WithTimeout(taskCtx, 30*time.Second)
    defer timeoutCancel()

    // Fetch the rendered HTML content
    var content string
    err := chromedp.Run(taskCtx,
        chromedp.Navigate(fullURL),
        chromedp.WaitReady("body"),
        chromedp.OuterHTML("html", &content, chromedp.ByQuery),
    )
    if err != nil {
        return "", fmt.Errorf("failed to fetch rendered content from URL %s: %v", fullURL, err)
    }

    return content, nil
}

// buildFullUrl constructs the full URL from a short URL.
func buildFullUrl(shortUrl string) (string, error) {
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

// initChrome initializes the Chrome browser instance.
func initChrome() {
    // Set up Chrome options
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.DisableGPU,
        chromedp.Headless,
        chromedp.NoSandbox,
        chromedp.Flag("blink-settings", "imagesEnabled=false"),
        chromedp.UserAgent(userAgent),
    )

    // Create the Chrome ExecAllocator context
    allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
    allocCancel = cancelAlloc

    // Create the browser context
    browserCtx, browserCancel = chromedp.NewContext(allocCtx)

    // Start the browser instance
    if err := chromedp.Run(browserCtx); err != nil {
        log.Fatalf("Failed to start Chrome: %v", err)
    }
}

// Shutdown gracefully closes the browser instance and releases resources.
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
