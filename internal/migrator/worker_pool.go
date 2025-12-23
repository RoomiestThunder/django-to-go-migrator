package migrator

import (
	"context"
	"fmt"
	"sync"
)

// Worker processes tasks from the job channel
type Worker struct {
	id       int
	jobsChan <-chan interface{}
	wg       *sync.WaitGroup
	process  ProcessFunc
}

// ProcessFunc is a function for processing a task
type ProcessFunc func(ctx context.Context, job interface{}) error

// WorkerPool manages a pool of workers for parallel processing
type WorkerPool struct {
	workers     []*Worker
	jobsChan    chan interface{}
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	processFunc ProcessFunc
	errorsChan  chan error
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(numWorkers int, processFunc ProcessFunc) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:     make([]*Worker, numWorkers),
		jobsChan:    make(chan interface{}, numWorkers*2),
		ctx:         ctx,
		cancel:      cancel,
		processFunc: processFunc,
		errorsChan:  make(chan error, numWorkers),
	}

	// Create workers
	for i := 0; i < numWorkers; i++ {
		pool.workers[i] = &Worker{
			id:       i + 1,
			jobsChan: pool.jobsChan,
			wg:       &pool.wg,
			process:  processFunc,
		}
		pool.wg.Add(1)
		go pool.workers[i].start(ctx, pool.errorsChan)
	}

	return pool
}

// Submit adds a task to the queue
func (wp *WorkerPool) Submit(job interface{}) error {
	select {
	case wp.jobsChan <- job:
		return nil
	case <-wp.ctx.Done():
		return fmt.Errorf("worker pool is closed")
	}
}

// Wait waits for all tasks to complete
func (wp *WorkerPool) Wait() []error {
	close(wp.jobsChan)
	wp.wg.Wait()
	wp.cancel()

	close(wp.errorsChan)
	var errors []error
	for err := range wp.errorsChan {
		if err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.cancel()
	close(wp.jobsChan)
}

// start runs the worker
func (w *Worker) start(ctx context.Context, errChan chan error) {
	defer w.wg.Done()

	for {
		select {
		case job, ok := <-w.jobsChan:
			if !ok {
				return
			}
			if err := w.process(ctx, job); err != nil {
				select {
				case errChan <- fmt.Errorf("worker %d: %w", w.id, err):
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// StreamProcessor processes data stream through worker pool
type StreamProcessor struct {
	pool          *WorkerPool
	batchSize     int64
	totalRows     int64
	processedRows int64
	mutex         sync.RWMutex
}

// NewStreamProcessor creates a new stream processor
func NewStreamProcessor(numWorkers int, batchSize int64, processFunc ProcessFunc) *StreamProcessor {
	return &StreamProcessor{
		pool:      NewWorkerPool(numWorkers, processFunc),
		batchSize: batchSize,
	}
}

// Submit adds data for processing
func (sp *StreamProcessor) Submit(data interface{}) error {
	sp.mutex.Lock()
	sp.totalRows++
	sp.mutex.Unlock()

	return sp.pool.Submit(data)
}

// Wait waits for all processing to complete
func (sp *StreamProcessor) Wait() []error {
	return sp.pool.Wait()
}

// GetStats returns processing statistics
func (sp *StreamProcessor) GetStats() (total int64, processed int64) {
	sp.mutex.RLock()
	defer sp.mutex.RUnlock()
	return sp.totalRows, sp.processedRows
}
