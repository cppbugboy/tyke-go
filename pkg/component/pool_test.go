package component

import "testing"

func TestObjectPoolAcquireRelease(t *testing.T) {
	pool := NewObjectPool(func() *int {
		v := 42
		return &v
	})

	obj := pool.Acquire()
	if obj == nil {
		t.Fatal("Acquire returned nil")
	}
	if *obj != 42 {
		t.Errorf("initial value = %d, want 42", *obj)
	}

	*obj = 100
	pool.Release(obj)

	obj2 := pool.Acquire()
	if obj2 == nil {
		t.Fatal("second Acquire returned nil")
	}
}

func TestObjectPoolMultipleAcquire(t *testing.T) {
	pool := NewObjectPool(func() *int {
		v := 0
		return &v
	})

	objs := make([]*int, 10)
	for i := range objs {
		objs[i] = pool.Acquire()
		*objs[i] = i
	}
	for _, obj := range objs {
		pool.Release(obj)
	}
}

func TestWorkerPoolSubmit(t *testing.T) {
	wp := NewWorkerPool(4)
	wp.Start()

	results := make(chan int, 100)
	for i := 0; i < 100; i++ {
		i := i
		wp.Submit(func() {
			results <- i
		})
	}
	wp.Stop(true)

	close(results)
	count := 0
	for range results {
		count++
	}
	if count != 100 {
		t.Errorf("executed %d tasks, want 100", count)
	}
}

func TestWorkerPoolStop(t *testing.T) {
	wp := NewWorkerPool(2)
	wp.Start()
	wp.Stop(true)

	err := wp.Submit(func() {})
	if err != ErrPoolStopped {
		t.Errorf("expected ErrPoolStopped, got %v", err)
	}
}
