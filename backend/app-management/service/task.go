package service

import (
	"bytes"
	"sync"
)

type Task struct {
	ID         string
	LogBuffer  *bytes.Buffer
	mu         sync.Mutex
	subscribers []chan string
	isFinished bool
}

type TaskService struct {
	tasks map[string]*Task
	mu    sync.RWMutex
}

func NewTaskService() *TaskService {
	return &TaskService{
		tasks: make(map[string]*Task),
	}
}

func (s *TaskService) GetOrCreate(id string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task, ok := s.tasks[id]; ok {
		return task
	}

	task := &Task{
		ID:         id,
		LogBuffer:  new(bytes.Buffer),
		subscribers: make([]chan string, 0),
	}
	s.tasks[id] = task
	return task
}

func (s *TaskService) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tasks, id)
}

func (t *Task) Write(p []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	n, err = t.LogBuffer.Write(p)
	
	// Broadcast to subscribers
	line := string(p)
	for _, sub := range t.subscribers {
		select {
		case sub <- line:
		default:
			// Buffer full or subscriber slow, skip
		}
	}
	
	return n, err
}

func (t *Task) Subscribe() (chan string, func()) {
	t.mu.Lock()
	defer t.mu.Unlock()

	ch := make(chan string, 100)
	
	// Send existing buffer first
	if t.LogBuffer.Len() > 0 {
		ch <- t.LogBuffer.String()
	}

	t.subscribers = append(t.subscribers, ch)
	
	cleanup := func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		for i, sub := range t.subscribers {
			if sub == ch {
				t.subscribers = append(t.subscribers[:i], t.subscribers[i+1:]...)
				close(ch)
				break
			}
		}
	}

	return ch, cleanup
}

func (t *Task) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.isFinished = true
	for _, sub := range t.subscribers {
		close(sub)
	}
	t.subscribers = nil
}

var MyTaskService = NewTaskService()
