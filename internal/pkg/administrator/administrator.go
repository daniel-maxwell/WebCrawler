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
    "webcrawler/internal/pkg/fetcher"
    "webcrawler/internal/pkg/queue"
)

const (
    numReaderWorkers  = 5
    numFetcherWorkers = 10
    queueCapacity     = 500
)

type Administrator struct {
    ctx          context.Context
    cancel       context.CancelFunc
    wg           sync.WaitGroup
    urlChan      chan string
    lineNumber   int
    progressFile string

    progressMutex sync.Mutex

    urlQueue *queue.Queue
}

func NewAdministrator(progressFilePath string) *Administrator {
    ctx, cancel := context.WithCancel(context.Background())

    q, err := queue.CreateQueue(queueCapacity)
    if err != nil {
        panic(fmt.Sprintf("Failed to create queue: %v", err))
    }

    return &Administrator{
        ctx:          ctx,
        cancel:       cancel,
        urlChan:      make(chan string, 100),
        progressFile: progressFilePath,
        urlQueue:     q,
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

    // Continuous loop - intended behavior
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
            // Insert URL into the queue, retry if full
            for {
                err := a.urlQueue.Insert(url)
                if err == nil {
                    // Successfully inserted
                    a.incrementLineNumber()
                    break
                } else {
                    // Queue full, wait a bit and retry
                    time.Sleep(100 * time.Millisecond)
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
            // For now, just print the fetched data
            fmt.Printf("Worker %d: Fetched %s -> Title: %s\n", id, pageData.URL, pageData.Title)
        }
    }
}

func (a *Administrator) incrementLineNumber() {
    a.progressMutex.Lock()
    a.lineNumber++
    currentLineNumber := a.lineNumber
    a.progressMutex.Unlock()

    if (currentLineNumber % 100) == 0 {
        a.saveProgress()
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
