package fetcher

import (
    "os"
    "bytes"
    "context"
    "errors"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "strings"
    "sync"
    "time"
    "math/rand"
    "encoding/json"
    "github.com/chromedp/chromedp"
    "golang.org/x/net/html"
    "golang.org/x/net/html/atom"
    "webcrawler/internal/pkg/utils"
)

type UAData struct {
    UA  string  `json:"ua"`
    Pct float64 `json:"pct"`
}

var (
    // Shared Chrome instance variables
    browserCtx    context.Context
    browserCancel context.CancelFunc
    allocCancel   context.CancelFunc
    chromeOnce    sync.Once
    uaData        []UAData

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
    }
)

const (
    maxBodySize = 2 * 1024 * 1024 // 2 MB
    maxParseTime = 5 * time.Second
)

func Init() error {
    // Load user agents
    jsonFile, err := os.Open("internal/pkg/fetcher/data/userAgents.json")
    if err != nil {
        return fmt.Errorf("failed to read user agents file: %v", err)
    }
    defer jsonFile.Close()
    if err := json.NewDecoder(jsonFile).Decode(&uaData); err != nil {
        return fmt.Errorf("error decoding user agents JSON: %v", err)
    }
    return nil
}

// Fetch orchestrates the fetching process.
// It tries using the HTTP client and falls back to chromedp if necessary.
func Fetch(ctx context.Context, shortUrl string) (PageData, error) {

    fullURL, err := utils.BuildFullUrl(shortUrl)

    if err != nil {
        fmt.Printf("Failed to build full URL from short URL %v: %v\n", shortUrl, err)
        return PageData{}, fmt.Errorf("failed to build full URL from short URL %v: %v", shortUrl, err)
    }

    // Initialize PageData
    var pageData PageData
    pageData.URL = fullURL
    pageData.IsSecure = strings.HasPrefix(fullURL, "https://")

    // Wait for permission from the rate limiter
    err = waitForPermission(ctx, fullURL)
    if err != nil {
        fmt.Printf("Error in rate limiter for URL %s: %v\n", fullURL, err)
        if errors.Is(err, ErrCrawlingDisallowed) {
            log.Printf("Crawling disallowed for URL: %s", fullURL)
            return PageData{}, err
        }
        return PageData{}, fmt.Errorf("error in rate limiter for URL %s: %v", fullURL, err)
    }

    // Attempt to fetch content using HTTP client
    startTime := time.Now()
    content, err := fetchContent(ctx, fullURL)
    pageData.LoadTime = time.Since(startTime)
    if err != nil {
        log.Printf("HTTP fetch failed for URL %s: %v", fullURL, err)
        return PageData{}, err
    }

    // Extract data from content
    pd, err := ExtractPageData(content, fullURL)
    if err != nil {
        return PageData{}, fmt.Errorf("failed to extract page data from URL %s: %v", fullURL, err)
    }
    pageData = pd

    // Check if data is sufficient
    if IsDataSufficient(pageData) {
        pageData.LastCrawled = time.Now()
        return pageData, nil
    }

    // Content is insufficient; fallback to rendering with chromedp
    startTime = time.Now()
    renderedContent, err := fetchRenderedContent(ctx, fullURL)
    pageData.LoadTime = time.Since(startTime)
    if err != nil {
        return PageData{}, fmt.Errorf("failed to fetch rendered content from URL %s: %v", fullURL, err)
    }

    // Extract data from rendered content
    pd, err = ExtractPageData(renderedContent, fullURL)
    if err != nil {
        return PageData{}, fmt.Errorf("failed to extract page data from rendered content of URL %s: %v", fullURL, err)
    }
    pageData = pd
    pageData.LastCrawled = time.Now()

    return pageData, nil
}


// Fetches the page content using the HTTP client.
func fetchContent(ctx context.Context, fullURL string) (string, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
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
    return content, nil
}

// Checks if the extracted PageData is sufficient.
func IsDataSufficient(pd PageData) bool {
    return true
    //return len(pd.VisibleText) >= 20
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
func ExtractPageData(content, baseURL string) (PageData, error) {
    var pd PageData
    pd.URL = baseURL

    doc, err := parseHTMLWithTimeout(content, maxParseTime)
    if err != nil {
        fmt.Printf("Failed to parse HTML content for URL: [%v] | Reason: [%v]\n", baseURL, err)
        return pd, err
    }

    // Extract page data
    pd.Title = extractTitle(doc)
    pd.MetaDescription, pd.MetaKeywords, pd.RobotsMeta, pd.Charset = extractMetaTags(doc)
    pd.CanonicalURL = extractCanonicalURL(doc)
    pd.Headings = extractHeadings(doc)
    pd.AltTexts = extractAltTexts(doc)
    pd.AnchorTexts, pd.InternalLinks, pd.ExternalLinks = extractAnchorTextsAndLinks(doc, baseURL)
    pd.OpenGraph = extractOpenGraphData(doc)
    pd.Author = extractAuthor(doc)
    pd.DatePublished, pd.DateModified = extractDates(doc)
    pd.StructuredData = extractStructuredData(doc)
    pd.VisibleText = ExtractVisibleText(doc)
    pd.IsSecure = strings.HasPrefix(baseURL, "https://")
    pd.Language = extractLanguage(doc)
    pd.SocialLinks = extractSocialLinks(pd.ExternalLinks)

    return pd, nil
}

// Extracts visible text from a string of HTML content
func ExtractVisibleTextFromString(content string) string {
    doc, err := html.Parse(strings.NewReader(content))
    if err != nil {
        return ""
    }
    return ExtractVisibleText(doc)
}

// Extracts visible text from an HTML node.
func ExtractVisibleText(n *html.Node) string {
    var buf bytes.Buffer
    var extract func(*html.Node)
    extract = func(n *html.Node) {
        if n.Type == html.TextNode && !isNonVisibleParent(n.Parent) {
            text := strings.TrimSpace(n.Data)
            if len(text) > 0 {
                buf.WriteString(text + " ")
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            extract(c)
        }
    }
    extract(n)
    return buf.String()
}

// Checks if a node or any of its ancestors are non-visible.
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

// FetchRenderedContent fetches the page content by rendering it using chromedp.
var fetchRenderedContent = func(ctx context.Context, fullURL string) (string, error) {
    // Initialize Chrome instance (only once per process)
    chromeOnce.Do(initChrome)

    // Create a new context for this fetch operation derived from the browser context
    childCtx, childCancel := chromedp.NewContext(browserCtx)

    // Add a timeout to the context
    combinedCtx, combinedCancel := context.WithCancel(childCtx)

    go func() {
        select {
        case <-ctx.Done():
            // If the caller’s ctx is done, cancel the combined context too
            combinedCancel()
        case <-combinedCtx.Done():
            // If combinedCtx is done first (normal completion), do nothing special
        }
    }()

    defer childCancel()
    defer combinedCancel()

    // Fetch the rendered HTML content
    var content string
    err := chromedp.Run(
        combinedCtx,
        chromedp.Navigate(fullURL),
        chromedp.WaitReady("body", chromedp.ByQuery),
        chromedp.OuterHTML("html", &content, chromedp.ByQuery),
    )
    if err != nil {
        return "", fmt.Errorf("failed to fetch rendered content from %s: %v", fullURL, err)
    }

    return content, nil
}

func getRandomUserAgent() string {
    if len(uaData) == 0 {
        log.Fatalf("No user agents loaded")
    }
    return uaData[rand.Intn(len(uaData))].UA
}

// initChrome initializes the Chrome browser instance.
func initChrome() {
    // Set up Chrome options
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.DisableGPU,
        chromedp.Headless,
        chromedp.NoSandbox,
        chromedp.Flag("blink-settings", "imagesEnabled=false"),
        chromedp.UserAgent(getRandomUserAgent()),
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
