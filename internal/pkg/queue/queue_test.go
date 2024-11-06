package queue

import (
	"testing"
)

func TestCreateQueue(t *testing.T) {
	q, err := CreateQueue(3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.capacity != 3 {
		t.Errorf("Expected queue size to be 3, got %d", q.capacity)
	}

	q, err = CreateQueue(1000000)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.capacity != 1000000 {
		t.Errorf("Expected queue size to be 1000000, got %d", q.capacity)
	}

	q, err = CreateQueue(0)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if q != nil {
		t.Errorf("Expected queue to be nil, got %v", q)
	}

	q, err = CreateQueue(-1)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if q != nil {
		t.Errorf("Expected queue to be nil, got %v", q)
	}
}

func TestInsert(t *testing.T) {
	q, err := CreateQueue(3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.Length())
	}

	err = q.Insert("a")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.Length())
	}

	err = q.Insert("b")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.Length())
	}

	err = q.Insert("c")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 3 {
		t.Errorf("Expected queue length to be 3, got %d", q.Length())
	}

	err = q.Insert("d")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if q.Length() != 3 {
		t.Errorf("Queue should be full, expected queue length to be 3, got %d", q.Length())
	}
}

func TestRemove(t *testing.T) {
	q, err := CreateQueue(3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	q.Insert("a")
	q.Insert("b")
	q.Insert("c")

	elem, err := q.Remove()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if elem != "a" {
		t.Errorf("Expected Removed element to be 'a', got '%s'", elem)
	}
	if q.Length() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.Length())
	}

	elem, err = q.Remove()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if elem != "b" {
		t.Errorf("Expected Removed element to be 'b', got '%s'", elem)
	}
	if q.Length() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.Length())
	}

	elem, err = q.Remove()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if elem != "c" {
		t.Errorf("Expected Removed element to be 'c', got '%s'", elem)
	}
	if q.Length() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.Length())
	}

	elem, err = q.Remove()
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if elem != "" {
		t.Errorf("Queue should be empty and return empty string. Expected Removed element to be '', got '%s'", elem)
	}
	if q.Length() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.Length())
	}
}

func TestLength(t *testing.T) {
	q, err := CreateQueue(3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.Length())
	}

	err = q.Insert("a")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.Length())
	}

	err = q.Insert("b")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.Length())
	}

	err = q.Insert("c")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if q.Length() != 3 {
		t.Errorf("Expected queue length to be 3, got %d", q.Length())
	}

	err = q.Insert("d")
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if q.Length() != 3 {
		t.Errorf("Queue should be full, expected queue length to be 3, got %d", q.Length())
	}

	value, err := q.Remove()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != "a" {
		t.Errorf("Expected Removed element to be 'a', got '%s'", value)
	}
	if q.Length() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.Length())
	}


	value, err = q.Remove()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != "b" {
		t.Errorf("Expected Removed element to be 'b', got '%s'", value)
	}
	if q.Length() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.Length())
	}

	value, err = q.Remove()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != "c" {
		t.Errorf("Expected Removed element to be 'c', got '%s'", value)
	}
	if q.Length() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.Length())
	}

	value, err = q.Remove()
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
	if value != "" {
		t.Errorf("Queue should be empty and return empty string. Expected Removed element to be '', got '%s'", value)
	}
}

func TestIsEmpty(t *testing.T) {
	q, err := CreateQueue(3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !q.IsEmpty() {
		t.Errorf("Expected queue to be empty")
	}
	q.Insert("a")
	if q.IsEmpty() {
		t.Errorf("Expected queue to not be empty")
	}
	q.Remove()
	if !q.IsEmpty() {
		t.Errorf("Expected queue to be empty again")
	}
}
