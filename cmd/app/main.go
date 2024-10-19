package main

// https://www.enjoyalgorithms.com/blog/web-crawler

import (
    "fmt"
    "webcrawler/internal/pkg/administrator"
)

func main() {
    fmt.Println("Main Called")
    administrator.Run()
}