package main

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
					j.cancel = cl
					heap.Push(p.inProgress, j)
					close(j.ack)
					if err := j.Work(nctx); err != nil {
						cl()
						fmt.Printf("job %d failed as %q\n", j.ID, err)
						p.inProgress.Remove(j.ID)
						break
					}
					fmt.Printf("Worker %d completed working on job %d with priority %d\n", i, j.ID, j.Priority)
					p.inProgress.Remove(j.ID)
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
				if p.inProgress.Len() < numWorkers && p.q.Len() > 0 {
					item := heap.Pop(p.q).(*PriorityJob)
					item.ack = make(chan struct{})
					p.ch <- item
					select {
					case <-item.ack:
					case <-ctx.Done():
					}
				}
				p.mu.Unlock()
			}
		}
	}()

}

var SchedulerClosed = errors.New("scheduler closed")

func (p *PriorityScheduler) Submit(job PriorityJob) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed.Load() || p.numWorkers < 1 {
		fmt.Println("submit shall not work since scheduler closed")
		return SchedulerClosed
	}

	if p.inProgress.Len() < p.numWorkers {
		// workers available, directly send
		job.ack = make(chan struct{})
		p.ch <- &job
		<-job.ack
	} else {
		if lowest := p.inProgress.peekLowestPriority(); lowest != nil && lowest.Priority < job.Priority {
			// higher priority, preempt
			fmt.Println("preemption started", job.ID, lowest.ID)
			lowest.cancel()
			job.ack = make(chan struct{})
			p.ch <- &job
			<-job.ack
			heap.Push(p.q, lowest)
		} else {
			heap.Push(p.q, &job)
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
			if i > 5 || p.inProgress.Len() == 0 {
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

func main() {
	fmt.Println("=== Test 1: Basic Priority Ordering ===")
	testBasicPriority()

	fmt.Println("\n=== Test 2: Preemption Test ===")
	testPreemption()

	fmt.Println("\n=== Test 3: Multiple Workers ===")
	testMultipleWorkers()

	fmt.Println("\n=== Test 4: Context Cancellation ===")
	testContextCancellation()

	fmt.Println("\n=== Test 5: Concurrent Submissions ===")
	testConcurrentSubmissions()

	fmt.Println("\n=== All tests completed ===")
}

func testBasicPriority() {
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := []int{}

	// Submit jobs with different priorities (higher number = higher priority)
	jobs := []PriorityJob{
		{ID: 1, Priority: 1, Work: func(cctx context.Context) error {
			mu.Lock()
			executed = append(executed, 1)
			mu.Unlock()
			t := time.NewTimer(20 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		}},
		{ID: 2, Priority: 5, Work: func(cctx context.Context) error {
			mu.Lock()
			executed = append(executed, 2)
			mu.Unlock()
			t := time.NewTimer(20 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		}},
		{ID: 3, Priority: 3, Work: func(cctx context.Context) error {
			mu.Lock()
			executed = append(executed, 3)
			mu.Unlock()
			t := time.NewTimer(20 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		}},
	}

	scheduler.Start(ctx, 1)

	for _, job := range jobs {
		scheduler.Submit(job)
	}

	scheduler.Wait()
	scheduler.Stop()

	fmt.Printf("Execution order: %v (expected high priority first: [2, 3, 1])\n", executed)
}

func testPreemption() {
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := []int{}
	jobStarted := make(chan int, 10)

	// PriorityJob 1: Low priority, long running
	job1 := PriorityJob{
		ID:       1,
		Priority: 1,
		Work: func(cctx context.Context) error {
			jobStarted <- 1
			mu.Lock()
			executed = append(executed, 1)
			mu.Unlock()
			t := time.NewTimer(100 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		},
	}

	// PriorityJob 2: High priority, should preempt
	job2 := PriorityJob{
		ID:       2,
		Priority: 10,
		Work: func(cctx context.Context) error {
			jobStarted <- 2
			mu.Lock()
			executed = append(executed, 2)
			mu.Unlock()
			t := time.NewTimer(10 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		},
	}

	// PriorityJob 3: Medium priority
	job3 := PriorityJob{
		ID:       3,
		Priority: 5,
		Work: func(cctx context.Context) error {
			jobStarted <- 3
			mu.Lock()
			executed = append(executed, 3)
			mu.Unlock()
			t := time.NewTimer(10 * time.Millisecond)
			defer t.Stop()
			select {
			case <-cctx.Done():
			case <-t.C:
			}
			return cctx.Err()
		},
	}

	scheduler.Start(ctx, 1)

	scheduler.Submit(job1)
	<-jobStarted // Wait for job1 to start

	time.Sleep(20 * time.Millisecond) // Let it run a bit

	scheduler.Submit(job2) // High priority should preempt
	scheduler.Submit(job3)

	scheduler.Wait()
	scheduler.Stop()

	mu.Lock()
	finalOrder := make([]int, len(executed))
	copy(finalOrder, executed)
	mu.Unlock()

	fmt.Printf("Execution order: %v\n", finalOrder)
	fmt.Printf("Expected: PriorityJob 1 starts, gets preempted by PriorityJob 2 (priority 10), then PriorityJob 3 (priority 5), then PriorityJob 1 completes\n")
	fmt.Printf("Note: PriorityJob 1 should appear twice in execution if preempted and re-queued\n")
}

func testMultipleWorkers() {
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := []int{}

	numJobs := 10
	jobs := make([]PriorityJob, numJobs)

	for i := 0; i < numJobs; i++ {
		id := i + 1
		jobs[i] = PriorityJob{
			ID:       id,
			Priority: id, // Priority equals ID
			Work: func(cctx context.Context) error {
				mu.Lock()
				executed = append(executed, id)
				mu.Unlock()
				t := time.NewTimer(200 * time.Millisecond)
				defer t.Stop()
				select {
				case <-cctx.Done():
				case <-t.C:
				}
				return cctx.Err()
			},
		}
	}

	scheduler.Start(ctx, 3) // 3 workers

	for _, job := range jobs {
		scheduler.Submit(job)
	}

	scheduler.Wait()
	scheduler.Stop()

	fmt.Printf("Executed %d jobs with 3 workers\n", len(executed))
	fmt.Printf("Execution order: %v\n", executed)
	fmt.Printf("Higher priority jobs should generally execute first\n")
}

func testContextCancellation() {
	scheduler := NewPriorityScheduler()
	ctx, cancel := context.WithCancel(context.Background())

	var mu sync.Mutex
	executed := []int{}

	scheduler.Start(ctx, 1)
	for i := 5; i > 0; i-- {
		id := i
		job := PriorityJob{
			ID:       id,
			Priority: id,
			Work: func(cctx context.Context) error {
				mu.Lock()
				executed = append(executed, id)
				mu.Unlock()
				t := time.NewTimer(50 * time.Millisecond)
				defer t.Stop()
				select {
				case <-cctx.Done():
				case <-t.C:
				}
				return cctx.Err()
			},
		}
		scheduler.Submit(job)
	}

	time.Sleep(80 * time.Millisecond)
	cancel() // Cancel context

	time.Sleep(50 * time.Millisecond)
	scheduler.Stop()

	mu.Lock()
	fmt.Printf("Executed %d jobs before cancellation (expected < 5): %v\n", len(executed), executed)
	mu.Unlock()
}

func testConcurrentSubmissions() {
	scheduler := NewPriorityScheduler()
	ctx := context.Background()

	var mu sync.Mutex
	executed := make(map[int]bool)

	scheduler.Start(ctx, 2)

	var wg sync.WaitGroup
	numGoroutines := 5
	jobsPerGoroutine := 4

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < jobsPerGoroutine; j++ {
				jobID := goroutineID*jobsPerGoroutine + j
				job := PriorityJob{
					ID:       jobID,
					Priority: jobID % 10,
					Work: func(cctx context.Context) error {
						mu.Lock()
						executed[jobID] = true
						mu.Unlock()
						t := time.NewTimer(5 * time.Millisecond)
						defer t.Stop()
						select {
						case <-cctx.Done():
						case <-t.C:
						}
						return cctx.Err()
					},
				}
				scheduler.Submit(job)
			}
		}(g)
	}

	wg.Wait()
	scheduler.Wait()
	scheduler.Stop()

	mu.Lock()
	fmt.Printf("Executed %d jobs from concurrent submissions (expected %d)\n",
		len(executed), numGoroutines*jobsPerGoroutine)
	mu.Unlock()
}
