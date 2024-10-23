package administrator

/*
 The administrator package is responsible for managing the progress of the workers. 
 Each worker reads a line from the file, processes the URL, and increments the line number.
 The administrator package reads the progress from a file, updates the progress, and saves the progress to the file every 100 lines.

 For each line, url proccer should collect 10 urls from the page. 
 Then 5 urls from all of those pages, 
 then 2 urls from all of those pages. 
 Then 1 url from all of those pages.
*/

import (
    "fmt"
    "sync"
    "bufio"
    "log"
    "os"
    "io/ioutil"
    "strconv"
    "strings"
    //"time"
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

    urlChan := make(chan string, 100)
    var wg sync.WaitGroup

    // Start workers
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for url := range urlChan {
                fmt.Println("Processing: ", url)
                // Process URL
                incrementLineNumber()
            }
        }()
    }

    // Continuous loop to restart from the top of the file
    for {
        // Load progress from file
        lineNumber = loadProgress()

        // Open file containing top 1m most visited URLs
        file, err := os.Open("internal/pkg/administrator/data/top-1m.txt")
        if err != nil {
            log.Fatal(err)
        }

        scanner := bufio.NewScanner(file)

        // Try to skip lines that have already been processed
        if err := updateProgress(scanner); err != nil {
            // Restart from the first line if unable to update progress
            lineNumber = 0
        }

        // Read URLs and send to channel
        for scanner.Scan() {
            url := scanner.Text()
            urlChan <- url
        }

        file.Close() // Close the file before restarting the loop

        // Reset the lineNumber to start from the beginning
        lineNumber = 0
        saveProgress()
    }

    // Note: In this continuous loop setup, you might not reach this point.
    // If you have a termination condition, you should handle closing the channel and waiting for workers.
    // close(urlChan)
    // wg.Wait()
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