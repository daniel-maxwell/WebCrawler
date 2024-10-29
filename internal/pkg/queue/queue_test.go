package queue

import (
	"testing"
)

func TestNewQueue(t *testing.T) {
	q := NewQueue(3)
	if q.Size != 3 {
		t.Errorf("Expected queue size to be 3, got %d", q.Size)
	}
	q = NewQueue(1000000)
	if q.Size != 1000000 {
		t.Errorf("Expected queue size to be 1000000, got %d", q.Size)
	}
	if len(q.Elements) != 0 {
		t.Errorf("Expected queue elements to be empty, got %v", q.Elements)
	}
	q = NewQueue(0)
	if q != nil {
		t.Errorf("Expected queue to be nil, got %v", q)
	}
	q = NewQueue(-1)
	if q != nil {
		t.Errorf("Expected queue to be nil, got %v", q)
	}
}

func TestEnqueue(t *testing.T) {
	q := NewQueue(3)
	q.Enqueue("a")
	if q.GetLength() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.GetLength())
	}
	q.Enqueue("b")
	if q.GetLength() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.GetLength())
	}
	q.Enqueue("c")
	if q.GetLength() != 3 {
		t.Errorf("Expected queue length to be 3, got %d", q.GetLength())
	}
	q.Enqueue("d")
	if q.GetLength() != 3 {
		t.Errorf("Queue should be full, expected queue length to be 3, got %d", q.GetLength())
	}
}

func TestDequeue(t *testing.T) {
	q := NewQueue(3)
	q.Enqueue("a")
	q.Enqueue("b")
	q.Enqueue("c")

	elem := q.Dequeue()
	if elem != "a" {
		t.Errorf("Expected dequeued element to be 'a', got '%s'", elem)
	}
	if q.GetLength() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.GetLength())
	}
	elem = q.Dequeue()
	if elem != "b" {
		t.Errorf("Expected dequeued element to be 'b', got '%s'", elem)
	}
	if q.GetLength() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.GetLength())
	}
	elem = q.Dequeue()
	if elem != "c" {
		t.Errorf("Expected dequeued element to be 'c', got '%s'", elem)
	}
	if q.GetLength() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.GetLength())
	}
	elem = q.Dequeue()
	if elem != "" {
		t.Errorf("Queue should be empty and return empty string. Expected dequeued element to be '', got '%s'", elem)
	}
}

func TestGetLength(t *testing.T) {
	q := NewQueue(3)
	if q.GetLength() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.GetLength())
	}
	q.Enqueue("a")
	if q.GetLength() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.GetLength())
	}
	q.Enqueue("b")
	if q.GetLength() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.GetLength())
	}
	q.Enqueue("c")
	if q.GetLength() != 3 {
		t.Errorf("Expected queue length to be 3, got %d", q.GetLength())
	}
	q.Enqueue("d")
	if q.GetLength() != 3 {
		t.Errorf("Queue should be full, expected queue length to be 3, got %d", q.GetLength())
	}
	q.Dequeue()
	if q.GetLength() != 2 {
		t.Errorf("Expected queue length to be 2, got %d", q.GetLength())
	}
	q.Dequeue()
	if q.GetLength() != 1 {
		t.Errorf("Expected queue length to be 1, got %d", q.GetLength())
	}
	q.Dequeue()
	if q.GetLength() != 0 {
		t.Errorf("Expected queue length to be 0, got %d", q.GetLength())
	}
}

func TestIsEmpty(t *testing.T) {
	q := NewQueue(3)
	if !q.IsEmpty() {
		t.Errorf("Expected queue to be empty")
	}
	q.Enqueue("a")
	if q.IsEmpty() {
		t.Errorf("Expected queue to not be empty")
	}
	q.Dequeue()
	if !q.IsEmpty() {
		t.Errorf("Expected queue to be empty again")
	}
}

func TestPeek(t *testing.T) {
	q := NewQueue(3)
	elem, err := q.Peek()
	if elem != "" || err == nil {
		t.Errorf("Expected empty queue to return empty string and error")
	}
	q.Enqueue("a")
	elem, err = q.Peek()
	if elem != "a" || err != nil {
		t.Errorf("Expected queue with one element to return 'a' and no error")
	}
	q.Enqueue("b")
	elem, err = q.Peek()
	if elem != "a" || err != nil {
		t.Errorf("Expected queue with two elements to return 'a' and no error")
	}
	q.Dequeue()
	elem, err = q.Peek()
	if elem != "b" || err != nil {
		t.Errorf("Expected queue with one element to return 'b' and no error")
	}
	q.Dequeue()
	elem, err = q.Peek()
	if elem != "" || err == nil {
		t.Errorf("Expected empty queue to return empty string and error")
	}
}