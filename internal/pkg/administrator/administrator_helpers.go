package administrator

import (
	"math"
	"time"
)

// Increments websites file line number
func (a *Administrator) incrementLineNumber() {
    a.progressMutex.Lock()
    a.lineNumber++
    currentLineNumber := a.lineNumber
    a.progressMutex.Unlock()
    if (currentLineNumber % 100) == 0 {
        a.saveProgress()
    }
}

// Increments the counter for the number of times a domain has been visited
func (a *Administrator) incrementDomainVisitCount(domain string) {
    a.domainMutex.Lock()
    a.domainVisits[domain]++
    a.domainMutex.Unlock()
}

// Gets the number of times a domain has been visited
func (a *Administrator) getDomainVisitCount(domain string) int {
    a.domainMutex.Lock()
    defer a.domainMutex.Unlock()
    return a.domainVisits[domain]
}

// Gets a decimal representation of how full the queue is from 0 to 1
func (a *Administrator) getQueueUsage() float64 {
    return float64(a.urlQueue.Length()) / float64(queueCapacity)
}

// Puts the administrator to sleep for a duration based on queue utilization
func (a *Administrator) sleepBasedOnQueueSize() {
    usage := a.getQueueUsage()
    sleepMs := math.Min(
        float64(maxSleepMs),
        math.Max(usage * float64(maxSleepMs), 0),
    )
    if sleepMs > 0 {
        time.Sleep(time.Duration(sleepMs) * time.Millisecond)
    }
}
