package concurrent_hustles

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// PriorityJob represents a unit of work with a priority
type PriorityJob struct {
	ID       int
	Priority int
	Work     func(ctx context.Context) error
	cancel   context.CancelFunc
	ack      chan struct{}
}

// WorkScheduler is the interface you need to implement
type WorkScheduler interface {
	// Start begins processing jobs with the given number of workers
	Start(ctx context.Context, numWorkers int)

	// Submit adds a job to the scheduler
	Submit(job PriorityJob) error

	// Wait blocks until all submitted jobs are completed
	Wait()

	// Stop gracefully shuts down the scheduler
	Stop()
}

type pq []*PriorityJob

func (q pq) Len() int {
	return len(q)
}

func (q pq) Less(i, j int) bool {
	return q[i].Priority > q[j].Priority
}

func (q pq) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q *pq) Push(x any) {
	*q = append(*q, x.(*PriorityJob))
}

func (q *pq) Pop() any {
	x := (*q)[q.Len()-1]
	(*q)[q.Len()-1] = nil
	*q = (*q)[:q.Len()-1]
	return x
}

// use only for minheap
func (q pq) peekLowestPriority() *PriorityJob {
	if q.Len() == 0 {
		return nil
	}
	return q[0]
}

func (q *pq) Remove(id int) {
	var ind = -1
	for i := range *q {
		if (*q)[i].ID == id {
			ind = i
			break
		}
	}
	if ind == -1 {
		return
	}
	*q = append((*q)[:ind], (*q)[ind+1:]...)
}

// PriorityScheduler is your implementation struct
type PriorityScheduler struct {
	q          *pq
	inProgress *pq
	wg         sync.WaitGroup
	mu         sync.Mutex
	cancel     context.CancelFunc
	ch         chan *PriorityJob
	numWorkers int
	closed     atomic.Bool
	llp        atomic.Int32 // latest lowest priority
}

// NewPriorityScheduler creates a new scheduler instance
func NewPriorityScheduler() WorkScheduler {
	var q, inProgress pq
	heap.Init(&q)
	heap.Init(&inProgress)
	return &PriorityScheduler{
		q:          &q,
		inProgress: &inProgress,
		ch:         make(chan *PriorityJob),
	}
}

func (p *PriorityScheduler) Start(ctx context.Context, numWorkers int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.numWorkers = numWorkers
	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	for i := range numWorkers {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case j := <-p.ch:
					fmt.Printf("Worker %d started working on job %d with priority %d \n", i, j.ID, j.Priority)
					nctx, cl := context.WithCancel(ctx)
					p.mu.Lock()
					j.cancel = cl
					heap.Push(p.inProgress, j)
					p.mu.Unlock()
					close(j.ack)
					if err := j.Work(nctx); err != nil {
						cl()
						fmt.Printf("job %d failed as %q\n", j.ID, err)
						p.mu.Lock()
						p.inProgress.Remove(j.ID)
						p.mu.Unlock()
						break
					}
					fmt.Printf("Worker %d completed working on job %d with priority %d\n", i, j.ID, j.Priority)
					p.mu.Lock()
					p.inProgress.Remove(j.ID)
					p.mu.Unlock()
					cl()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		t := time.NewTicker(5 * time.Millisecond)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				p.mu.Lock()
				lInp := p.inProgress.Len()
				lQ := p.q.Len()
				p.mu.Unlock()
				if lInp < numWorkers && lQ > 0 {
					p.mu.Lock()
					item := heap.Pop(p.q).(*PriorityJob)
					p.mu.Unlock()
					item.ack = make(chan struct{})
					p.ch <- item
					select {
					case <-item.ack:
					case <-ctx.Done():
					}
				}
			}
		}
	}()

}

var SchedulerClosed = errors.New("scheduler closed")

func (p *PriorityScheduler) Submit(job PriorityJob) error {
	if p.closed.Load() || p.numWorkers < 1 {
		fmt.Println("submit shall not work since scheduler closed")
		return SchedulerClosed
	}
	p.mu.Lock()
	lInProgres := p.inProgress.Len()
	p.mu.Unlock()

	if lInProgres < p.numWorkers {
		// workers available, directly send
		job.ack = make(chan struct{})
		p.ch <- &job
		<-job.ack
	} else {
		p.mu.Lock()
		lowest := p.inProgress.peekLowestPriority()
		p.mu.Unlock()
		if lowest != nil && lowest.Priority < job.Priority {
			// higher priority, preempt
			fmt.Println("preemption started", job.ID, lowest.ID)
			lowest.cancel()
			job.ack = make(chan struct{})
			p.ch <- &job
			<-job.ack
			p.mu.Lock()
			heap.Push(p.q, lowest)
			p.mu.Unlock()
		} else {
			p.mu.Lock()
			heap.Push(p.q, &job)
			p.mu.Unlock()
		}
	}
	return nil
}

func (p *PriorityScheduler) Wait() {
	if !p.closed.CompareAndSwap(false, true) {
		return
	}
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	var i int
	for {
		select {
		case <-t.C:
			i++
			p.mu.Lock()
			empty := p.inProgress.Len() == 0
			p.mu.Unlock()
			if i > 5 || empty {
				return
			}
		}
	}
}

func (p *PriorityScheduler) Stop() {
	p.Wait()
	p.cancel()
	p.wg.Wait()
}
