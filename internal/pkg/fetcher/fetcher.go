package fetcher

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/temoto/robotstxt"
)

// Fetch retrieves the text content from the body of a web page.
func Fetch(shortUrl string) string {
	url := buildFullUrl(shortUrl)

	// Check robots.txt before crawling
	permitted, err := isCrawlingPermitted(url)
	if err != nil {
		log.Printf("Error checking robots.txt for URL %v: %v", url, err)
	}
	if !permitted {
		log.Printf("Crawling disallowed by robots.txt for URL: %v", url)
		return ""
	}

	// Set up Chrome options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.Headless,
		chromedp.NoSandbox,
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
	)

	// Set up allocator and context without a timeout
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancel()

	// Run a dummy task to initialize Chrome without a timeout
	if err := chromedp.Run(ctx); err != nil {
		log.Fatalf("Failed to initialize Chrome: %v", err)
	}

	// Now add a timeout to the context
	ctx, timeoutCancel := context.WithTimeout(ctx, 30*time.Second)
	defer timeoutCancel()

	// Fetch the HTML content and return it
	var content string
	err = chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
		chromedp.Text("body", &content, chromedp.ByQuery), // Controls what to extract from the page
	)
	if err != nil {
		log.Printf("Failed to fetch HTML content from URL: %v | err: %v ", url, err)
	}

	return content
}

// buildFullUrl constructs the full URL from a short URL.
func buildFullUrl(shortUrl string) string {
	var urlBuilder strings.Builder
	urlBuilder.WriteString("https://www.")
	urlBuilder.WriteString(shortUrl)
	urlBuilder.WriteString("/")
	return urlBuilder.String()
}

// isCrawlingPermitted checks the domain's robots.txt file to see if the URL is allowed to be crawled.
func isCrawlingPermitted(targetURL string) (bool, error) {
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return false, err
	}

	robotsURL := parsedURL.Scheme + "://" + parsedURL.Host + "/robots.txt"
	response, err := http.Get(robotsURL)
	if err != nil {
		// If robots.txt cannot be fetched, treat as no robots.txt
		return true, nil
	}
	defer response.Body.Close()

	robotsData, err := robotstxt.FromResponse(response)
	if err != nil {
		// If robots.txt cannot be parsed, treat as no robots.txt
		return true, nil
	}

	group := robotsData.FindGroup("Crawlerbot")
	return group.Test(parsedURL.Path), nil
}
