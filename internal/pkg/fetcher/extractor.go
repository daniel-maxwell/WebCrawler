package fetcher

import (
    "bytes"
    "net/url"
    "strings"
    "time"
    "golang.org/x/net/html"
)

// Extracts the title of the HTML document.
func extractTitle(doc *html.Node) string {
    var title string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "title" {
            if n.FirstChild != nil {
                title = n.FirstChild.Data
            }
            return
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return title
}

// Extracts the meta tags of the HTML document.
func extractMetaTags(doc *html.Node) (metaDescription, metaKeywords, robotsMeta, charset string) {
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "meta" {
            var name, content, httpEquiv, metaCharset string
            for _, attr := range n.Attr {
                switch strings.ToLower(attr.Key) {
                case "name":
                    name = strings.ToLower(attr.Val)
                case "content":
                    content = attr.Val
                case "http-equiv":
                    httpEquiv = strings.ToLower(attr.Val)
                case "charset":
                    metaCharset = attr.Val
                }
            }
            if name == "description" {
                metaDescription = content
            } else if name == "keywords" {
                metaKeywords = content
            } else if name == "robots" {
                robotsMeta = content
            } else if metaCharset != "" {
                charset = metaCharset
            } else if httpEquiv == "content-type" {
                // Parse content-type to get charset
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return
}

// Extracts the canonical URL of the HTML document.
func extractCanonicalURL(doc *html.Node) string {
    var canonicalURL string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "link" {
            var rel, href string
            for _, attr := range n.Attr {
                switch strings.ToLower(attr.Key) {
                case "rel":
                    rel = strings.ToLower(attr.Val)
                case "href":
                    href = attr.Val
                }
            }
            if rel == "canonical" {
                canonicalURL = href
                return
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return canonicalURL
}

// Extracts the headings of the HTML document.
func extractHeadings(doc *html.Node) map[string][]string {
    headings := make(map[string][]string)
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode {
            switch n.Data {
            case "h1", "h2", "h3", "h4", "h5", "h6":
                var headingText string
                if n.FirstChild != nil {
                    headingText = ExtractNodeText(n)
                }
                headings[n.Data] = append(headings[n.Data], headingText)
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return headings
}

// Extracts the alt texts of the images in the HTML document.
func extractAltTexts(doc *html.Node) []string {
    var altTexts []string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "img" {
            for _, attr := range n.Attr {
                if strings.ToLower(attr.Key) == "alt" && attr.Val != "" {
                    altTexts = append(altTexts, attr.Val)
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return altTexts
}

// Extracts the anchor texts and links in the HTML document.
func extractAnchorTextsAndLinks(doc *html.Node, baseURL string) (anchorTexts []string, internalLinks []string, externalLinks []string) {
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "a" {
            var href, anchorText string
            for _, attr := range n.Attr {
                if strings.ToLower(attr.Key) == "href" {
                    href = attr.Val
                }
            }
            if n.FirstChild != nil {
                anchorText = ExtractNodeText(n)
            }
            if href != "" {
                anchorTexts = append(anchorTexts, anchorText)
                // Resolve href relative to baseURL
                link, err := url.Parse(href)
                if err == nil {
                    base, err := url.Parse(baseURL)
                    if err == nil {
                        resolvedURL := base.ResolveReference(link)
                        if resolvedURL.Host == base.Host {
                            internalLinks = append(internalLinks, resolvedURL.String())
                        } else {
                            externalLinks = append(externalLinks, resolvedURL.String())
                        }
                    }
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return
}

// Extracts the Open Graph data of the HTML document.
func extractOpenGraphData(doc *html.Node) map[string]string {
    openGraph := make(map[string]string)
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "meta" {
            var property, content string
            for _, attr := range n.Attr {
                switch strings.ToLower(attr.Key) {
                case "property":
                    property = attr.Val
                case "content":
                    content = attr.Val
                }
            }
            if strings.HasPrefix(property, "og:") {
                openGraph[property] = content
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return openGraph
}

// Extracts the author of the HTML document.
func extractAuthor(doc *html.Node) string {
    var author string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "meta" {
            var name, content string
            for _, attr := range n.Attr {
                switch strings.ToLower(attr.Key) {
                case "name":
                    name = strings.ToLower(attr.Val)
                case "content":
                    content = attr.Val
                }
            }
            if name == "author" {
                author = content
                return
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return author
}

// Extracts the published and modified dates of the HTML document.
func extractDates(doc *html.Node) (datePublished, dateModified time.Time) {
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "meta" {
            var property, content string
            for _, attr := range n.Attr {
                switch strings.ToLower(attr.Key) {
                case "property":
                    property = attr.Val
                case "content":
                    content = attr.Val
                }
            }
            if property == "article:published_time" {
                t, err := time.Parse(time.RFC3339, content)
                if err == nil {
                    datePublished = t
                }
            } else if property == "article:modified_time" {
                t, err := time.Parse(time.RFC3339, content)
                if err == nil {
                    dateModified = t
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return
}

// Extracts the structured data of the HTML document.
func extractStructuredData(doc *html.Node) []string {
    var structuredData []string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode {
            if n.Data == "script" {
                var scriptType string
                for _, attr := range n.Attr {
                    if strings.ToLower(attr.Key) == "type" {
                        scriptType = attr.Val
                    }
                }
                if scriptType == "application/ld+json" && n.FirstChild != nil {
                    structuredData = append(structuredData, n.FirstChild.Data)
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return structuredData
}

// Extracts the language of the HTML document.
func extractLanguage(doc *html.Node) string {
    var lang string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "html" {
            for _, attr := range n.Attr {
                if strings.ToLower(attr.Key) == "lang" {
                    lang = attr.Val
                    return
                }
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return lang
}

// Extracts the social links from the external links.
func extractSocialLinks(externalLinks []string) []string {
    var socialLinks []string
    for _, link := range externalLinks {
        if strings.Contains(link, "facebook.com") ||
            strings.Contains(link, "twitter.com") ||
			strings.Contains(link, "x.com") ||
            strings.Contains(link, "instagram.com") ||
            strings.Contains(link, "linkedin.com") {
            socialLinks = append(socialLinks, link)
        }
    }
    return socialLinks
}

// Extracts the visible text from an HTML node.
func ExtractNodeText(n *html.Node) string {
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