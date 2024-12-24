package fetcher

import (
    "os"
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
    "math/rand"
    "encoding/json"
    "github.com/chromedp/chromedp"
    "golang.org/x/net/html"
    "golang.org/x/net/html/atom"
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
        Timeout: 15 * time.Second,
        Transport: &http.Transport{
            MaxIdleConnsPerHost: 10,
        },
    }
)

func Init() {
    // Load user agents
    jsonFile, err := os.Open("internal/pkg/fetcher/data/userAgents.json")
    if err != nil {
        log.Fatalf("failed to read user agents file: %v", err)
    }
    defer jsonFile.Close()
    if err := json.NewDecoder(jsonFile).Decode(&uaData); err != nil {
        log.Fatalf("Error decoding JSON: %v", err)
    }
}

// Fetch orchestrates the fetching process.
// It tries using the HTTP client and falls back to chromedp if necessary.
func Fetch(shortUrl string) (PageData, error) {
    fullURL, err := buildFullUrl(shortUrl)
    if err != nil {
        return PageData{}, fmt.Errorf("failed to build full URL from short URL %v: %v", shortUrl, err)
    }

    // Initialize PageData
    var pageData PageData
    pageData.URL = fullURL
    pageData.IsSecure = strings.HasPrefix(fullURL, "https://")

    // Wait for permission from the rate limiter
    err = waitForPermission(fullURL)
    if err != nil {
        if errors.Is(err, ErrCrawlingDisallowed) {
            log.Printf("Crawling disallowed for URL: %s", fullURL)
            return PageData{}, err
        }
        return PageData{}, fmt.Errorf("error in rate limiter for URL %s: %v", fullURL, err)
    }

    // Attempt to fetch content using HTTP client
    startTime := time.Now()
    content, err := fetchContent(fullURL)
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
    log.Printf("Data insufficient for URL %s, falling back to chromedp", fullURL)
    startTime = time.Now()
    renderedContent, err := fetchRenderedContent(fullURL)
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
func fetchContent(fullURL string) (string, error) {
    req, err := http.NewRequest("GET", fullURL, nil)
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

    bodyBytes, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", fmt.Errorf("failed to read response body: %v", err)
    }

    content := string(bodyBytes)
    return content, nil
}

// Checks if the extracted PageData is sufficient.
func IsDataSufficient(pd PageData) bool {
    minContentLength := 50
    if len(pd.VisibleText) >= minContentLength && pd.Title != "" {
        return true
    }
    return false
}

// Extracts data from HTML content and populates PageData.
func ExtractPageData(content, baseURL string) (PageData, error) {
    var pd PageData
    pd.URL = baseURL

    doc, err := html.Parse(strings.NewReader(content))
    if err != nil {
        return pd, err
    }

    // Extract various data
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
var fetchRenderedContent = func(fullURL string) (string, error) {
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
