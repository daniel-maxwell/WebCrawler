package administrator

import (
    "fmt"
    "sync"
    "bufio"
    "log"
    "os"
    "io/ioutil"
    "strconv"
    "strings"
    "time"
)

var (
    lineNumber     int
    progressFile   = "internal/pkg/administrator/data/progress.txt"
    progressMutex  sync.Mutex
    visitedURLs    = make(map[string]struct{})
    visitedMutex   sync.RWMutex
    numWorkers     = 10
)

func Run() {
    fmt.Println("Administrator Called")

    // Load progress from file - to make crawler crash resistant
    lineNumber = loadProgress()

    // Open file containing top 1m most visited URLs
    file, err := os.Open("internal/pkg/administrator/data/top-1m.txt")
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    // Create a scanner to read the file line by line
    scanner := bufio.NewScanner(file)

    // Try to skip lines that have already been processed
    if err := updateProgress(scanner); err != nil {
        log.Printf("Failed to update progress, restarting from first line: %v", err)
        lineNumber = 0
    }

    urlChan := make(chan string, 100)
    var wg sync.WaitGroup

    // Start workers
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for url := range urlChan {
                fmt.Println("Processing: ", url)
                time.Sleep(1000 * time.Millisecond)
                incrementLineNumber()
            }
        }()
    }

    // Read URLs and send to channel
    for scanner.Scan() {
        url := scanner.Text()
        urlChan <- url
    }
    close(urlChan) // Close channel when done
    wg.Wait()      // Wait for all workers to finish
}


// Loads progress from progress file
func loadProgress() int {
    data, err := ioutil.ReadFile(progressFile)
    if err != nil {
        return 0
    }
    lineNum, err := strconv.Atoi(strings.TrimSpace(string(data)))
    if err != nil {
        return 0
    }
    return lineNum
}

// Updates the progress by skipping already processed lines or return an error
func updateProgress(scanner *bufio.Scanner) error {
    currentLine := 0
    for currentLine < lineNumber {
        if !scanner.Scan() {
            if err := scanner.Err(); err != nil {
                return fmt.Errorf("error while skipping lines: %v", err)
            }
            // EOF reached before skipping all lines
            return fmt.Errorf("reached EOF after skipping %d lines, expected to skip %d", currentLine, lineNumber)
        }
        currentLine++
    }
    return nil
}

// Increments the line number. Calls saveProgress every 100 lines.
func incrementLineNumber() {
    progressMutex.Lock()
    lineNumber++
    progressMutex.Unlock()
    if (lineNumber % 100) == 0 {
        saveProgress()
    }
}

// Saves the current line number to the progress file
func saveProgress() {
    progressMutex.Lock()
    defer progressMutex.Unlock()
    data := []byte(fmt.Sprintf("%d\n", lineNumber))
    err := ioutil.WriteFile(progressFile, data, 0644)
    if err != nil {
        log.Printf("Error saving progress: %v", err)
    }
}