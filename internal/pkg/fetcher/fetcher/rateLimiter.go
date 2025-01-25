package fetcher

import (
	"errors"
	"net/http"
	"net/url"
	"sync"
    "context"
	"time"
	"github.com/temoto/robotstxt"
)

type RobotsData struct {
    group         *robotstxt.Group
    crawlDelay    time.Duration
    lastAccess    time.Time
    robotsFetched time.Time
    mu            sync.Mutex
}

var (
    robotsCache             = make(map[string]*RobotsData)
    robotsCacheMutex        sync.Mutex
    ErrCrawlingDisallowed   = errors.New("crawling disallowed by robots.txt")
    sleepFunc               = time.Sleep
)

// Checks if crawling is permitted for the given URL
// and enforces the Crawl-delay specified in robots.txt.
func waitForPermission(context context.Context, targetURL string) error {
    parsedURL, err := url.Parse(targetURL)
    if err != nil {
        return err
    }
    domain := parsedURL.Hostname()

    // Retrieve or initialize RobotsData
    robotsCacheMutex.Lock()
    rData, exists := robotsCache[domain]
    if !exists {
        rData = &RobotsData{}
        robotsCache[domain] = rData
    }
    rData.crawlDelay = min(rData.crawlDelay, 5 * time.Second) // Cap crawl delay at 5 seconds
    robotsCacheMutex.Unlock()

    rData.mu.Lock()
    defer rData.mu.Unlock()

    // Refresh robots.txt if needed
    if time.Since(rData.robotsFetched) > 24 * time.Hour || rData.group == nil {
        err := fetchRobotsData(context, parsedURL, rData)
        if err != nil {
            return err
        }
    }

    // Check if crawling is permitted
    if rData.group != nil && !rData.group.Test(parsedURL.Path) {
        return ErrCrawlingDisallowed
    }

    // Enforce crawl delay
    now := time.Now()
    elapsed := now.Sub(rData.lastAccess)
    waitTime := rData.crawlDelay - elapsed
    if waitTime > 0 {
        if waitTime > rData.crawlDelay {
            // In case of clock adjustments or anomalies
            waitTime = rData.crawlDelay
        }
        sleepFunc(waitTime)
        rData.lastAccess = rData.lastAccess.Add(rData.crawlDelay)
    } else {
        rData.lastAccess = now
    }

    return nil
}

// Fetches and parses the robots.txt file for the domain.
// It updates the RobotsData with the parsed information.
func fetchRobotsData(context context.Context, parsedURL *url.URL, rData *RobotsData) error {
    robotsURL := parsedURL.Scheme + "://" + parsedURL.Host + "/robots.txt"

    req, err := http.NewRequestWithContext(context, "GET", robotsURL, nil)
    if err != nil {
        return err
    }
    req.Header.Set("User-Agent", getRandomUserAgent())

    resp, err := httpClient.Do(req)
    if err != nil {
        // Assume allow all (but set group to nil to indicate that no robots.txt was found)
        rData.group = nil
        rData.crawlDelay = 0
        rData.robotsFetched = time.Now()
        return nil
    }
    defer resp.Body.Close()

    robots, err := robotstxt.FromResponse(resp)
    if err != nil {
        // Assume allow all
        rData.group = nil
        rData.crawlDelay = 0
        rData.robotsFetched = time.Now()
        return nil
    }

    group := robots.FindGroup(getRandomUserAgent())
    if group == nil {
        group = robots.FindGroup("*")
    }
    var crawlDelay time.Duration
    if group != nil && group.CrawlDelay >= 0 {
        crawlDelay = time.Duration(group.CrawlDelay * time.Second)
    }
    rData.group = group
    rData.crawlDelay = crawlDelay
    rData.robotsFetched = time.Now()

    return nil
}
