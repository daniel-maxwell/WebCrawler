package fetcher

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

type RobotsData struct {
    group         *robotstxt.Group
    crawlDelay    time.Duration
    lastAccess    time.Time
    robotsFetched time.Time
    mutex         sync.Mutex
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
    robotsData, exists := robotsCache[domain]
    if !exists {
        robotsData = &RobotsData{}
        robotsCache[domain] = robotsData
    }

    robotsData.crawlDelay = min(robotsData.crawlDelay, 5 * time.Second) // Cap crawl delay at 5 seconds
    robotsCacheMutex.Unlock()

    robotsData.mutex.Lock()
    defer robotsData.mutex.Unlock()

    // Refresh robots.txt if needed
    if time.Since(robotsData.robotsFetched) > 24 * time.Hour || robotsData.group == nil {
        err := fetchRobotsData(context, parsedURL, robotsData)
        if err != nil {
            return err
        }
    }

    // Check if crawling is permitted
    if robotsData.group != nil && !robotsData.group.Test(parsedURL.Path) {
        return ErrCrawlingDisallowed
    }

    // Enforce crawl delay
    now := time.Now()
    elapsed := now.Sub(robotsData.lastAccess)
    waitTime := robotsData.crawlDelay - elapsed
    if waitTime > 0 {
        if waitTime > robotsData.crawlDelay {
            // In case of clock adjustments or anomalies
            waitTime = robotsData.crawlDelay
        }
        sleepFunc(waitTime)
        robotsData.lastAccess = robotsData.lastAccess.Add(robotsData.crawlDelay)
    } else {
        robotsData.lastAccess = now
    }

    return nil
}

// Fetches and parses the robots.txt file for the domain.
// It updates the RobotsData with the parsed information.
func fetchRobotsData(context context.Context, parsedURL *url.URL, robotsData *RobotsData) error {
    robotsURL := parsedURL.Scheme + "://" + parsedURL.Host + "/robots.txt"

    req, err := http.NewRequestWithContext(context, "GET", robotsURL, nil)
    if err != nil {
        return err
    }
    req.Header.Set("User-Agent", getRandomUserAgent())

    resp, err := httpClient.Do(req)
    if err != nil {
        // Assume allow all (but set group to nil to indicate that no robots.txt was found)
        robotsData.group = nil
        robotsData.crawlDelay = 0
        robotsData.robotsFetched = time.Now()
        return nil
    }
    defer resp.Body.Close()

    robots, err := robotstxt.FromResponse(resp)
    if err != nil {
        // Assume allow all
        robotsData.group = nil
        robotsData.crawlDelay = 0
        robotsData.robotsFetched = time.Now()
        return nil
    }

    group := robots.FindGroup(getRandomUserAgent())
    if group == nil {
        group = robots.FindGroup("*")
    }
    var crawlDelay time.Duration
    if group != nil && group.CrawlDelay >= 0 {
        crawlDelay = group.CrawlDelay
    }
    robotsData.group = group
    robotsData.crawlDelay = crawlDelay
    robotsData.robotsFetched = time.Now()

    return nil
}
