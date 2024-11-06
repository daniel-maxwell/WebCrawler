package administrator

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAdministratorRun(t *testing.T) {
	// Set up temporary directories and files for testing
	tempDir, err := os.MkdirTemp("", "administrator_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Temporary test files and directories
	progressFile := filepath.Join(tempDir, "progress.txt")
	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}
	dataFile := filepath.Join(dataDir, "top-1m.txt")

	urls := []string{
		"http://example.com",
		"http://example.org",
		"http://example.net",
	}

	// Write URLs to the data file
	if err := os.WriteFile(dataFile, []byte(strings.Join(urls, "\n")), 0644); err != nil {
		t.Fatalf("Failed to write data file: %v", err)
	}

	// Override the file path in the Administrator code to use the temporary data file
	originalDataFilePath := "internal/pkg/administrator/data/top-1m.txt"
	if err := os.MkdirAll(filepath.Dir(originalDataFilePath), 0755); err != nil {
		t.Fatalf("Failed to create original data directory: %v", err)
	}
	defer os.RemoveAll("internal")

	// Copy the temporary data file to the expected location
	inputData, err := os.ReadFile(dataFile)
	if err != nil {
		t.Fatalf("Failed to read temporary data file: %v", err)
	}
	if err := os.WriteFile(originalDataFilePath, inputData, 0644); err != nil {
		t.Fatalf("Failed to write to original data file path: %v", err)
	}

	admin := NewAdministrator(progressFile)

	// Run the Administrator in a separate goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		admin.Run()
	}()

	// Allow some time for the Administrator to process URLs
	time.Sleep(2 * time.Second)

	admin.ShutDown()

	wg.Wait()

	// Verify that the progress file has been updated correctly
	progressData, err := os.ReadFile(progressFile)
	if err != nil {
		t.Fatalf("Failed to read progress file: %v", err)
	}

	lineNumber, err := strconv.Atoi(strings.TrimSpace(string(progressData)))
	if err != nil {
		t.Fatalf("Failed to parse progress file content: %v", err)
	}

	// Check if the lineNumber matches the number of URLs processed, 
	// rounded down to the nearest 100
	expectedLineNumber := len(urls) / 100 * 100
	if lineNumber != expectedLineNumber {
		t.Errorf("Expected lineNumber to be %d, got %d", expectedLineNumber, lineNumber)
	}
}

func TestLoadAndSaveProgress(t *testing.T) {
	// Create a temporary progress file
	tempFile, err := os.CreateTemp("", "progress.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary progress file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Initialize the Administrator with the temporary progress file
	admin := NewAdministrator(tempFile.Name())

	// Set a line number and save progress
	admin.lineNumber = 42
	admin.saveProgress()

	// Load progress and verify the line number
	loadedLineNumber := admin.loadProgress()
	if loadedLineNumber != 42 {
		t.Errorf("Expected loaded line number to be 42, got %d", loadedLineNumber)
	}
}

func TestIncrementLineNumber(t *testing.T) {
	tempFile, err := os.CreateTemp("", "progress.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary progress file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	admin := NewAdministrator(tempFile.Name())

	// Set initial line number
	admin.lineNumber = 99

	// Increment line number and check if saveProgress is called
	admin.incrementLineNumber()
	if admin.lineNumber != 100 {
		t.Errorf("Expected lineNumber to be 100, got %d", admin.lineNumber)
	}

	// Reset the flag and increment again
	admin.incrementLineNumber()
	if admin.lineNumber != 101 {
		t.Errorf("Expected lineNumber to be 101, got %d", admin.lineNumber)
	}
}
