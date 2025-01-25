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
	numFetcherWorkers = 10
	queueCapacity     = 100000
	domainLimit       = 100
	maxSleepMs        = 5000
)

type Administrator struct {
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	urlChan         chan string
	lineNumber      int
	progressFile    string
	progressMutex   sync.Mutex
	bloomFilter     *bloomfilter.BloomFilterManager
	urlQueue        *queue.Queue
	fetcherPool     *workerPool.WorkerPool
	domainVisits    map[string]int
	domainMutex     sync.Mutex
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
		ctx:          ctx,
		cancel:       cancel,
		urlChan:      make(chan string, 50), // TODO: Investigate buffer size requirements
		progressFile: progressFilePath,
		bloomFilter:  filter,
		urlQueue:     q,
		fetcherPool:  fetcherWorkerPool,
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

	// Instead of old fetcher goroutines, spawn N “queue consumer” goroutines:
	for i := 0; i < 5; i++ { // e.g. 5 concurrency for reading from queue
		a.wg.Add(1)
		go a.queueConsumer(i)
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

// queueConsumer pulls URLs from a.urlQueue, then calls the process worker
func (a *Administrator) queueConsumer(id int) {
    defer a.wg.Done()

    for {
        select {
        case <-a.ctx.Done():
            return
        default:
        }

        url, err := a.urlQueue.Remove()
        if err != nil {
            // queue empty, wait a bit
            time.Sleep(500 * time.Millisecond)
            continue
        }

        // domain checks, bloom filter checks, etc.
        if a.bloomFilter.IsVisited(url) {
            continue
        }

        // Now call the worker pool
        ctx, cancel := context.WithTimeout(a.ctx, 30 * time.Second)
        resp, err := a.fetcherPool.FetchURL(ctx, url)
        cancel()
        if err != nil {
            log.Printf("[queueConsumer %d] failed to fetch %s: %v\n", id, url, err)
            continue
        }

		fmt.Printf("[queueConsumer %d] Worker fetched URL=%s\n", id, resp.PageData.URL)

        // Optionally parse resp.Body for further links, or log it:
        //log.Printf("[queueConsumer %d] Worker fetched URL=%s status=%d", id, resp.URL, resp.StatusCode)
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
	fmt.Printf("Shutting down administrator...\n")
	// ...
	if a.fetcherPool != nil {
		a.fetcherPool.Shutdown()
	}
	fmt.Println("\n\n\nShutting down administrator")
	a.cancel()
	a.wg.Wait()
}
