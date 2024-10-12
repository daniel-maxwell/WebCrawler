package seeder

import "fmt"

type AsyncURLSeeder interface {
    SeedChannel() (<-chan string, <-chan error)
}


