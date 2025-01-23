package fetcher

import (
    "context"
    //"errors"
    //"io"
    "net/http"
    "net/http/httptest"
    "net/url"
    "strings"
    "sync"
    "testing"
    "time"
    "golang.org/x/net/html"
)

/*
// Tests the case where the content fetched from the server is sufficient,
// so the fetcher does not need to use chromedp to render the page.
func TestFetch_SufficientContent(t *testing.T) {
    resetGlobals()

    // Mock server with sufficient content
    targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, `<html><head><title>Test Page</title></head><body><p>This is a test page with sufficient content.</p></body></html>`)
    }))
    defer targetServer.Close()

    // Mock robots.txt allowing all
    robotsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, `User-agent: *
Disallow:`)
    }))
    defer robotsServer.Close()

    // Save original httpClient
    originalHttpClient := httpClient
    defer func() { httpClient = originalHttpClient }()

    // Use MockRoundTripper
    httpClient = &http.Client{
        Transport: &MockRoundTripper{
            targetServer: targetServer,
            robotsServer: robotsServer,
        },
    }

    // Call Fetch
    pageData, err := Fetch(targetServer.URL)
    if err != nil {
        t.Fatalf("Fetch returned error: %v", err)
    }

    // Assertions
    if pageData.Title != "Test Page" {
        t.Errorf("Expected Title 'Test Page', got '%s'", pageData.Title)
    }
    if len(pageData.VisibleText) == 0 {
        t.Errorf("Expected VisibleText to be populated")
    }
    if pageData.LastCrawled.IsZero() {
        t.Errorf("Expected LastCrawled to be set")
    }
}*/

/*
// Tests the case where the content fetched from the server is insufficient,
// so the fetcher falls back to using chromedp to render the page.
func TestFetch_InsufficientContent_FallbackToChromedp(t *testing.T) {
    resetGlobals()

    // Mock server with insufficient content
    targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, `<html><head><title></title></head><body><p>Short.</p></body></html>`)
    }))
    defer targetServer.Close()

    // Mock robots.txt allowing all
    robotsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, `User-agent: *
Disallow:`)
    }))
    defer robotsServer.Close()

    // Save original httpClient
    originalHttpClient := httpClient
    defer func() { httpClient = originalHttpClient }()

    // Use MockRoundTripper
    httpClient = &http.Client{
        Transport: &MockRoundTripper{
            targetServer: targetServer,
            robotsServer: robotsServer,
        },
    }

    // Mock fetchRenderedContent
    originalFetchRenderedContent := fetchRenderedContent
    defer func() { fetchRenderedContent = originalFetchRenderedContent }()

    fetchRenderedContent = func(fullURL string) (string, error) {
        return `<html><head><title>Rendered Page</title></head><body><p>This is rendered content with sufficient length.</p></body></html>`, nil
    }

    // Call Fetch
    pageData, err := Fetch(targetServer.URL)
    if err != nil {
        t.Fatalf("Fetch returned error: %v", err)
    }

    // Assertions
    if pageData.Title != "Rendered Page" {
        t.Errorf("Expected Title 'Rendered Page', got '%s'", pageData.Title)
    }
    if len(pageData.VisibleText) == 0 {
        t.Errorf("Expected VisibleText to be populated")
    }
    if pageData.LastCrawled.IsZero() {
        t.Errorf("Expected LastCrawled to be set")
    }
}*/

/*
// Tests the case where crawling is disallowed by robots.txt.
func TestFetch_CrawlingDisallowed(t *testing.T) {
    resetGlobals()

    // Mock server
    targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        t.Errorf("Fetch should not have been called due to robots.txt disallow")
    }))
    defer targetServer.Close()

    // Mock robots.txt disallowing all
    robotsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, `User-agent: *
Disallow: /`)
    }))
    defer robotsServer.Close()

    // Save original httpClient
    originalHttpClient := httpClient
    defer func() { httpClient = originalHttpClient }()

    // Use MockRoundTripper
    httpClient = &http.Client{
        Transport: &MockRoundTripper{
            targetServer: targetServer,
            robotsServer: robotsServer,
        },
    }

    // Call Fetch
    _, err := Fetch(targetServer.URL)
    if err == nil {
        t.Fatalf("Expected error due to crawling disallowed, but got nil")
    }
    if !errors.Is(err, ErrCrawlingDisallowed) {
        t.Fatalf("Expected ErrCrawlingDisallowed, got %v", err)
    }
}*/

// Test extracting the title from an HTML document.
func TestExtractTitle(t *testing.T) {
    htmlContent := `<html><head><title>Test Title</title></head><body></body></html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    title := extractTitle(doc)
    if title != "Test Title" {
        t.Errorf("Expected title 'Test Title', got '%s'", title)
    }
}

// Test extracting meta tags from an HTML document.
func TestExtractMetaTags(t *testing.T) {
    htmlContent := `
    <html>
        <head>
            <meta name="description" content="Test Description">
            <meta name="keywords" content="test, keywords">
            <meta name="robots" content="index, follow">
            <meta charset="UTF-8">
        </head>
        <body></body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    metaDescription, metaKeywords, robotsMeta, charset := extractMetaTags(doc)
    if metaDescription != "Test Description" {
        t.Errorf("Expected MetaDescription 'Test Description', got '%s'", metaDescription)
    }
    if metaKeywords != "test, keywords" {
        t.Errorf("Expected MetaKeywords 'test, keywords', got '%s'", metaKeywords)
    }
    if robotsMeta != "index, follow" {
        t.Errorf("Expected RobotsMeta 'index, follow', got '%s'", robotsMeta)
    }
    if charset != "UTF-8" {
        t.Errorf("Expected Charset 'UTF-8', got '%s'", charset)
    }
}

// Test extracting the canonical URL from an HTML document.
func TestExtractCanonicalURL(t *testing.T) {
    htmlContent := `
    <html>
        <head>
            <link rel="canonical" href="https://example.com/canonical-url">
        </head>
        <body></body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    canonicalURL := extractCanonicalURL(doc)
    if canonicalURL != "https://example.com/canonical-url" {
        t.Errorf("Expected CanonicalURL 'https://example.com/canonical-url', got '%s'", canonicalURL)
    }
}

// Test extracting headings from an HTML document.
func TestExtractHeadings(t *testing.T) {
    htmlContent := `
    <html>
        <body>
            <h1>Main Heading</h1>
            <h2>Subheading 1</h2>
            <h2>Subheading 2</h2>
            <h3>Sub-subheading</h3>
        </body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    headings := extractHeadings(doc)
    if len(headings["h1"]) != 1 || headings["h1"][0] != "Main Heading" {
        t.Errorf("Expected h1 'Main Heading', got '%v'", headings["h1"])
    }
    if len(headings["h2"]) != 2 {
        t.Errorf("Expected 2 h2 headings, got '%v'", headings["h2"])
    }
    if len(headings["h3"]) != 1 || headings["h3"][0] != "Sub-subheading" {
        t.Errorf("Expected h3 'Sub-subheading', got '%v'", headings["h3"])
    }
}

// Test extracting alt texts from an HTML document.
func TestExtractAltTexts(t *testing.T) {
    htmlContent := `
    <html>
        <body>
            <img src="image1.jpg" alt="First Image">
            <img src="image2.jpg" alt="Second Image">
            <img src="image3.jpg">
        </body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    altTexts := extractAltTexts(doc)
    if len(altTexts) != 2 {
        t.Errorf("Expected 2 alt texts, got %d", len(altTexts))
    }
    if altTexts[0] != "First Image" || altTexts[1] != "Second Image" {
        t.Errorf("Unexpected alt texts: %v", altTexts)
    }
}

// Test extracting anchor texts and links from an HTML document.
func TestExtractAnchorTextsAndLinks(t *testing.T) {
    baseURL := "https://example.com"
    htmlContent := `
    <html>
        <body>
            <a href="/internal-page">Internal Link</a>
            <a href="https://external.com">External Link</a>
            <a href="#">Anchor Link</a>
        </body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    anchorTexts, internalLinks, externalLinks := extractAnchorTextsAndLinks(doc, baseURL)
    if len(anchorTexts) != 3 {
        t.Errorf("Expected 3 anchor texts, got %d", len(anchorTexts))
    }
    if len(internalLinks) != 2 { // "/internal-page" and "#"
        t.Errorf("Expected 2 internal links, got %d", len(internalLinks))
    }
    if len(externalLinks) != 1 {
        t.Errorf("Expected 1 external link, got %d", len(externalLinks))
    }
}

// Test extracting Open Graph data from an HTML document.
func TestExtractOpenGraphData(t *testing.T) {
    htmlContent := `
    <html>
        <head>
            <meta property="og:title" content="OG Title">
            <meta property="og:description" content="OG Description">
        </head>
        <body></body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    openGraph := extractOpenGraphData(doc)
    if openGraph["og:title"] != "OG Title" {
        t.Errorf("Expected og:title 'OG Title', got '%s'", openGraph["og:title"])
    }
    if openGraph["og:description"] != "OG Description" {
        t.Errorf("Expected og:description 'OG Description', got '%s'", openGraph["og:description"])
    }
}

// Test extracting the author from an HTML document.
func TestExtractAuthor(t *testing.T) {
    htmlContent := `
    <html>
        <head>
            <meta name="author" content="Test Author">
        </head>
        <body></body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    author := extractAuthor(doc)
    if author != "Test Author" {
        t.Errorf("Expected author 'Test Author', got '%s'", author)
    }
}

// Test extracting dates from an HTML document.
func TestExtractDates(t *testing.T) {
    htmlContent := `
    <html>
        <head>
            <meta property="article:published_time" content="2021-01-01T12:00:00Z">
            <meta property="article:modified_time" content="2021-01-02T12:00:00Z">
        </head>
        <body></body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    datePublished, dateModified := extractDates(doc)
    expectedPublished, _ := time.Parse(time.RFC3339, "2021-01-01T12:00:00Z")
    expectedModified, _ := time.Parse(time.RFC3339, "2021-01-02T12:00:00Z")
    if !datePublished.Equal(expectedPublished) {
        t.Errorf("Expected DatePublished '%v', got '%v'", expectedPublished, datePublished)
    }
    if !dateModified.Equal(expectedModified) {
        t.Errorf("Expected DateModified '%v', got '%v'", expectedModified, dateModified)
    }
}

// Test extracting structured data from an HTML document.
func TestExtractStructuredData(t *testing.T) {
    htmlContent := `
    <html>
        <head>
            <script type="application/ld+json">
                { "@context": "http://schema.org", "@type": "WebSite" }
            </script>
        </head>
        <body></body>
    </html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    structuredData := extractStructuredData(doc)
    if len(structuredData) != 1 {
        t.Errorf("Expected 1 structured data block, got %d", len(structuredData))
    }
    if !strings.Contains(structuredData[0], `"@type": "WebSite"`) {
        t.Errorf("Unexpected structured data content: %s", structuredData[0])
    }
}

// Test extracting the language from an HTML document.
func TestExtractLanguage(t *testing.T) {
    htmlContent := `<html lang="en"><head></head><body></body></html>`
    doc, err := html.Parse(strings.NewReader(htmlContent))
    if err != nil {
        t.Fatalf("Failed to parse HTML: %v", err)
    }
    lang := extractLanguage(doc)
    if lang != "en" {
        t.Errorf("Expected language 'en', got '%s'", lang)
    }
}

// Test extracting social links from a list of external links.
func TestExtractSocialLinks(t *testing.T) {
    externalLinks := []string{
        "https://facebook.com/test",
        "https://twitter.com/test",
        "https://example.com",
    }
    socialLinks := extractSocialLinks(externalLinks)
    if len(socialLinks) != 2 {
        t.Errorf("Expected 2 social links, got %d", len(socialLinks))
    }
    if !strings.Contains(socialLinks[0], "facebook.com") || !strings.Contains(socialLinks[1], "twitter.com") {
        t.Errorf("Unexpected social links: %v", socialLinks)
    }
}

// Test evaluating if the extracted data is sufficient.
func TestIsDataSufficient(t *testing.T) {
    pd := PageData{
        Title:       "Test Page",
        VisibleText: strings.Repeat("This is a test. ", 20),
    }
    if !IsDataSufficient(pd) {
        t.Errorf("Expected data to be sufficient")
    }

    pd2 := PageData{
        Title:       "",
        VisibleText: "Short text",
    }
    if IsDataSufficient(pd2) {
        t.Errorf("Expected data to be insufficient")
    }
}

// Test building a full URL from a short URL.

/*
// Test waiting for permission to crawl a URL.
func TestWaitForPermission(t *testing.T) {
    resetGlobals()

    // Mock robots.txt allowing all
    robotsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        io.WriteString(w, `User-agent: *
Disallow:`)
    }))
    defer robotsServer.Close()

    // Save original httpClient
    originalHttpClient := httpClient
    defer func() { httpClient = originalHttpClient }()

    // Use MockRoundTripper
    httpClient = &http.Client{
        Transport: &MockRoundTripper{
            robotsServer: robotsServer,
        },
    }

    err := waitForPermission("https://example.com/test")
    if err != nil {
        t.Fatalf("waitForPermission returned error: %v", err)
    }
}

/****************************************************************************/
/*****************************  TEST HELPERS   ******************************/
/****************************************************************************/

// MockRoundTripper intercepts HTTP requests and directs them to appropriate handlers.
type MockRoundTripper struct {
    targetServer *httptest.Server
    robotsServer *httptest.Server
}

// RoundTrip intercepts HTTP requests and directs them to the appropriate server.
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
    // Clone the request to avoid modifying the original
    reqClone := req.Clone(context.Background())
    if strings.HasSuffix(req.URL.Path, "/robots.txt") {
        reqClone.URL = &url.URL{
            Scheme: req.URL.Scheme,
            Host:   m.robotsServer.Listener.Addr().String(),
            Path:   req.URL.Path,
        }
    } else {
        reqClone.URL = &url.URL{
            Scheme: req.URL.Scheme,
            Host:   m.targetServer.Listener.Addr().String(),
            Path:   req.URL.Path,
        }
    }
    return http.DefaultTransport.RoundTrip(reqClone)
}

// Reset global variables between tests
func resetGlobals() {
    robotsCache = make(map[string]*RobotsData)
    chromeOnce = sync.Once{}
    browserCtx = nil
    browserCancel = nil
    allocCancel = nil
}

