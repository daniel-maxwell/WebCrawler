package administrator

import (
	"math"
    "strings"
	"time"
    "webcrawler/internal/pkg/utils"
)

// Increments websites file line number
func (admin *Administrator) incrementLineNumber() {
    admin.progressMutex.Lock()
    admin.lineNumber++
    currentLineNumber := admin.lineNumber
    admin.progressMutex.Unlock()
    if (currentLineNumber % 100) == 0 {
        admin.saveProgress()
    }
}

// Increments the counter for the number of times a domain has been visited
func (admin *Administrator) incrementDomainVisitCount(domain string) {
    admin.domainMutex.Lock()
    admin.domainVisits[domain]++
    admin.domainMutex.Unlock()
}

// Gets the number of times a domain has been visited
func (admin *Administrator) getDomainVisitCount(domain string) int {
    admin.domainMutex.Lock()
    defer admin.domainMutex.Unlock()
    return admin.domainVisits[domain]
}

// Gets a decimal representation of how full the queue is from 0 to 1
func (admin *Administrator) getQueueUsage() float64 {
    return float64(admin.urlQueue.Length()) / float64(queueCapacity)
}

// Puts the administrator to sleep for a duration based on queue utilization
func (admin *Administrator) sleepBasedOnQueueSize() {
    usage := admin.getQueueUsage()
    sleepMs := math.Min(
        float64(maxSleepMs),
        math.Max(usage * float64(maxSleepMs), 0),
    )
    if sleepMs > 0 {
        time.Sleep(time.Duration(sleepMs) * time.Millisecond)
    }
}

// Enqueues URLs extracted from a page, interleaving internal and external URLs
// until a limit is reached or no more URLs are available
func (admin *Administrator) enqueueExtractedURLs(sourceURL    string, 
                                                 internalURLs []string,
                                                 externalURLs []string) {
    // This code tries to enqueue links found on the page in a way that
    // avoids too many links from the same domain being contiguous in the queue.
    totalLinksEnqueued, internalIdx, externalIdx := 0, 0, 0

    currentDomain, domainParseErr := utils.GetDomainFromURL(sourceURL)
    
    // Enqueue based on queue usage within bounds of 2 to 20
    enqueueLimit := min(20, max(2, 100 - (int(admin.getQueueUsage()) * 100)))
    visitLimit := domainLimit
    
    // Double the limit for .org, .edu or .ac.uk domains
    if domainParseErr == nil && (strings.HasSuffix(currentDomain, ".org") || 
                                 strings.HasSuffix(currentDomain, ".edu") || 
                                 strings.HasSuffix(currentDomain, ".ac.uk")) {                   
        enqueueLimit = enqueueLimit * 2 
        visitLimit   = domainLimit  * 2
    }

    doEnqueueInternalURLs := domainParseErr == nil

    // Enqueue URLs in a round-robin fashion until the limit is reached
    for totalLinksEnqueued < enqueueLimit {

        if internalIdx >= len(internalURLs) && externalIdx >= len(externalURLs) {
            break
        }

        if doEnqueueInternalURLs && admin.getDomainVisitCount(currentDomain) < visitLimit {

            for internalIdx < len(internalURLs) && admin.bloomFilter.IsVisited(internalURLs[internalIdx]) {
                internalIdx++
            }

            if internalIdx < len(internalURLs) {
                err := admin.urlQueue.Insert(internalURLs[internalIdx])
                if err == nil { // Success, increment domain visit count and break out of the loop.
                    admin.incrementDomainVisitCount(currentDomain)
                    totalLinksEnqueued++
                }
                internalIdx++
            }

        } else {
            internalIdx = len(internalURLs)
        }

        for externalIdx < len(externalURLs) && admin.bloomFilter.IsVisited(externalURLs[externalIdx]) {
            externalIdx++
        }

        if externalIdx < len(externalURLs) {
            domain, err := utils.GetDomainFromURL(externalURLs[externalIdx])
            if err == nil && admin.getDomainVisitCount(domain) < visitLimit {
                err := admin.urlQueue.Insert(externalURLs[externalIdx])
                if err == nil {
                    admin.incrementDomainVisitCount(domain)
                    totalLinksEnqueued++
                }
            }
            externalIdx++
        }
    }
}
