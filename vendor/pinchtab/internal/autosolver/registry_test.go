package autosolver

import (
	"context"
	"testing"
)

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	s := &mockSolver{name: "test", priority: 10}
	if err := r.Register(s); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Duplicate registration should fail.
	if err := r.Register(s); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_MustRegister_Panics(t *testing.T) {
	r := NewRegistry()

	s := &mockSolver{name: "test", priority: 10}
	r.MustRegister(s)

	defer func() {
		if rv := recover(); rv == nil {
			t.Error("expected panic for duplicate MustRegister")
		}
	}()
	r.MustRegister(s)
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockSolver{name: "alpha", priority: 10})

	s, ok := r.Get("alpha")
	if !ok || s == nil {
		t.Fatal("expected solver to be found")
	}
	if s.Name() != "alpha" {
		t.Errorf("expected name 'alpha', got %q", s.Name())
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("expected false for unknown solver")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockSolver{name: "temp", priority: 10})
	r.Unregister("temp")

	_, ok := r.Get("temp")
	if ok {
		t.Error("expected solver to be gone after Unregister")
	}

	names := r.Names()
	for _, n := range names {
		if n == "temp" {
			t.Error("expected 'temp' removed from Names()")
		}
	}
}

func TestRegistry_PriorityOrder(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockSolver{name: "low", priority: 100})
	r.MustRegister(&mockSolver{name: "high", priority: 1})
	r.MustRegister(&mockSolver{name: "mid", priority: 50})

	names := r.Names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "high" {
		t.Errorf("expected first='high', got %q", names[0])
	}
	if names[1] != "mid" {
		t.Errorf("expected second='mid', got %q", names[1])
	}
	if names[2] != "low" {
		t.Errorf("expected third='low', got %q", names[2])
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockSolver{name: "b", priority: 20})
	r.MustRegister(&mockSolver{name: "a", priority: 10})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 solvers, got %d", len(all))
	}
	if all[0].Name() != "a" {
		t.Errorf("expected first solver 'a', got %q", all[0].Name())
	}
	if all[1].Name() != "b" {
		t.Errorf("expected second solver 'b', got %q", all[1].Name())
	}
}

func TestRegistry_MatchingSolvers(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(&mockSolver{name: "handles", priority: 10, canHandle: true})
	r.MustRegister(&mockSolver{name: "skips", priority: 5, canHandle: false})

	page := &mockPage{title: "test"}
	matched := r.MatchingSolvers(context.Background(), page)

	if len(matched) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matched))
	}
	if matched[0].Name() != "handles" {
		t.Errorf("expected 'handles', got %q", matched[0].Name())
	}
}

func TestRegistry_Empty(t *testing.T) {
	r := NewRegistry()

	names := r.Names()
	if len(names) != 0 {
		t.Errorf("expected empty names, got %v", names)
	}

	all := r.All()
	if len(all) != 0 {
		t.Errorf("expected empty all, got %d", len(all))
	}

	matched := r.MatchingSolvers(context.Background(), &mockPage{})
	if len(matched) != 0 {
		t.Errorf("expected empty matches, got %d", len(matched))
	}
}
