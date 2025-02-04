package fetcher

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"
	"golang.org/x/net/html"
	"webcrawler/internal/pkg/types"
)

// Tests the main function that walks the HTML.
func TestTraverseAndExtractPageContent(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		baseURL          string
		expectError      bool
		expectedTitle    string
		expectedLanguage string
		expectedIsSecure bool
	}{
		{
			name: "English content with internal & external links, headings, images, and JSON-LD",
			content: `<html lang="en">
				<head>
					<title>Test Page</title>
					<base href="https://example.com/"/>
					<meta charset="UTF-8">
					<meta name="description" content="A simple test page">
				</head>
				<body>
					<p>Some visible text.</p>
					<a href="/internal">Internal Link</a>
					<a href="http://external.com/page">External Link</a>
					<a href="https://facebook.com/profile">Facebook Link</a>
					<img alt="Image Alt Text" src="image.jpg"/>
					<h1>Heading 1</h1>
					<script type="application/ld+json">
						{"@context": "https://schema.org", "@type": "WebPage"}
					</script>
				</body>
			</html>`,
			baseURL:          "https://example.com",
			expectError:      false,
			expectedTitle:    "Test Page",
			expectedLanguage: "en",
			expectedIsSecure: true,
		},
		{
			name: "Non-English content should error",
			content: `<html lang="es">
				<head><title>Test Page</title></head>
				<body><p>Hola</p></body>
			</html>`,
			baseURL:     "https://example.com",
			expectError: true,
		},
		{
			name: "Title contains filtered term",
			content: `<html lang="en">
				<head><title>Porn Site</title></head>
				<body><p>Content</p></body>
			</html>`,
			baseURL:     "https://example.com",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pageData, err := traverseAndExtractPageContent(tc.content, tc.baseURL)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pageData.Title != tc.expectedTitle {
				t.Errorf("expected title %q, got %q", tc.expectedTitle, pageData.Title)
			}
			if pageData.Language != tc.expectedLanguage {
				t.Errorf("expected language %q, got %q", tc.expectedLanguage, pageData.Language)
			}
			if pageData.IsSecure != tc.expectedIsSecure {
				t.Errorf("expected IsSecure=%v, got %v", tc.expectedIsSecure, pageData.IsSecure)
			}

			if tc.name == "English content with internal & external links, headings, images, and JSON-LD" {
				if len(pageData.InternalLinks) != 1 {
					t.Errorf("expected 1 internal link, got %d", len(pageData.InternalLinks))
				}
				if len(pageData.ExternalLinks) != 2 {
					t.Errorf("expected 2 external links, got %d", len(pageData.ExternalLinks))
				}
				if len(pageData.SocialLinks) != 1 {
					t.Errorf("expected 1 social link, got %d", len(pageData.SocialLinks))
				}
				if len(pageData.Headings["h1"]) != 1 || pageData.Headings["h1"][0] != "Heading 1" {
					t.Errorf("expected h1 heading 'Heading 1', got %v", pageData.Headings["h1"])
				}
				if len(pageData.AltTexts) != 1 || pageData.AltTexts[0] != "Image Alt Text" {
					t.Errorf("expected alt text 'Image Alt Text', got %v", pageData.AltTexts)
				}
				if len(pageData.StructuredData) != 1 {
					t.Errorf("expected one structured data entry, got %d", len(pageData.StructuredData))
				}
			}
		})
	}
}

// Verifies that getAttribute returns the correct attribute value.
func TestGetAttribute(t *testing.T) {
	node := &html.Node{
		Attr: []html.Attribute{
			{Key: "HREF", Val: "http://example.com"},
			{Key: "class", Val: "btn"},
		},
	}
	got := getAttribute(node, "href")
	if got != "http://example.com" {
		t.Errorf("expected %q, got %q", "http://example.com", got)
	}
	got = getAttribute(node, "class")
	if got != "btn" {
		t.Errorf("expected %q, got %q", "btn", got)
	}
	got = getAttribute(node, "nonexistent")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// Checks that normalizeText removes extra whitespace.
func TestNormalizeText(t *testing.T) {
	input := "  This  is \n a   test \n"
	expected := "This is a test"
	got := normalizeText(input)
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// Builds a simple HTML snippet and verifies text extraction.
func TestExtractNodeText(t *testing.T) {
	htmlStr := `<div>Hello <span>World</span>!</div>`
	node, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}
	// Find the first <div> node.
	var div *html.Node
	var findDiv func(*html.Node)
	findDiv = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "div" && div == nil {
			div = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findDiv(c)
		}
	}
	findDiv(node)
	if div == nil {
		t.Fatal("div node not found")
	}
	got := extractNodeText(div)
	expected := "Hello World!"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// Ensures that only links with known social domains are returned.
func TestFilterSocialLinks(t *testing.T) {
	links := []string{
		"https://facebook.com/profile",
		"https://twitter.com/user",
		"https://example.com/page",
		"https://instagram.com/pic",
		"https://linkedin.com/in/someone",
		"https://other.com",
	}
	filtered := filterSocialLinks(links)
	expected := []string{
		"https://facebook.com/profile",
		"https://twitter.com/user",
		"https://instagram.com/pic",
		"https://linkedin.com/in/someone",
	}

	if len(filtered) != len(expected) {
		t.Errorf("expected %d social links, got %d", len(expected), len(filtered))
	}
	for _, link := range expected {
		found := false
		for _, f := range filtered {
			if f == link {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected social link %q not found", link)
		}
	}
}

// Ensures that the function correctly finds the <html> element.
func TestFindHTMLNode(t *testing.T) {
	htmlStr := `<html lang="en"><head></head><body></body></html>`
	node, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}
	htmlNode := findHTMLNode(node)
	if htmlNode == nil {
		t.Fatal("html node not found")
	}
	if htmlNode.Data != "html" {
		t.Errorf("expected html node, got %q", htmlNode.Data)
	}
}

// Checks that titles containing filtered terms are rejected.
func TestCheckTitleFilter(t *testing.T) {
	err := checkTitleFilter("This is a safe title")
	if err != nil {
		t.Errorf("unexpected error for safe title: %v", err)
	}
	err = checkTitleFilter("This site is XXX rated")
	if err == nil {
		t.Error("expected error for filtered title, got nil")
	}
}

// Verifies that a heading node’s text is stored properly.
func TestStoreHeading(t *testing.T) {
	textNode := &html.Node{
		Type: html.TextNode,
		Data: "Heading Test",
	}
	node := &html.Node{
		Type:       html.ElementNode,
		Data:       "h1",
		FirstChild: textNode,
		LastChild:  textNode,
	}
	var pd types.PageData
	storeHeading(node, &pd)
	if len(pd.Headings["h1"]) != 1 || pd.Headings["h1"][0] != "Heading Test" {
		t.Errorf("expected heading 'Heading Test', got %v", pd.Headings["h1"])
	}
}

// Verifies that a canonical link is processed correctly.
func TestParseLink(t *testing.T) {
	node := &html.Node{
		Type: html.ElementNode,
		Data: "link",
		Attr: []html.Attribute{
			{Key: "rel", Val: "canonical"},
			{Key: "href", Val: "/canonical"},
		},
	}
	var pd types.PageData
	base, _ := url.Parse("https://example.com/path")
	parseLink(node, &pd, base)
	expected := "https://example.com/canonical"
	if pd.CanonicalURL != expected {
		t.Errorf("expected canonical URL %q, got %q", expected, pd.CanonicalURL)
	}
}

// Verifies that JSON-LD script content is captured.
func TestParseScript(t *testing.T) {
	node := &html.Node{
		Type: html.ElementNode,
		Data: "script",
		Attr: []html.Attribute{
			{Key: "type", Val: "application/ld+json"},
		},
		FirstChild: &html.Node{
			Type: html.TextNode,
			Data: `{"@context": "https://schema.org"}`,
		},
	}
	var pd types.PageData
	parseScript(node, &pd)
	if len(pd.StructuredData) != 1 {
		t.Errorf("expected 1 structured data entry, got %d", len(pd.StructuredData))
	}
	if pd.StructuredData[0] != `{"@context": "https://schema.org"}` {
		t.Errorf("unexpected structured data: %q", pd.StructuredData[0])
	}
}

// Checks that various meta tags (charset, description, OpenGraph) are processed.
func TestParseMetaTags(t *testing.T) {
	nodeCharset := &html.Node{
		Type: html.ElementNode,
		Data: "meta",
		Attr: []html.Attribute{
			{Key: "charset", Val: "UTF-8"},
		},
	}
	var pd types.PageData
	parseMetaTags(nodeCharset, &pd)
	if pd.Charset != "UTF-8" {
		t.Errorf("expected charset UTF-8, got %q", pd.Charset)
	}

	nodeDesc := &html.Node{
		Type: html.ElementNode,
		Data: "meta",
		Attr: []html.Attribute{
			{Key: "name", Val: "description"},
			{Key: "content", Val: "Test description"},
		},
	}
	pd = types.PageData{}
	parseMetaTags(nodeDesc, &pd)
	if pd.MetaDescription != "Test description" {
		t.Errorf("expected meta description 'Test description', got %q", pd.MetaDescription)
	}

	nodeOG := &html.Node{
		Type: html.ElementNode,
		Data: "meta",
		Attr: []html.Attribute{
			{Key: "property", Val: "og:title"},
			{Key: "content", Val: "OG Title"},
		},
	}
	pd = types.PageData{}
	parseMetaTags(nodeOG, &pd)
	if pd.OpenGraph == nil || pd.OpenGraph["og:title"] != "OG Title" {
		t.Errorf("expected og:title 'OG Title', got %v", pd.OpenGraph)
	}
}

// Calls parseTimestamps directly to ensure dates are parsed.
func TestParseTimestamps(t *testing.T) {
	var pd types.PageData
	now := time.Now().UTC()
	ts := now.Format(time.RFC3339)

	parseTimestamps("article:published_time", ts, &pd)
	if pd.DatePublished.IsZero() {
		t.Errorf("expected published time to be set")
	}
	pd = types.PageData{}
	parseTimestamps("article:modified_time", ts, &pd)
	if pd.DateModified.IsZero() {
		t.Errorf("expected modified time to be set")
	}
}

// Checks that text nodes are appended except when inside ignored elements.
func TestHandleTextNode(t *testing.T) {
	var buf bytes.Buffer
	// Create a text node with a normal parent.
	parent := &html.Node{Data: "p"}
	textNode := &html.Node{Type: html.TextNode, Data: "Hello", Parent: parent}
	handleTextNode(textNode, &buf)
	if buf.String() != "Hello " {
		t.Errorf("expected 'Hello', got %q", buf.String())
	}

	// Reset and test that text in a <script> is ignored.
	buf.Reset()
	scriptParent := &html.Node{Data: "script"}
	scriptText := &html.Node{Type: html.TextNode, Data: "Should be ignored", Parent: scriptParent}
	handleTextNode(scriptText, &buf)
	if buf.String() != "" {
		t.Errorf("expected empty string for script text, got %q", buf.String())
	}
}
