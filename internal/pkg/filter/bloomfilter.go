package bloomfilter

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sync"
	"github.com/bits-and-blooms/bloom/v3"
)

// This is a wrapper around a Bloom filter that provides thread-safe access to it.
type BloomFilterManager struct {
	filter      *bloom.BloomFilter
	mutex       sync.Mutex
	savePath    string
	saveEvery   int
	saveCounter int
}

// Creates a new BloomFilterManager instance.
func NewBloomFilterManager(savePath string, saveEvery int, capacity int, fpRate float64) (*BloomFilterManager, error) {
	manager := &BloomFilterManager{
		savePath:  savePath,
		saveEvery: saveEvery,
	}

	// Attempt to load the bloom filter from disk
	filter, err := loadBloomFilter(savePath)
	if err != nil {
		return nil, fmt.Errorf("error while loading bloom filter: %v", err)
	}

	// No filter found, create a new one
	if filter == nil {
		filter = bloom.NewWithEstimates(uint(capacity), fpRate)
	}

	manager.filter = filter

	return manager, nil
}

// Loads a Bloom filter from disk.
func loadBloomFilter(path string) (*bloom.BloomFilter, error) {
	// Check if the file exists
	file, err := os.Open(path)
	if os.IsNotExist(err) { // File does not exist, just return nil
		return nil, nil
	}
	if err != nil { // File does exist, but there were problems opening it
		return nil, fmt.Errorf("error while opening bloom filter file on disk: %v", err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	filter := &bloom.BloomFilter{}
	if _, err := filter.ReadFrom(reader); err != nil { // problems reading the file
		return nil, fmt.Errorf("error while reading bloom filter from disk: %v", err)
	}

	return filter, nil
}

// Save persists the Bloom filter to disk.
func (filterManager *BloomFilterManager) save() error {
    filterManager.mutex.Lock()
    defer filterManager.mutex.Unlock()

    file, err := os.Create(filterManager.savePath)
    if err != nil {
        return err
    }
    defer file.Close()

    writer := bufio.NewWriter(file)
    _, err = filterManager.filter.WriteTo(writer)
    if err != nil {
        return err
    }
    return writer.Flush()
}

// Checks if a URL has been visited.
func (filterManager *BloomFilterManager) IsVisited(url string) bool {
    filterManager.mutex.Lock()
    defer filterManager.mutex.Unlock()
    return filterManager.filter.TestString(url)
}

// Checks if a URL has been visited and marks it as visited.
func (filterManager *BloomFilterManager) CheckAndMark(url string) bool {
    filterManager.mutex.Lock()
    defer filterManager.mutex.Unlock()
    
    if filterManager.filter.TestString(url) {
        return true
    }
    
    filterManager.filter.AddString(url)
    filterManager.saveCounter++
    
    if filterManager.saveCounter >= filterManager.saveEvery {
        filterManager.saveCounter = 0
        go filterManager.asyncSave()
    }
    
    return false
}

// Saves the Bloom filter to disk asynchronously.
func (filterManager *BloomFilterManager) asyncSave() {
    if err := filterManager.save(); err != nil {
        log.Printf("Error saving Bloom filter: %v", err)
    }
}
