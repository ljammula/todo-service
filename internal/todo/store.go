package todo

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
)

var ErrNotFound = errors.New("todo not found")

type Item struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type CreateInput struct {
	Title string `json:"title"`
}

type UpdateInput struct {
	Title     *string `json:"title,omitempty"`
	Completed *bool   `json:"completed,omitempty"`
}

type Store struct {
	mu     sync.RWMutex
	nextID atomic.Int64
	items  map[int64]Item
}

func NewStore() *Store {
	s := &Store{items: make(map[int64]Item)}
	s.nextID.Store(1)
	return s
}

func (s *Store) Create(input CreateInput) Item {
	id := s.nextID.Add(1) - 1
	item := Item{ID: id, Title: input.Title, Completed: false}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = item
	return item
}

func (s *Store) List() []Item {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]Item, 0, len(s.items))
	for _, item := range s.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (s *Store) Update(id int64, input UpdateInput) (Item, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	if input.Title != nil {
		item.Title = *input.Title
	}
	if input.Completed != nil {
		item.Completed = *input.Completed
	}
	s.items[id] = item
	return item, nil
}

func (s *Store) Delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}
