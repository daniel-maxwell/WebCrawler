package queue

import ( 
    "errors"
    "fmt"
) 

type Queue struct { 
    Elements []int 
    Size     int 
} 
  
func (q *Queue) Enqueue(elem string) { 
    if q.GetLength() == q.Size { 
        fmt.Println("Overflow") 
        return
    } 
    q.Elements = append(q.Elements, elem) 
} 
  
func (q *Queue) Dequeue() string { 
    if q.IsEmpty() { 
        fmt.Println("UnderFlow") 
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
  
func (q *Queue) GetLength() string { 
    return len(q.Elements) 
} 
  
func (q *Queue) IsEmpty() bool { 
    return len(q.Elements) == 0
} 
  
func (q *Queue) Peek() (int, error) { 
    if q.IsEmpty() { 
        return "", errors.New("empty queue") 
    } 
    return q.Elements[0], nil 
} 
