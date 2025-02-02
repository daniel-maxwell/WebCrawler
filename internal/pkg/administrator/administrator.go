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
	"webcrawler/internal/pkg/fetcher/fetcher"
	workerPool "webcrawler/internal/pkg/fetcher/pool"
	bloomfilter "webcrawler/internal/pkg/filter"
	"webcrawler/internal/pkg/queue"
	"webcrawler/internal/pkg/utils"
)

// TODO: Investigate the following constants to optimize for production
const (
	numReaderWorkers  = 3
	numQueueConsumers = 15
	queueCapacity     = 10000
	domainLimit       = 100 // Note: Limit is doubled for .org and .edu domains
	maxSleepMs        = 100000
)

type Administrator struct {
	context       context.Context
	cancel        context.CancelFunc
	waitGroup     sync.WaitGroup
	urlChan       chan string
	lineNumber    int
	progressFile  string
	progressMutex sync.Mutex
	bloomFilter   *bloomfilter.BloomFilterManager
	urlQueue      *queue.Queue
	fetcherPool   *workerPool.WorkerPool
	domainVisits  map[string]int
	domainMutex   sync.Mutex
}

// Creates a new Administrator instance
func NewAdministrator(progressFilePath string) *Administrator {
	context, cancel := context.WithCancel(context.Background())

	q, err := queue.CreateQueue(queueCapacity)
	if err != nil {
		panic(fmt.Sprintf("Failed to create queue: %v", err))
	}

	err = fetcher.Init()
	if err != nil {
		panic(err)
	}

	// Initialize the new WorkerPool of size 10
	fetcherWorkerPool, err := workerPool.NewWorkerPool(10)
	if err != nil {
		panic(fmt.Sprintf("Failed to create fetcher worker pool: %v", err))
	}

	// Create a new bloom filter manager - capacity will need to be increased for prod
	filter, err := bloomfilter.NewBloomFilterManager("internal/pkg/filter/data/bloomfilter.dat", 1000, 1000000, 0.01)
	if err != nil {
		panic(fmt.Sprintf("Failed to create bloom filter: %v", err))
	}

	return &Administrator{
		context:      context,
		cancel:       cancel,
		urlChan:      make(chan string, 50), // TODO: Investigate buffer size requirements
		progressFile: progressFilePath,
		bloomFilter:  filter,
		urlQueue:     q,
		fetcherPool:  fetcherWorkerPool,
		domainVisits: make(map[string]int),
	}
}

// Starts the administrator and workers
func (admin *Administrator) Run() {
	fmt.Println("Administrator Run Called")

	// Start Reader Workers
	for i := 0; i < numReaderWorkers; i++ {
		admin.waitGroup.Add(1)
		go admin.readerWorker(i)
	}

	// Instead of old fetcher goroutines, spawn N “queue consumer” goroutines:
	for i := 0; i < numQueueConsumers; i++ { // concurrency for reading from queue
		admin.waitGroup.Add(1)
		go admin.queueConsumer(i)
	}

	// Continuous loop
	for {
		select {
		case <-admin.context.Done():
			close(admin.urlChan) // No more URLs to read from file
			admin.waitGroup.Wait()
			return
		default:
			admin.lineNumber = admin.loadProgress()

			file, err := os.Open("internal/pkg/administrator/data/top-1m.txt")
			if err != nil {
				log.Fatal(err)
			}

			scanner := bufio.NewScanner(file)
			if err := admin.updateProgress(scanner); err != nil {
				admin.lineNumber = 0
				file.Seek(0, 0)
				scanner = bufio.NewScanner(file)
			}

			for scanner.Scan() {
				select {
				case <-admin.context.Done():
					file.Close()
					return
				default:
					url := scanner.Text()
					admin.sleepBasedOnQueueSize()
					admin.urlChan <- url
				}
			}
			file.Close()

			// Once done reading file, reset lineNumber and save progress, then restart.
			admin.lineNumber = 0
			admin.saveProgress()
		}
	}
}

// Reads URLs from the file and sends them to the URL channel
func (admin *Administrator) readerWorker(id int) {
	defer admin.waitGroup.Done()
	for {
		select {
		case <-admin.context.Done():
			return
		case url, ok := <-admin.urlChan:
			admin.incrementLineNumber()
			if !ok {
				return // channel closed
			}
			if admin.bloomFilter.CheckAndMark(url) {
				continue // URL already visited
			}
			retryTime := 1.3
			// Insert URL into the queue, retry if full
			for {
				if retryTime > 10 {
					break
				}
				err := admin.urlQueue.Insert(url)
				if err == nil { // Successfully inserted
					if domain, err := utils.GetDomainFromURL(url); err == nil {
						admin.incrementDomainVisitCount(domain)
					}
					break
				} else {
					// Queue full, wait a bit and retry
					log.Printf("Worker %d: failed to insert %s: %v", id, url, err)
					time.Sleep(time.Duration(retryTime) * time.Second)
					retryTime *= retryTime // Exponential backoff
					select {
					case <-admin.context.Done():
						return
					default:
					}
				}
			}
		}
	}
}

// Pulls URLs from admin.urlQueue, then calls the process worker
func (admin *Administrator) queueConsumer(id int) {
	defer admin.waitGroup.Done()

	for {
		select {
		case <-admin.context.Done():
			return
		default:
		}

		url, err := admin.urlQueue.Remove()
		if err != nil { // queue empty, wait a bit
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if admin.bloomFilter.IsVisited(url) {
			continue
		} else {
			if domain, err := utils.GetDomainFromURL(url); err == nil {
				if fullUrl, err := utils.BuildFullUrl(url); err == nil {
					if domain != fullUrl {
						admin.bloomFilter.CheckAndMark(url)
					}
				} else {
					admin.bloomFilter.CheckAndMark(url)
				}
			} else {
				admin.bloomFilter.CheckAndMark(url)
			}
		}

		// Now call the worker pool
		context, cancel := context.WithTimeout(admin.context, 30 * time.Second)
		response, err := admin.fetcherPool.FetchURL(context, url)
		cancel()
		if err != nil {
			log.Printf("[queueConsumer %d] error in call to fetchURL %s: %v\n", id, url, err)
			continue
		} else {
			if response.FetchError == "" {
				fmt.Printf("queueConsumer Worker %d fetched URL: [%s] | Title: [%s] \n", id, response.PageData.URL, response.PageData.Title)
				admin.enqueueExtractedURLs(url, response.PageData.InternalLinks, response.PageData.ExternalLinks)
			}
		}
	}
}

// Saves the current line number to the progress file
func (admin *Administrator) saveProgress() {
	admin.progressMutex.Lock()
	defer admin.progressMutex.Unlock()
	data := []byte(fmt.Sprintf("%d\n", admin.lineNumber))
	err := os.WriteFile(admin.progressFile, data, 0644)
	if err != nil {
		log.Printf("Error saving progress: %v", err)
	}
}

// Loads the current line number from the progress file
func (admin *Administrator) loadProgress() int {
	data, err := os.ReadFile(admin.progressFile)
	if err != nil {
		return 0
	}
	lineNum, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return lineNum
}

// Skips lines until reaching the last saved line number
func (admin *Administrator) updateProgress(scanner *bufio.Scanner) error {
	currentLine := 0
	for currentLine < admin.lineNumber {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error while skipping lines: %v", err)
			}
			return fmt.Errorf("reached EOF after skipping %d lines, expected to skip %d", currentLine, admin.lineNumber)
		}
		currentLine++
	}
	return nil
}

// Shuts down the administrator
func (admin *Administrator) ShutDown() {
	fmt.Printf("Shutting down administrator. Current Crawler Status: {\nQueue Usage: %v\n, Domain Visits: %v\n, Line Number: %v\n, Bloom Filter: %v\n}\n\n\n", admin.getQueueUsage(), admin.domainVisits, admin.lineNumber, admin.bloomFilter)
	fmt.Printf("Shutting down administrator...\n")
	if admin.fetcherPool != nil {
		admin.fetcherPool.Shutdown()
	}
	fmt.Println("\n\n\nShutting down administrator")
	admin.cancel()
	admin.waitGroup.Wait()
}
