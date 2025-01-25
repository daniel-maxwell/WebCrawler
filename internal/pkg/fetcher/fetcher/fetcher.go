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
    "time"
    "math/rand"
    "encoding/json"
    "golang.org/x/net/html"
    "golang.org/x/net/html/atom"
    "webcrawler/internal/pkg/types"
    "webcrawler/internal/pkg/utils"
)

type UAData struct {
    UA  string  `json:"ua"`
    Pct float64 `json:"pct"`
}

var (
    // Shared Chrome instance variables
    browserCancel context.CancelFunc
    allocCancel   context.CancelFunc
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
    jsonFile, err := os.Open("internal/pkg/fetcher/fetcher/data/userAgents.json")
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
func Fetch(context context.Context, shortUrl string) (types.PageData, error) {

    fullURL, err := utils.BuildFullUrl(shortUrl)

    if err != nil {
        fmt.Printf("Failed to build full URL from short URL %v: %v\n", shortUrl, err)
        return types.PageData{}, fmt.Errorf("failed to build full URL from short URL %v: %v", shortUrl, err)
    }

    // Initialize PageData
    var pageData types.PageData
    pageData.URL = fullURL
    pageData.IsSecure = strings.HasPrefix(fullURL, "https://")

    // Wait for permission from the rate limiter
    err = waitForPermission(context, fullURL)
    if err != nil {
        fmt.Printf("Error in rate limiter for URL %s: %v\n", fullURL, err)
        if errors.Is(err, ErrCrawlingDisallowed) {
            log.Printf("Crawling disallowed for URL: %s", fullURL)
            return types.PageData{}, err
        }
        return types.PageData{}, fmt.Errorf("error in rate limiter for URL %s: %v", fullURL, err)
    }

    // Attempt to fetch content using HTTP client
    startTime := time.Now()
    content, err := fetchContent(context, fullURL)
    pageData.LoadTime = time.Since(startTime)
    if err != nil {
        log.Printf("HTTP fetch failed for URL %s: %v", fullURL, err)
        return types.PageData{}, err
    }

    // Extract data from content
    pd, err := ExtractPageData(content, fullURL)
    if err != nil {
        return types.PageData{}, fmt.Errorf("failed to extract page data from URL %s: %v", fullURL, err)
    }
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
    return content, nil
}

// Checks if the extracted PageData is sufficient.
func IsDataSufficient(pageData types.PageData) bool {
    return true
    //return len(pageData.VisibleText) >= 20
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
func ExtractPageData(content, baseURL string) (types.PageData, error) {
    var pageData types.PageData
    pageData.URL = baseURL

    doc, err := parseHTMLWithTimeout(content, maxParseTime)
    if err != nil {
        fmt.Printf("Failed to parse HTML content for URL: [%v] | Reason: [%v]\n", baseURL, err)
        return pageData, err
    }

    // Extract page data
    pageData.Title = extractTitle(doc)
    pageData.MetaDescription, pageData.MetaKeywords, pageData.RobotsMeta, pageData.Charset = extractMetaTags(doc)
    pageData.CanonicalURL = extractCanonicalURL(doc)
    pageData.Headings = extractHeadings(doc)
    pageData.AltTexts = extractAltTexts(doc)
    pageData.AnchorTexts, pageData.InternalLinks, pageData.ExternalLinks = extractAnchorTextsAndLinks(doc, baseURL)
    pageData.OpenGraph = extractOpenGraphData(doc)
    pageData.Author = extractAuthor(doc)
    pageData.DatePublished, pageData.DateModified = extractDates(doc)
    pageData.StructuredData = extractStructuredData(doc)
    pageData.VisibleText = ExtractVisibleText(doc)
    pageData.IsSecure = strings.HasPrefix(baseURL, "https://")
    pageData.Language = extractLanguage(doc)
    pageData.SocialLinks = extractSocialLinks(pageData.ExternalLinks)

    return pageData, nil
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
func ExtractVisibleText(node *html.Node) string {
    var buf bytes.Buffer
    var extract func(*html.Node)
    extract = func(node *html.Node) {
        if node.Type == html.TextNode && !isNonVisibleParent(node.Parent) {
            text := strings.TrimSpace(node.Data)
            if len(text) > 0 {
                buf.WriteString(text + " ")
            }
        }
        for child := node.FirstChild; child != nil; child = child.NextSibling {
            extract(child)
        }
    }
    extract(node)
    return buf.String()
}

// Checks if a node or any of its ancestors are non-visible.
func isNonVisibleParent(node *html.Node) bool {
    for ; node != nil; node = node.Parent {
        if node.Type == html.ElementNode {
            switch node.DataAtom {
            case atom.Script, atom.Style, atom.Head, atom.Meta, atom.Link, atom.Noscript:
                return true
            }
            // Add checks for 'display: none' if CSS parsing is implemented
        }
    }
    return false
}

func getRandomUserAgent() string {
    if len(uaData) == 0 {
        log.Fatalf("No user agents loaded")
    }
    return uaData[rand.Intn(len(uaData))].UA
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
