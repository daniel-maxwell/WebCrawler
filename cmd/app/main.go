package main

// https://www.enjoyalgorithms.com/blog/web-crawler

import (
	"fmt"
	"webcrawler/internal/pkg/administrator"
)

func main() {
	fmt.Println("Main Called")
	administrator := administrator.NewAdministrator("internal/pkg/administrator/data/progress.txt")
	defer administrator.ShutDown()
    administrator.Run() // Careful! This will run indefinitely.
}
