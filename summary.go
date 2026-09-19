package main

// Summary is an aggregate completion-state snapshot of a todo list.
type Summary struct {
	Total int
	Done  int
	Open  int
}

// Summarize counts the given todos and returns their aggregate completion
// state. The input slice is only read, never mutated.
func Summarize(todos []Todo) Summary {
	var s Summary
	for _, t := range todos {
		s.Total++
		if t.Done {
			s.Done++
		}
	}
	s.Open = s.Total - s.Done
	return s
}

// PercentDone returns the share of done todos as a whole percentage,
// rounded half-up, using integer arithmetic only. It returns 0 when there
// are no todos.
func (s Summary) PercentDone() int {
	if s.Total == 0 {
		return 0
	}
	return (s.Done*100*2 + s.Total) / (2 * s.Total)
}
