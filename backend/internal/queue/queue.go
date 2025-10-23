package queue

import (
	"fmt"
	"sync"
)

type Job struct {
	Fn   func() error
	Errc chan error
}

type RequestQueueManager struct {
	JobQueue      chan Job
	MaxWorkers    int
	wg            sync.WaitGroup
}

func NewRequestQueueManager(queueSize int, maxWorkers int) *RequestQueueManager {
	manager := &RequestQueueManager{
		JobQueue:      make(chan Job, queueSize),
		MaxWorkers:    maxWorkers,
	}
	manager.startWorkers()
	return manager
}

func (rqm *RequestQueueManager) startWorkers() {
	for i := 0; i < rqm.MaxWorkers; i++ {
		rqm.wg.Add(1)
		go func(workerID int) {
			defer rqm.wg.Done()
			fmt.Printf("Worker %d started\n", workerID)
			for job := range rqm.JobQueue {
				err := job.Fn()
				if job.Errc != nil {
					job.Errc <- err
				}
			}
			fmt.Printf("Worker %d stopped\n", workerID)
		}(i)
	}
}

func (rqm *RequestQueueManager) EnqueueJob(job Job) {
	rqm.JobQueue <- job
}

func (rqm *RequestQueueManager) Shutdown() {
	close(rqm.JobQueue)
	rqm.wg.Wait()
}