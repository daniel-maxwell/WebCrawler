package main

// https://www.enjoyalgorithms.com/blog/web-crawler

import (
    "fmt"
    "webcrawler/internal/pkg/seeder"
)

func main() {
    fmt.Println("Main called")
    seeder.CreateSeeder(0)
}