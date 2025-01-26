package fetcher

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"
	"golang.org/x/net/html"
	"webcrawler/internal/pkg/types"
)

// Parses HTML content and extracts all data in one traversal
func traverseAndExtractPageContent(content, baseURL string) (types.PageData, error) {
	var pageData types.PageData
	pageData.URL = baseURL

	doc, parseError := parseHTMLWithTimeout(content, maxParseTime)
	if parseError != nil {
		fmt.Printf("Failed to parse HTML content for URL: [%v] | Reason: [%v]\n", baseURL, parseError)
		return pageData, parseError
	}

	baseParsed, err := url.Parse(baseURL)
    if err != nil {
        fmt.Printf("Failed to parse baseURL: [%v] | Reason: [%v]\n", baseURL, err)
        return pageData, err
    }

	var visibleTextBuilder strings.Builder

	traverseNodes(doc, &pageData, baseParsed, &visibleTextBuilder)

	pageData.VisibleText = visibleTextBuilder.String()

	pageData.IsSecure = (baseParsed.Scheme == "https")
	pageData.SocialLinks = extractSocialLinks(pageData.ExternalLinks)

	return pageData, nil
}

// Walks the DOM and calls the appropriate extraction handlers
func traverseNodes(currentNode *html.Node, 
				   pageData *types.PageData, 
				   baseParsed *url.URL, 
				   visibleTextBuilder *strings.Builder) {
    if currentNode.Type == html.TextNode {
        handleTextNode(currentNode, visibleTextBuilder)
    } else if currentNode.Type == html.ElementNode {
        handleElementNode(currentNode, pageData, baseParsed)
    }

    for child := currentNode.FirstChild; child != nil; child = child.NextSibling {
        traverseNodes(child, pageData, baseParsed, visibleTextBuilder)
    }
}

// Extracts visible text (ignoring <script>, <noscript> and <style>)
func handleTextNode(currentNode *html.Node, visibleTextBuilder *strings.Builder) {
    if currentNode.Parent != nil {
        parentTag := strings.ToLower(currentNode.Parent.Data)
        if parentTag == "script" || parentTag == "noscript" || parentTag == "style" {
            return
        }
    }
    visibleTextBuilder.WriteString(currentNode.Data)
}

// Directs each element type to its specialized extractor
func handleElementNode(currentNode *html.Node, pageData *types.PageData, baseParsed *url.URL, ) {
    switch strings.ToLower(currentNode.Data) {
    case "title":
        handleTitle(currentNode, pageData)
    case "meta":
        handleMeta(currentNode, pageData)
    case "link":
        handleLink(currentNode, pageData)
    case "img":
        handleImage(currentNode, pageData)
    case "script":
        handleScript(currentNode, pageData)
    case "html":
        handleHtmlTag(currentNode, pageData)
    case "a":
        handleAnchor(currentNode, pageData, baseParsed)
    case "h1", "h2", "h3", "h4", "h5", "h6":
        handleHeading(currentNode, pageData)
    }
}

// Extracts the <title> text
func handleTitle(currentNode *html.Node, pageData *types.PageData) {
	if currentNode.FirstChild != nil {
		pageData.Title = currentNode.FirstChild.Data
	}
}

// Extracts meta tags, Open Graph, charset, robots, author, etc.
func handleMeta(currentNode *html.Node, pageData *types.PageData) {
	var metaName, metaContent, httpEquiv, metaCharset, property string

	for _, attribute := range currentNode.Attr {
		switch strings.ToLower(attribute.Key) {
		case "name":
			metaName = strings.ToLower(attribute.Val)
		case "content":
			metaContent = attribute.Val
		case "http-equiv":
			httpEquiv = strings.ToLower(attribute.Val)
		case "charset":
			metaCharset = attribute.Val
		case "property":
			property = attribute.Val
		}
	}

	// Handle standard meta attributes
	switch metaName {
	case "description":
		pageData.MetaDescription = metaContent
	case "keywords":
		pageData.MetaKeywords = metaContent
	case "robots":
		pageData.RobotsMeta = metaContent
	case "author":
		pageData.Author = metaContent
	}

	// Handle charset if provided directly
	if metaCharset != "" {
		pageData.Charset = metaCharset
	} else if httpEquiv == "content-type" {
		// TODO: parse "metaContent" to extract charset from "text/html; charset=UTF-8" etc.
	}

	// Handle Open Graph (property="og:...")
	if strings.HasPrefix(property, "og:") {
		if pageData.OpenGraph == nil {
			pageData.OpenGraph = make(map[string]string)
		}
		pageData.OpenGraph[property] = metaContent
	}

	// Handle published/modified times (Open Graph article tags)
	if property == "article:published_time" {
		if parsedTime, timeErr := time.Parse(time.RFC3339, metaContent); timeErr == nil {
			pageData.DatePublished = parsedTime
		}
	} else if property == "article:modified_time" {
		if parsedTime, timeErr := time.Parse(time.RFC3339, metaContent); timeErr == nil {
			pageData.DateModified = parsedTime
		}
	}
}

// Extracts the canonical link if present
func handleLink(currentNode *html.Node, pageData *types.PageData) {
	var relValue, hrefValue string
	for _, attribute := range currentNode.Attr {
		switch strings.ToLower(attribute.Key) {
		case "rel":
			relValue = strings.ToLower(attribute.Val)
		case "href":
			hrefValue = attribute.Val
		}
	}
	if relValue == "canonical" {
		pageData.CanonicalURL = hrefValue
	}
}

// Extracts alt text from <img> tags
func handleImage(currentNode *html.Node, pageData *types.PageData) {
	for _, attribute := range currentNode.Attr {
		if strings.ToLower(attribute.Key) == "alt" && attribute.Val != "" {
			pageData.AltTexts = append(pageData.AltTexts, attribute.Val)
		}
	}
}

// Extracts JSON-LD (structured data) from <script type="application/ld+json">
func handleScript(currentNode *html.Node, pageData *types.PageData) {
	var scriptType string
	for _, attribute := range currentNode.Attr {
		if strings.ToLower(attribute.Key) == "type" {
			scriptType = attribute.Val
		}
	}
	if scriptType == "application/ld+json" && currentNode.FirstChild != nil {
		pageData.StructuredData = append(pageData.StructuredData, currentNode.FirstChild.Data)
	}
}

// Extracts the lang attribute from <html>
func handleHtmlTag(currentNode *html.Node, pageData *types.PageData) {
	for _, attribute := range currentNode.Attr {
		if strings.ToLower(attribute.Key) == "lang" {
			pageData.Language = attribute.Val
		}
	}
}

// Processes anchor text and determines if links are internal or external
func handleAnchor(currentNode *html.Node, pageData *types.PageData, baseParsed *url.URL) {
    // Capture anchor text
    if currentNode.FirstChild != nil {
        anchorText := extractNodeText(currentNode)
        if anchorText != "" {
            pageData.AnchorTexts = append(pageData.AnchorTexts, anchorText)
        }
    }

    // Resolve the href
    var hrefValue string
    for _, attribute := range currentNode.Attr {
        if strings.ToLower(attribute.Key) == "href" {
            hrefValue = attribute.Val
            break
        }
    }

    if hrefValue != "" {
        linkURL, parseError := url.Parse(hrefValue)
        if parseError == nil {
            // We already have baseParsed from TraverseAndExtractPageContent
            resolvedURL := baseParsed.ResolveReference(linkURL)
            if resolvedURL.Host == baseParsed.Host {
                pageData.InternalLinks = append(pageData.InternalLinks, resolvedURL.String())
            } else {
                pageData.ExternalLinks = append(pageData.ExternalLinks, resolvedURL.String())
            }
        }
    }
}

// Grabs text from <h1>.. <h6> tags and stores them by heading level
func handleHeading(currentNode *html.Node, pageData *types.PageData) {
	headingText := extractNodeText(currentNode)
	headingTag := strings.ToLower(currentNode.Data)

	if pageData.Headings == nil {
		pageData.Headings = make(map[string][]string)
	}
	pageData.Headings[headingTag] = append(pageData.Headings[headingTag], headingText)
}

// Small DFS that accumulates text from all descendant text nodes
func extractNodeText(currentNode *html.Node) string {
	var buffer bytes.Buffer
	var gatherText func(*html.Node)
	gatherText = func(node *html.Node) {
		if node.Type == html.TextNode {
			buffer.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			gatherText(child)
		}
	}
	gatherText(currentNode)
	return buffer.String()
}

// Filters externalLinks for known social domains
func extractSocialLinks(externalLinks []string) []string {
	var socialLinks []string
	for _, link := range externalLinks {
		lowerLink := strings.ToLower(link)
		if 	strings.Contains(lowerLink, "facebook.com")  ||
			strings.Contains(lowerLink, "twitter.com") 	 ||
			strings.Contains(lowerLink, "x.com") 		 ||
			strings.Contains(lowerLink, "instagram.com") ||
			strings.Contains(lowerLink, "linkedin.com") {
			socialLinks = append(socialLinks, link)
		}
	}
	return socialLinks
}
