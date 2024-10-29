package queue

import ( 
    "errors"
) 

type Queue struct { 
    Elements []string
    Size     int
} 

// Creates a new queue with the specified size
func NewQueue(size int) *Queue {
    if size <= 0 {
        return nil
    }
    return &Queue{Size: size}
}

// Adds an element to the end of the queue
func (q *Queue) Enqueue(elem string) { 
    if q.GetLength() == q.Size { 
        return
    } 
    q.Elements = append(q.Elements, elem) 
} 

// Removes the first element of the queue
func (q *Queue) Dequeue() string { 
    if q.IsEmpty() { 
        return ""
    } 
    element := q.Elements[0] 
    if q.GetLength() == 1 { 
        q.Elements = nil 
        return element 
    } 
    q.Elements = q.Elements[1:] 
    return element // Slice off the element once it is dequeued. 
} 

// Returns the length of the queue
func (q *Queue) GetLength() int { 
    return len(q.Elements) 
} 

// Returns true if the queue is empty, false otherwise
func (q *Queue) IsEmpty() bool { 
    return len(q.Elements) == 0
} 

// Returns the first element of the queue
func (q *Queue) Peek() (string, error) { 
    if q.IsEmpty() { 
        return "", errors.New("empty queue") 
    } 
    return q.Elements[0], nil 
} 
