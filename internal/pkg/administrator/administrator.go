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
	numFetcherWorkers = 30
	domainLimit       = 35
	queueCapacity     = 1000000
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
	filter, err := bloomfilter.NewBloomFilterManager("internal/pkg/filter/data/bloomfilter.dat", 1000, 1000000, 0.1)
	if err != nil {
		panic(fmt.Sprintf("Failed to create bloom filter: %v", err))
	}

	return &Administrator{
		ctx:          ctx,
		cancel:       cancel,
		urlChan:      make(chan string, 100), // TODO: Investigate buffer size requirements
		progressFile: progressFilePath,
		bloomFilter:  filter,
		urlQueue:     q,
		domainVisits: make(map[string]int),
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
			fmt.Println("Shutting down Run loop")
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
					// We don't close urlChan here again, it's done above.
					return
				default:
					url := scanner.Text()
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
			if !ok {
				return // channel closed
			}
			if a.bloomFilter.IsVisited(url) {
				continue // URL already visited
			}
			retryTime := 3
			// Insert URL into the queue, retry if full
			for {
				if retryTime > 100 {
					log.Printf("Worker %d: failed to insert %s after 3 retries", id, url)
				}
				err := a.urlQueue.Insert(url)
				if err == nil { // Successfully inserted
					if domain, err := utils.GetDomainFromURL(url); err == nil {
						a.incrementDomainVisitCount(domain)
					}
					a.incrementLineNumber()
					break
				} else {
					// Queue full, wait a bit and retry
					log.Printf("Worker %d: failed to insert %s: %v", id, url, err)
					time.Sleep(time.Duration(retryTime) * time.Millisecond)
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
	defer a.wg.Done()
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			url, err := a.urlQueue.Remove()
			if err != nil {
				// Queue is empty, wait a bit before trying again
				time.Sleep(50 * time.Millisecond)
				continue
			}
			// We have a URL, let's fetch it
			pageData, err := fetcher.Fetch(url)
			if err != nil {
				log.Printf("Worker %d: failed to fetch %s: %v", id, url, err)
				continue
			}

			a.bloomFilter.MarkVisited(url)

			/* // Uncomment this code to print the fetched data (JSON format)!
			   pageDataJSONBytes, err := json.MarshalIndent(pageData, "", "  ")
			   if err != nil {
			       panic(err)
			   }
			   fmt.Println(string(pageDataJSONBytes))
			*/
			fmt.Printf("Worker %d: Successfully Fetched %s\n", id, url)

			// This code tries to enqueue links found on the page in a way that
			// avoids too many links from the same domain being contiguous in the queue.
			totalLinksEnqueued := 0
			internalLinksIdx := 0
			externalLinksIdx := 0

			currentDomain, domainParseErr := utils.GetDomainFromURL(url)

			for totalLinksEnqueued < 10 {

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
						retryTime := 3
						for retryTime <= 100 {
							err := a.urlQueue.Insert(pageData.InternalLinks[internalLinksIdx])
							if err == nil { // Success, increment domain visit count and break out of the loop.
								a.incrementDomainVisitCount(currentDomain)
								break
							}
							// If we got here, err != nil, so log the failure and retry
							log.Printf("Worker %d: failed inserting %s: %v", id, pageData.InternalLinks[internalLinksIdx], err)
							time.Sleep(time.Duration(retryTime) * time.Millisecond)
							retryTime *= retryTime // Exponential backoff
						}
						totalLinksEnqueued++
						internalLinksIdx++
					}
				}

				for externalLinksIdx < len(pageData.ExternalLinks) &&
					a.bloomFilter.IsVisited(pageData.ExternalLinks[externalLinksIdx]) {
					externalLinksIdx++
				}

				if externalLinksIdx < len(pageData.ExternalLinks) {
					domain, err := utils.GetDomainFromURL(pageData.ExternalLinks[externalLinksIdx])
					if err == nil {
						visitsCount := a.getDomainVisitCount(domain)
						if visitsCount < domainLimit {
							retryTime := 3
							for retryTime <= 100 {
								err := a.urlQueue.Insert(pageData.ExternalLinks[externalLinksIdx])
								if err == nil { // Success! Break out of the loop.
									a.incrementDomainVisitCount(domain)
									break
								}
								// If we got here, err != nil, so log the failure and retry
								log.Printf("Worker %d: failed to insert %s: %v", id, pageData.ExternalLinks[externalLinksIdx], err)
								time.Sleep(time.Duration(retryTime) * time.Millisecond)
								retryTime *= retryTime // Exponential backoff
							}
							totalLinksEnqueued++
							externalLinksIdx++
						}
					}
				}
			}
		}
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
	a.cancel()
	a.wg.Wait()
}
