package administrator

func (a *Administrator) incrementLineNumber() {
    a.progressMutex.Lock()
    a.lineNumber++
    currentLineNumber := a.lineNumber
    a.progressMutex.Unlock()
    if (currentLineNumber % 100) == 0 {
        a.saveProgress()
    }
}

func (a *Administrator) incrementDomainVisitCount(domain string) {
    a.domainMutex.Lock()
    a.domainVisits[domain]++
    a.domainMutex.Unlock()
}

func (a *Administrator) getDomainVisitCount(domain string) int {
    a.domainMutex.Lock()
    defer a.domainMutex.Unlock()
    return a.domainVisits[domain]
}

