package main

import (
    "encoding/gob"
    "log"
    "os"
    "time"
    "context"
    "webcrawler/internal/pkg/fetcher/fetcher"
    "webcrawler/internal/pkg/types"
)

// Exactly as before...
type Request struct {
    RequestID string
    URL       string
}
type Response struct {
    RequestID  string
    PageData   types.PageData
    FetchError string
    FetchTime  time.Duration
}

func main() {
    // Initialize fetcher as before (robots.txt, dialers, etc.)
    if err := fetcher.Init(); err != nil {
        log.Fatalf("Failed to init fetcher: %v", err)
    }
    defer fetcher.Shutdown()

    // Instead of using bufio.Scanner for lines, create a gob decoder
    dec := gob.NewDecoder(os.Stdin)
    // And a gob encoder for stdout
    enc := gob.NewEncoder(os.Stdout)

    for {
        // Read Request from stdin
        var request Request
        if err := dec.Decode(&request); err != nil {
            log.Printf("Gob decode error (likely EOF or invalid data): %v", err)
            return // worker exit
        }

        start := time.Now()

        // Actually fetch using your existing fetcher
        ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
        pageData, err := fetcher.Fetch(ctx, request.URL)
        cancel()
        elapsed := time.Since(start)

        // Create a Response
        response := Response{
            RequestID: request.RequestID,
            PageData:  pageData,
            FetchTime: elapsed,
        }
        if err != nil {
            response.FetchError = err.Error()
        }

        // 5) Write back with gob
        if err := enc.Encode(response); err != nil {
            log.Printf("Gob encode error: %v", err)
            return
        }
    }
}
