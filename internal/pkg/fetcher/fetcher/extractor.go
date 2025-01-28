package fetcher

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"webcrawler/internal/pkg/types"
	"golang.org/x/net/html"
)

var (
	socialDomains = map[string]struct{}{
		"facebook.com": {},
		"twitter.com":  {},
		"x.com":        {},
		"instagram.com":{},
		"linkedin.com": {},
	}
	filterTerms = [6]string{"xxx", "porn", "sex", "onlyfans", "gore", "hentai"}
)

// Remains but ensure it uses context with timeout
func traverseAndExtractPageContent(content, baseURL string) (types.PageData, error) {
	var pageData types.PageData
	baseParsed, err := url.Parse(baseURL)
	if err != nil {
		return pageData, fmt.Errorf("invalid base URL: %w", err)
	}

	doc, err := parseHTMLWithTimeout(content, maxParseTime)
	if err != nil {
		return pageData, err
	}

	// Extract <base> tag first to modify base URL
	if newBase := findBaseTag(doc); newBase != nil {
		if resolved := baseParsed.ResolveReference(newBase); resolved != nil {
			baseParsed = resolved
		}
	}

	pageData.IsSecure = baseParsed.Scheme == "https"

	if !isEnglishContent(doc) {
		return pageData, errors.New("non-English content")
	}

	var (
		textBuf       bytes.Buffer
		linkBuffer    = make([]string, 0, 50)
		externalLinks = make([]string, 0, 20)
	)

	stack := []*html.Node{doc}
	for len(stack) > 0 {
		node := stack[len(stack) - 1]
		stack = stack[:len(stack) - 1]

		switch node.Type {
		case html.TextNode:
			handleTextNode(node, &textBuf)
		case html.ElementNode:
            if err := handleElement(node, &pageData, baseParsed, &linkBuffer, &externalLinks); err != nil {
                return pageData, err // Early return on error
            }
        }

		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}

	pageData.VisibleText = normalizeText(textBuf.String())
	pageData.InternalLinks = linkBuffer
	pageData.ExternalLinks = externalLinks
	pageData.SocialLinks = filterSocialLinks(externalLinks)

	return pageData, nil
}

func isEnglishContent(doc *html.Node) bool {
	htmlNode := findHTMLNode(doc)
	if htmlNode == nil {
		return true
	}

	for _, attr := range htmlNode.Attr {
		if strings.EqualFold(attr.Key, "lang") {
			lang := strings.ToLower(strings.SplitN(attr.Val, "-", 2)[0])
			return lang == "en"
		}
	}
	return true
}

func findHTMLNode(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "html" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findHTMLNode(c); found != nil {
			return found
		}
	}
	return nil
}

// Retrieves an attribute value case-insensitively with zero allocations
func getAttribute(n *html.Node, attrName string) string {
    attrName = strings.ToLower(attrName)
    for _, attr := range n.Attr {
        if strings.EqualFold(attr.Key, attrName) {
            return attr.Val
        }
    }
    return ""
}

func handleTextNode(node *html.Node, buf *bytes.Buffer) {
	parent := node.Parent
	if parent == nil {
		return
	}

	switch parent.Data {
	case "script", "style", "noscript", "template":
		return
	}

	buf.WriteString(node.Data)
}

func handleElement(node *html.Node, pd *types.PageData, base *url.URL, 
    internalLinks *[]string, externalLinks *[]string) error {
    switch node.Data {
		case "html":  // Added case for html element
			handleHtmlTag(node, pd)
		case "title":
			if err := handleTitle(node, pd); err != nil {
				return err // Propagate error upward
			}
		case "meta":
			parseMetaTags(node, pd)
		case "a":
			processAnchor(node, pd, base, internalLinks, externalLinks)
		case "img":
			parseImage(node, pd)
		case "h1", "h2", "h3", "h4", "h5", "h6":
			storeHeading(node, pd)
		case "link":
			parseLink(node, pd, base)
		case "script":
			parseScript(node, pd)
	}
	return nil
}

func handleHtmlTag(node *html.Node, pd *types.PageData) {
    for _, attr := range node.Attr {
        if strings.EqualFold(attr.Key, "lang") {
            pd.Language = strings.TrimSpace(attr.Val)
            return
        }
    }
    // Default if no lang attribute
    pd.Language = "unspecified"
}

func handleTitle(node *html.Node, pd *types.PageData) error {
    pd.Title = extractNodeText(node)
    if pd.Title == "" {
        return nil
    }
    if err := checkTitleFilter(pd.Title); err != nil {
        return fmt.Errorf("title contians filtered terms: %w", err)
    }
    return nil
}


func parseMetaTags(node *html.Node, pd *types.PageData) {
	var (
		name, content, charset, property string
		httpEquiv                        string
	)

	for _, attr := range node.Attr {
		switch strings.ToLower(attr.Key) {
		case "name":
			name = attr.Val
		case "content":
			content = attr.Val
		case "charset":
			charset = attr.Val
		case "property":
			property = attr.Val
		case "http-equiv":
			httpEquiv = strings.ToLower(attr.Val)
		}
	}

	switch {
	case charset != "":
		pd.Charset = charset
	case httpEquiv == "content-type":
		if parts := strings.SplitN(content, "charset=", 2); len(parts) > 1 {
			pd.Charset = strings.TrimSpace(parts[1])
		}
	case strings.HasPrefix(property, "og:"):
		if pd.OpenGraph == nil {
			pd.OpenGraph = make(map[string]string)
		}
		pd.OpenGraph[property] = content
	case name == "description":
		pd.MetaDescription = content
	case name == "robots":
		pd.RobotsMeta = content
	}

	parseTimestamps(property, content, pd)
}

func parseTimestamps(property, content string, pd *types.PageData) {
	if content == "" {
		return
	}

	var t time.Time
	var err error
	
	switch property {
	case "article:published_time", "datepublished":
		t, err = time.Parse(time.RFC3339, content)
		if err == nil {
			pd.DatePublished = t
		}
	case "article:modified_time", "datemodified":
		t, err = time.Parse(time.RFC3339, content)
		if err == nil {
			pd.DateModified = t
		}
	}
}

func processAnchor(node *html.Node, pd *types.PageData, base *url.URL,
	internalLinks *[]string, externalLinks *[]string) {

	href := getAttribute(node, "href")
	if href == "" {
		return
	}

	parsed, err := url.Parse(href)
	if err != nil {
		return
	}

	resolved := base.ResolveReference(parsed)
	if !isValidScheme(resolved) {
		return
	}

	anchorText := extractNodeText(node)
	if anchorText != "" {
		pd.AnchorTexts = append(pd.AnchorTexts, anchorText)
	}

	if resolved.Host == base.Host {
		*internalLinks = append(*internalLinks, resolved.String())
	} else {
		*externalLinks = append(*externalLinks, resolved.String())
	}
}

func isValidScheme(u *url.URL) bool {
	return u.Scheme == "http" || u.Scheme == "https"
}

func filterSocialLinks(links []string) []string {
	social := make([]string, 0, 5)
	for _, link := range links {
		u, err := url.Parse(link)
		if err != nil {
			continue
		}
		if _, exists := socialDomains[u.Hostname()]; exists {
			social = append(social, link)
		}
	}
	return social
}

func normalizeText(text string) string {
	scanner := bufio.NewScanner(strings.NewReader(text))
	var builder strings.Builder

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			builder.WriteString(line)
			builder.WriteByte(' ')
		}
	}

	return strings.Join(strings.Fields(builder.String()), " ")
}

// extractNodeText collects text from all text nodes in the subtree (iterative version)
func extractNodeText(node *html.Node) string {
	var builder strings.Builder
	stack := make([]*html.Node, 0, 32)
	stack = append(stack, node)
	
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		
		if current.Type == html.TextNode {
			builder.WriteString(current.Data)
		}
		
		// Add children in reverse order to maintain document order
		for child := current.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return strings.TrimSpace(builder.String())
}

// Verifies title against blocked terms (hot-path optimized)
func checkTitleFilter(title string) error {
	if len(filterTerms) == 0 {
		return nil
	}

	lowerTitle := strings.ToLower(title)
	for _, term := range filterTerms {
		if strings.Contains(lowerTitle, term) {
			return fmt.Errorf("title contains filtered term: %q", term)
		}
	}
	return nil
}

// Extracts alt attributes from img elements
func parseImage(node *html.Node, pd *types.PageData) {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "alt") && attr.Val != "" {
			pd.AltTexts = append(pd.AltTexts, attr.Val)
		}
	}
}

// Captures heading text by level
func storeHeading(node *html.Node, pd *types.PageData) {
	if pd.Headings == nil {
		pd.Headings = make(map[string][]string, 6)
	}
	
	text := extractNodeText(node)
	tag := strings.ToLower(node.Data)
	pd.Headings[tag] = append(pd.Headings[tag], text)
}

// parseLink handles canonical link discovery
func parseLink(node *html.Node, pd *types.PageData, base *url.URL) {
	var href, rel string
	for _, attr := range node.Attr {
		switch strings.ToLower(attr.Key) {
		case "href":
			href = attr.Val
		case "rel":
			rel = strings.ToLower(attr.Val)
		}
	}

	if href == "" || !strings.Contains(rel, "canonical") {
		return
	}

	if parsed, err := url.Parse(href); err == nil {
		pd.CanonicalURL = base.ResolveReference(parsed).String()
	}
}

// parseScript extracts JSON-LD content
func parseScript(node *html.Node, pd *types.PageData) {
	var scriptType string
	var content strings.Builder

	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "type") {
			scriptType = strings.ToLower(attr.Val)
			break
		}
	}

	if scriptType != "application/ld+json" {
		return
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			content.WriteString(child.Data)
		}
	}

	if content.Len() > 0 {
		pd.StructuredData = append(pd.StructuredData, content.String())
	}
}

// Locates the first valid base href (HTML5-compliant)
func findBaseTag(doc *html.Node) *url.URL {
	stack := make([]*html.Node, 0, 256)
	stack = append(stack, doc)
	
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if current.Type == html.ElementNode && current.Data == "base" {
			for _, attr := range current.Attr {
				if strings.EqualFold(attr.Key, "href") {
					if u, err := url.Parse(attr.Val); err == nil {
						return u
					}
				}
			}
			return nil // First base tag wins per spec
		}

		// Prioritize head section search
		if current.Type == html.ElementNode && current.Data == "head" {
			for child := current.FirstChild; child != nil; child = child.NextSibling {
				stack = append(stack, child)
			}
			continue
		}

		// Depth-first search
		for child := current.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return nil
}

