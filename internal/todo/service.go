package todo

import (
	"fmt"
	"strings"
)

type Service struct {
	store *Store
}

type ListResult struct {
	Items []Item `json:"items"`
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) Create(input CreateInput) (Item, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return Item{}, fmt.Errorf("title is required")
	}
	return s.store.Create(input), nil
}

func (s *Service) ListItems() []Item {
	return s.store.List()
}

func (s *Service) List() ListResult {
	return ListResult{Items: s.store.List()}
}

func (s *Service) Update(id int64, input UpdateInput) (Item, error) {
	if input.Title != nil {
		title := strings.TrimSpace(*input.Title)
		if title == "" {
			return Item{}, fmt.Errorf("title cannot be empty")
		}
		input.Title = &title
	}
	return s.store.Update(id, input)
}

func (s *Service) Delete(id int64) error {
	return s.store.Delete(id)
}
