package fetcher

import (
    "context"
    "strings"
    "log"
    "time"
    "github.com/chromedp/chromedp"
)

func Fetch(shortUrl string) string {

    // Construct the full URL
    var urlBuilder strings.Builder
    urlBuilder.WriteString("https://www.")
    urlBuilder.WriteString(shortUrl)
    urlBuilder.WriteString("/")
    url := urlBuilder.String()

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
    err := chromedp.Run(ctx,
        chromedp.Navigate(url),
        chromedp.WaitReady("body"),
        chromedp.Text("body", &content, chromedp.ByQuery), // Controls what to extract from the page
    )
    if err != nil {
        // Togglable fatal error for debugging, this will be refactored later
        log.Printf("Failed to fetch HTML content from URL: %v | err: %v ", url, err)
        // log.Fatalf("Failed to fetch HTML content from URL: %v | err: %v ", url, err)
    }

    return content
}
