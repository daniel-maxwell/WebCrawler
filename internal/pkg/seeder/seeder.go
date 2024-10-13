package seeder

import "fmt"

// Asynchronous seeder which returns a URL of a web page to be crawled when called.
type AsyncURLSeeder interface {
    SeedChannel() (<-chan string, <-chan error)
}