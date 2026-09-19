package main

import "testing"

func TestSummarizeAndPercentDone(t *testing.T) {
	mkTodos := func(done, total int) []Todo {
		var ts []Todo
		for i := 0; i < total; i++ {
			ts = append(ts, Todo{ID: "x", Title: "t", Done: i < done})
		}
		return ts
	}

	cases := []struct {
		name    string
		todos   []Todo
		want    Summary
		wantPct int
	}{
		{name: "empty", todos: []Todo{}, want: Summary{0, 0, 0}, wantPct: 0},
		{name: "all done", todos: mkTodos(5, 5), want: Summary{5, 5, 0}, wantPct: 100},
		{name: "none done", todos: mkTodos(0, 5), want: Summary{5, 0, 5}, wantPct: 0},
		{name: "1 of 3", todos: mkTodos(1, 3), want: Summary{3, 1, 2}, wantPct: 33},
		{name: "2 of 3", todos: mkTodos(2, 3), want: Summary{3, 2, 1}, wantPct: 67},
		{name: "1 of 8", todos: mkTodos(1, 8), want: Summary{8, 1, 7}, wantPct: 13},
		{name: "3 of 8", todos: mkTodos(3, 8), want: Summary{8, 3, 5}, wantPct: 38},
		{name: "1 of 2", todos: mkTodos(1, 2), want: Summary{2, 1, 1}, wantPct: 50},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := Summarize(c.todos)
			if s != c.want {
				t.Errorf("Summarize() = %+v, want %+v", s, c.want)
			}
			if got := s.PercentDone(); got != c.wantPct {
				t.Errorf("PercentDone() = %d, want %d", got, c.wantPct)
			}
		})
	}
}
