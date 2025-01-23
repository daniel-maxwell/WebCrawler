package administrator

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	//"encoding/json"
	"webcrawler/internal/pkg/fetcher"
	bloomfilter "webcrawler/internal/pkg/filter"
	"webcrawler/internal/pkg/queue"
	"webcrawler/internal/pkg/utils"
)

// TODO: Investigate the following constants to optimize for production
const (
	numReaderWorkers  = 3
	numFetcherWorkers = 10
	queueCapacity     = 100000
	domainLimit       = 100
	maxSleepMs        = 5000
)

type Administrator struct {
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	urlChan       chan string
	lineNumber    int
	progressFile  string
	progressMutex sync.Mutex
	bloomFilter   *bloomfilter.BloomFilterManager
	urlQueue      *queue.Queue
	domainVisits  map[string]int
	domainMutex   sync.Mutex
	fetcherMap    map[int]LogEntry
	fetcherMapMutex sync.Mutex
}

type LogEntry struct {
    Time    string
    Message string
}

func NewAdministrator(progressFilePath string) *Administrator {
	ctx, cancel := context.WithCancel(context.Background())

	q, err := queue.CreateQueue(queueCapacity)
	if err != nil {
		panic(fmt.Sprintf("Failed to create queue: %v", err))
	}

	err = fetcher.Init()
	if err != nil {
		panic(err)
	}

	// Create a new bloom filter manager - capacity will need to be increased for prod
	filter, err := bloomfilter.NewBloomFilterManager("internal/pkg/filter/data/bloomfilter.dat", 1000, 1000000, 0.01)
	if err != nil {
		panic(fmt.Sprintf("Failed to create bloom filter: %v", err))
	}

	return &Administrator{
		ctx:          ctx,
		cancel:       cancel,
		urlChan:      make(chan string, 50), // TODO: Investigate buffer size requirements
		progressFile: progressFilePath,
		bloomFilter:  filter,
		urlQueue:     q,
		domainVisits: make(map[string]int),
		fetcherMap:   make(map[int]LogEntry),
	}
}

func (a *Administrator) Run() {
	fmt.Println("Administrator Run Called")

	// Start Reader Workers
	for i := 0; i < numReaderWorkers; i++ {
		a.wg.Add(1)
		go a.readerWorker(i)
	}

	// Start Fetcher Workers
	for i := 0; i < numFetcherWorkers; i++ {
		a.wg.Add(1)
		go a.fetcherWorker(i)
	}

	// Continuous loop
	for {
		select {
		case <-a.ctx.Done():
			close(a.urlChan) // No more URLs read from file
			a.wg.Wait()
			return
		default:
			a.lineNumber = a.loadProgress()

			file, err := os.Open("internal/pkg/administrator/data/top-1m.txt")
			if err != nil {
				log.Fatal(err)
			}

			scanner := bufio.NewScanner(file)
			if err := a.updateProgress(scanner); err != nil {
				a.lineNumber = 0
				file.Seek(0, 0)
				scanner = bufio.NewScanner(file)
			}

			for scanner.Scan() {
				select {
				case <-a.ctx.Done():
					file.Close()
					return
				default:
					url := scanner.Text()
					a.sleepBasedOnQueueSize()
					a.urlChan <- url
				}
			}
			file.Close()

			// Once done reading file, reset lineNumber and save progress, then restart.
			a.lineNumber = 0
			a.saveProgress()
		}
	}
}

func (a *Administrator) readerWorker(id int) {
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		case url, ok := <-a.urlChan:
			a.incrementLineNumber()
			if !ok {
				return // channel closed
			}
			if a.bloomFilter.IsVisited(url) {
				continue // URL already visited
			}
			retryTime := 1.3
			// Insert URL into the queue, retry if full
			for {
				if retryTime > 10 {
					break
				}
				err := a.urlQueue.Insert(url)
				if err == nil { // Successfully inserted
					if domain, err := utils.GetDomainFromURL(url); err == nil {
						a.incrementDomainVisitCount(domain)
					}
					break
				} else {
					// Queue full, wait a bit and retry
					log.Printf("Worker %d: failed to insert %s: %v", id, url, err)
					time.Sleep(time.Duration(retryTime) * time.Second)
					retryTime *= retryTime // Exponential backoff
					select {
					case <-a.ctx.Done():
						return
					default:
					}
				}
			}
		}
	}
}

func (a *Administrator) fetcherWorker(id int) {
	a.setFetcherLog(id, "Fetcher Worker Started")
	defer a.wg.Done()
	for {
		a.setFetcherLog(id, "Fetcher Worker Loop Started")
		select {
		case <-a.ctx.Done():
			a.setFetcherLog(id, "Fetcher Worker Loop Ended Due to ctx.Done()")
			return
		default:
			a.setFetcherLog(id, "Fetcher Worker Loop Default Case")
			url, err := a.urlQueue.Remove()
			a.fetcherMap[id] = LogEntry{Time: time.Now().Format("15:04:05"), Message: "Fetcher Worker URL Removed"}
			if err != nil {
				a.setFetcherLog(id, "Fetcher Worker Queue Empty, Sleeping for 500ms")
				// Queue is empty, wait a bit before trying again
				time.Sleep(500 * time.Millisecond)
				continue
			}
			a.setFetcherLog(id, "Fetcher Worker URL Found, Fetching...")

			ctx, cancel := context.WithTimeout(a.ctx, 30 * time.Second)
			pageData, err := fetcher.Fetch(ctx, url) // Fetch the URL
			cancel()

			if err != nil {
				a.setFetcherLog(id, "Fetcher Worker Fetch Failed Due to Error " + err.Error())
				log.Printf("Worker %d: failed to fetch %s: %v", id, url, err)
				continue
			}
			a.setFetcherLog(id, "Fetcher Worker URL Fetched Successfully, URL: " + url)

			a.bloomFilter.MarkVisited(url)
			a.setFetcherLog(id, "Fetcher Worker URL Marked Visited")
			fmt.Printf("Worker %d: Fetched %s\n", id, url)

			/* // Uncomment this code to print the fetched data (JSON format)!
			   pageDataJSONBytes, err := json.MarshalIndent(pageData, "", "  ")
			   if err != nil {
			       panic(err)
			   }
			   fmt.Println(string(pageDataJSONBytes))
			*/

			// This code tries to enqueue links found on the page in a way that
			// avoids too many links from the same domain being contiguous in the queue.
			totalLinksEnqueued := 0
			internalLinksIdx := 0
			externalLinksIdx := 0
			a.setFetcherLog(id, "Fetcher Worker Getting Domain From URL")
			currentDomain, domainParseErr := utils.GetDomainFromURL(url)
			a.setFetcherLog(id, "Fetcher Worker Domain Parsed")

			for totalLinksEnqueued < 10 {
				a.setFetcherLog(id, "Fetcher Worker Looping Through Links")

				if internalLinksIdx >= len(pageData.InternalLinks) &&
					externalLinksIdx >= len(pageData.ExternalLinks) {
					break
				}

				if domainParseErr == nil && a.getDomainVisitCount(currentDomain) < domainLimit {

					for internalLinksIdx < len(pageData.InternalLinks) &&
						a.bloomFilter.IsVisited(pageData.InternalLinks[internalLinksIdx]) {
						internalLinksIdx++
					}

					if internalLinksIdx < len(pageData.InternalLinks) {
						err := a.urlQueue.Insert(pageData.InternalLinks[internalLinksIdx])
						if err == nil { // Success, increment domain visit count and break out of the loop.
							a.incrementDomainVisitCount(currentDomain)
							totalLinksEnqueued++
						}
						internalLinksIdx++
					}

				} else {
					internalLinksIdx = len(pageData.InternalLinks)
				}

				for externalLinksIdx < len(pageData.ExternalLinks) &&
					a.bloomFilter.IsVisited(pageData.ExternalLinks[externalLinksIdx]) {
					externalLinksIdx++
				}

				if externalLinksIdx < len(pageData.ExternalLinks) {
					domain, err := utils.GetDomainFromURL(pageData.ExternalLinks[externalLinksIdx])
					if err == nil && a.getDomainVisitCount(domain) < domainLimit {
						err := a.urlQueue.Insert(pageData.ExternalLinks[externalLinksIdx])
						if err == nil {
							a.incrementDomainVisitCount(domain)
							totalLinksEnqueued++
						} else {
							log.Printf("Worker %d: failed to insert %s: %v", id, pageData.ExternalLinks[externalLinksIdx], err)
						}
						
					}
					externalLinksIdx++
				}
			}
			a.setFetcherLog(id, "Fetcher Worker Links Enqueued")
		}
		a.setFetcherLog(id, "Fetcher Worker Loop Ended")
	}
}

func (a *Administrator) saveProgress() {
	a.progressMutex.Lock()
	defer a.progressMutex.Unlock()
	data := []byte(fmt.Sprintf("%d\n", a.lineNumber))
	err := os.WriteFile(a.progressFile, data, 0644)
	if err != nil {
		log.Printf("Error saving progress: %v", err)
	}
}

func (a *Administrator) loadProgress() int {
	data, err := os.ReadFile(a.progressFile)
	if err != nil {
		return 0
	}
	lineNum, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return lineNum
}

func (a *Administrator) updateProgress(scanner *bufio.Scanner) error {
	currentLine := 0
	for currentLine < a.lineNumber {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error while skipping lines: %v", err)
			}
			return fmt.Errorf("reached EOF after skipping %d lines, expected to skip %d", currentLine, a.lineNumber)
		}
		currentLine++
	}
	return nil
}

func (a *Administrator) ShutDown() {
	fmt.Printf("Shutting down administrator. Current Crawler Status: {\nQueue Usage: %v\n, Domain Visits: %v\n, Line Number: %v\n, Bloom Filter: %v\n}\n\n\n", a.getQueueUsage(), a.domainVisits, a.lineNumber, a.bloomFilter)
	for i, logEntry := range a.getFetcherLogs() {
		fmt.Printf("Fetcher Worker %d: %v\n", i, logEntry)
	}
	fmt.Println("\n\n\nShutting down administrator")
	a.cancel()
	a.wg.Wait()
}

func (a *Administrator) setFetcherLog(id int, message string) {
    a.fetcherMapMutex.Lock()
    defer a.fetcherMapMutex.Unlock()

    a.fetcherMap[id] = LogEntry{
        Time:    time.Now().Format("15:04:05"),
        Message: message,
    }
}

func (a *Administrator) getFetcherLogs() map[int]LogEntry {
    a.fetcherMapMutex.Lock()
    defer a.fetcherMapMutex.Unlock()

    // Return a copy if you wish to avoid exposing the original map
    copyMap := make(map[int]LogEntry, len(a.fetcherMap))
    for k, v := range a.fetcherMap {
        copyMap[k] = v
    }
    return copyMap
}
