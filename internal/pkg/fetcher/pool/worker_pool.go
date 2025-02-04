package worker_pool

import (
    "os"
    "encoding/gob"
    "context"
    "fmt"
    "log"
    "math/rand"
    "os/exec"
    "sync"
    "time"
    "webcrawler/internal/pkg/types"
)

// Represents one child process
type Worker struct {
    id      	int
    cmd     	*exec.Cmd
    gobEncoder  *gob.Encoder
    gobDecoder  *gob.Decoder
    doneChannel chan struct{}
    mutex      	sync.Mutex
}

// Manages a pool of worker processes
type WorkerPool struct {
    size       		int
    workerChannel   chan *Worker // holds idle workers
    workers    		[]*Worker
    workerMutex   	sync.Mutex
    shutdownChannel chan struct{}
    waitGroup       sync.WaitGroup
}

// WorkerRequest / WorkerResponse mirror the IPC protocol
type WorkerRequest struct {
    RequestID string
    URL       string
}

type WorkerResponse struct {
    RequestID  string
    PageData   types.PageData
    FetchError string
    FetchTime  time.Duration
}

const (
    FETCHER_MAIN_PATH = "internal/pkg/fetcher/cmd/app/fetcher_main.go"
)

// Spawns `size` worker processes.
func NewWorkerPool(size int) (*WorkerPool, error) {
    workerPool := &WorkerPool{
        size:       	 size,
        workerChannel:   make(chan *Worker, size),
        shutdownChannel: make(chan struct{}),
    }

    for i := 0; i < size; i++ {
        worker, err := startWorker(i)
        if err != nil {
            return nil, fmt.Errorf("failed to start worker %d: %v", i, err)
        }
        workerPool.workers = append(workerPool.workers, worker)
        workerPool.workerChannel <- worker
    }
    return workerPool, nil
}

// Sends a URL to an idle worker, waits up to the context deadline
func (workerPool *WorkerPool) FetchURL(context context.Context, url string) (WorkerResponse, error) {
    response := WorkerResponse{}

    // Don’t accept new requests if shutting down
    select {
    case <-workerPool.shutdownChannel:
        return response, fmt.Errorf("worker pool is shutting down")
    default:
    }

    // Grab a worker
    var worker *Worker
    select {
    case worker = <-workerPool.workerChannel:
    case <-context.Done():
        return response, fmt.Errorf("no worker available before timeout: %w", context.Err())
    }

    // Add to wait group so we can wait if needed on shutdown
    workerPool.waitGroup.Add(1)
    defer workerPool.waitGroup.Done()

    // Send request
    request := WorkerRequest{
        RequestID: fmt.Sprintf("req-%d", rand.Int63()),
        URL:       url,
    }
    response, err := sendRequest(context, worker, request)
    if err != nil {
        // kill this worker and try to spawn a new one
        log.Printf("Killing worker %d due to error: %v", worker.id, err)
        killWorker(worker)
        newWorker, spawnErr := startWorker(worker.id)
        if spawnErr == nil {
            workerPool.replaceWorker(worker, newWorker)
        } else {
            log.Printf("[WorkerPool] failed to respawn worker %d: %v", worker.id, spawnErr)
        }
        return response, err
    }

    // success => put worker back
    workerPool.workerChannel <- worker
    return response, nil
}

// Kills all child processes and waits for in-flight requests to end.
func (workerPool *WorkerPool) Shutdown() {
    close(workerPool.shutdownChannel)

    workerPool.workerMutex.Lock()
    for _, worker := range workerPool.workers {
        killWorker(worker)
    }
    workerPool.workerMutex.Unlock()

    // Wait for any in-flight requests to finish
    workerPool.waitGroup.Wait()

    // Close the worker channel
    close(workerPool.workerChannel)

    // Clear the worker list
    workerPool.workers = []*Worker{}
}

// ### WORKER MANAGEMENT INTERNALS ###

// Starts a new worker process
func startWorker(id int) (*Worker, error) {
    cmd := exec.Command("go", "run", FETCHER_MAIN_PATH)
    // or a prebuilt binary: exec.Command("./worker_binary")

    stdoutPipe, err := cmd.StdoutPipe()
    if err != nil { return nil, err }
    
    stdinPipe, err := cmd.StdinPipe()
    if err != nil { return nil, err }

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    cmd.Stderr = os.Stderr

    worker := &Worker{
        id:     id,
        cmd:    cmd,
        gobEncoder: gob.NewEncoder(stdinPipe),
        gobDecoder: gob.NewDecoder(stdoutPipe),
        doneChannel: make(chan struct{}),
    }

    // Monitor for exit
    go func() {
        _ = cmd.Wait()
        close(worker.doneChannel)
    }()
    return worker, nil
}

// Kills a worker process
func killWorker(worker *Worker) {
    worker.mutex.Lock()
    defer worker.mutex.Unlock()
    _ = worker.cmd.Process.Kill()
    <-worker.doneChannel
}

// Sends a request to a worker and waits for a response
func sendRequest(context context.Context, worker *Worker, request WorkerRequest) (WorkerResponse, error) {
    var response WorkerResponse


    worker.mutex.Lock()
    if err := worker.gobEncoder.Encode(request); err != nil {
        return response, fmt.Errorf("failed to encode request gob: %w", err)
    }
    worker.mutex.Unlock()

    doneChannel := make(chan error, 1)
    go func() {
        worker.mutex.Lock()
        err := worker.gobDecoder.Decode(&response)
        worker.mutex.Unlock()
        doneChannel <- err
    }()

    // 3) Wait for decode or timeout
    select {
    case <-context.Done():
        return response, fmt.Errorf("request timed out: %w", context.Err())
    case err := <-doneChannel:
        if err != nil {
            return response, fmt.Errorf("gob decode error: %w", err)
        }
        return response, nil
    }
}

// Replaces an old worker with a new one
func (workerPool *WorkerPool) replaceWorker(oldWorker, newWorker *Worker) {
    workerPool.workerMutex.Lock()
    defer workerPool.workerMutex.Unlock()
    for i, worker := range workerPool.workers {
        if worker == oldWorker {
            workerPool.workers[i] = newWorker
            // put new worker in channel
            workerPool.workerChannel <- newWorker
            return
        }
    }
}
