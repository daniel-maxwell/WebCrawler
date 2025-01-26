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
    if err := fetcher.Init(); err != nil {
        log.Fatalf("Failed to init fetcher: %v", err)
    }
    defer fetcher.Shutdown()

    dec := gob.NewDecoder(os.Stdin)
    enc := gob.NewEncoder(os.Stdout)

    for {
        var request Request
        if err := dec.Decode(&request); err != nil {
            log.Printf("Gob decode error (likely EOF or invalid data): %v", err)
            return // worker exit
        }

        start := time.Now()

        ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
        pageData, err := fetcher.Fetch(ctx, request.URL)
        cancel()
        elapsed := time.Since(start)

        response := Response{
            RequestID: request.RequestID,
            PageData:  pageData,
            FetchTime: elapsed,
        }
        if err != nil {
            response.FetchError = err.Error()
        }

        if err := enc.Encode(response); err != nil {
            log.Printf("Gob encode error: %v", err)
            return
        }
    }
}
