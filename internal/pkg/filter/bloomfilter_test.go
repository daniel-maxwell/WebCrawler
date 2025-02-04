package bloomfilter

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestNewBloomFilterManager_LoadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "filter.dat")

	// Create and save initial filter
	initialManager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)
	initialManager.MarkVisited("test-url")

	// Load the saved filter
	newManager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)
	assert.True(t, newManager.IsVisited("test-url"))
}

func TestNewBloomFilterManager_NewFilterWhenNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent.dat")

	manager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)
	assert.False(t, manager.IsVisited("test-url"))
}

func TestNewBloomFilterManager_ErrorOpeningFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dummy")

	// Create a directory to simulate existing path
	assert.NoError(t, os.Mkdir(path, 0755))

	_, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.Error(t, err)
}

func TestIsVisited_ReturnsFalseForNewFilter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "filter.dat")

	manager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)
	assert.False(t, manager.IsVisited("new-url"))
}

func TestMarkVisited_AddsURLToFilter(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "filter.dat")

	manager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)

	url := "http://example.com"
	manager.MarkVisited(url)
	assert.True(t, manager.IsVisited(url))
}

func TestMarkVisited_SavesAfterThreshold(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "filter.dat")

	manager, err := NewBloomFilterManager(path, 2, 1000, 0.01)
	assert.NoError(t, err)

	manager.MarkVisited("url1")
	manager.MarkVisited("url2") // Triggers save

	newManager, err := NewBloomFilterManager(path, 2, 1000, 0.01)
	assert.NoError(t, err)
	assert.True(t, newManager.IsVisited("url1"))
	assert.True(t, newManager.IsVisited("url2"))
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "filter.dat")

	manager, err := NewBloomFilterManager(path, 100, 1000, 0.01)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	urls := []string{"url1", "url2", "url3", "url4"}

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			manager.MarkVisited(u)
		}(url)
	}
	wg.Wait()

	for _, url := range urls {
		assert.True(t, manager.IsVisited(url))
	}
}

func TestLoadBloomFilter_InvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.dat")

	assert.NoError(t, os.WriteFile(path, []byte("invalid data"), 0644))
	_, err := loadBloomFilter(path)
	assert.Error(t, err)
}

func TestSave_FileIsCreated(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "filter.dat")

	manager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)

	// File should not exist before saving
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))

	manager.MarkVisited("url1") // Triggers save
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestSave_NonexistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nonexistent", "filter.dat")

	manager, err := NewBloomFilterManager(path, 1, 1000, 0.01)
	assert.NoError(t, err)

	manager.MarkVisited("url1")
	_, err = os.Stat(path)
	assert.True(t, os.IsNotExist(err))
}
