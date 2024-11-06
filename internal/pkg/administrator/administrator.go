package administrator

import (
	"bufio"
	"context"
	"fmt"
	"log"
	//"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"webcrawler/internal/pkg/fetcher"
	//"webcrawler/internal/pkg/queue"
)

const numWorkers = 10
var processedSingleUrl = false

type Administrator struct {
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	urlChan       chan string
	lineNumber    int
	progressFile  string
	progressMutex sync.Mutex
}

func NewAdministrator(progressFilePath string) *Administrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Administrator{
		ctx:          ctx,
		cancel:       cancel,
		urlChan:      make(chan string, 100),
		progressFile: progressFilePath,
	}
}

func (a *Administrator) Run() {
	fmt.Println("Administrator Called")

	// Start workers
	for i := 0; i < numWorkers; i++ {
		a.wg.Add(1)
		go func() {
			defer a.wg.Done()
			for url := range a.urlChan {
				select {
				case <-a.ctx.Done():
					return
				default:
					// Process URL
					processURL(url) // dummy method for now

					a.incrementLineNumber()
				}
			}
		}()
	}

	// Continuous loop to restart from the top of the file
	for {
		select {
		case <-a.ctx.Done():
			fmt.Println("Shutting down Run loop")
			close(a.urlChan)
			a.wg.Wait()
			return
		default:
			// Load progress from file
			a.lineNumber = a.loadProgress()

			// Open file containing top 1m most visited URLs
			file, err := os.Open("internal/pkg/administrator/data/top-1m.txt")
			if err != nil {
				log.Fatal(err)
			}

			scanner := bufio.NewScanner(file)

			// Try to skip lines that have already been processed
			if err := a.updateProgress(scanner); err != nil {
				// Restart from the first line if unable to update progress
				a.lineNumber = 0
				file.Seek(0, 0)
				scanner = bufio.NewScanner(file)
			}

			// Read URLs and send to channel
			for scanner.Scan() {
				select {
				case <-a.ctx.Done():
					file.Close()
					close(a.urlChan)
					a.wg.Wait()
					return
				default:
					url := scanner.Text()
					a.urlChan <- url
				}
			}

			file.Close() // Close the file before restarting the loop

			// Reset the lineNumber to start from the beginning
			a.lineNumber = 0
			a.saveProgress()
		}
	}
}

// ShutDown cancels the context and waits for all workers to finish
func (a *Administrator) ShutDown() {
	a.cancel()
	a.wg.Wait()
}

// Methods for progress handling
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

func (a *Administrator) saveProgress() {
	a.progressMutex.Lock()
	defer a.progressMutex.Unlock()
	data := []byte(fmt.Sprintf("%d\n", a.lineNumber))
	err := os.WriteFile(a.progressFile, data, 0644)
	if err != nil {
		log.Printf("Error saving progress: %v", err)
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

func (a *Administrator) updateProgress(scanner *bufio.Scanner) error {
	currentLine := 0
	for currentLine < a.lineNumber {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("error while skipping lines: %v", err)
			}
			// EOF reached before skipping all lines
			return fmt.Errorf("reached EOF after skipping %d lines, expected to skip %d", currentLine, a.lineNumber)
		}
		currentLine++
	}
	return nil
}

// Dummy method to simulate processing a URL
// This will eventually just push the URL to the back of the queue
func processURL(url string) string {
	fmt.Println("Navigating to URL:", url)
	response := ""
	if (!processedSingleUrl) {
		response = fetcher.Fetch(url)
		processedSingleUrl = true
	}
	return response
}
