package main

// https://www.enjoyalgorithms.com/blog/web-crawler

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"webcrawler/internal/pkg/administrator"
)

func main() {
	fmt.Println("Main Called")
	administrator := administrator.NewAdministrator("internal/pkg/administrator/data/progress.txt")
	defer administrator.ShutDown()

	// Set up a channel to listen for interrupt or terminate signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived stop signal. Shutting down gracefully...")
		// Call ShutDown on the administrator
		administrator.ShutDown()
	}()
	administrator.Run() // Careful! This will run indefinitely.
}
