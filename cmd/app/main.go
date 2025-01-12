package main

// https://www.enjoyalgorithms.com/blog/web-crawler

import (
	"fmt"
	"webcrawler/internal/pkg/administrator"
)

func main() {
	fmt.Println("Main Called")
	administrator := administrator.NewAdministrator("internal/pkg/administrator/data/progress.txt")
	administrator.Run() // Careful! This will run indefinitely.
	administrator.ShutDown()
}
