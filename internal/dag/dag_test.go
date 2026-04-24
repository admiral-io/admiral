package dag

import (
	"testing"
)

func TestEmptyGraph(t *testing.T) {
	g := New()
	phases, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 0 {
		t.Fatalf("expected 0 phases, got %d", len(phases))
	}
}

func TestSingleNode(t *testing.T) {
	g := New()
	g.AddNode("a")
	phases, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 1 || len(phases[0]) != 1 || phases[0][0] != "a" {
		t.Fatalf("expected [[a]], got %v", phases)
	}
}

func TestParallelNodes(t *testing.T) {
	g := New()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("c")
	phases, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 1 {
		t.Fatalf("expected 1 phase, got %d: %v", len(phases), phases)
	}
	if len(phases[0]) != 3 {
		t.Fatalf("expected 3 nodes in phase 0, got %d", len(phases[0]))
	}
}

func TestLinearChain(t *testing.T) {
	// c depends on b, b depends on a → 3 phases
	g := New()
	g.AddEdge("b", "a") // b depends on a
	g.AddEdge("c", "b") // c depends on b
	phases, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d: %v", len(phases), phases)
	}
	assertPhase(t, phases[0], []string{"a"})
	assertPhase(t, phases[1], []string{"b"})
	assertPhase(t, phases[2], []string{"c"})
}

func TestDiamond(t *testing.T) {
	// app depends on db and cache; db and cache depend on vpc
	g := New()
	g.AddEdge("db", "vpc")
	g.AddEdge("cache", "vpc")
	g.AddEdge("app", "db")
	g.AddEdge("app", "cache")
	phases, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d: %v", len(phases), phases)
	}
	assertPhase(t, phases[0], []string{"vpc"})
	assertPhase(t, phases[1], []string{"cache", "db"})
	assertPhase(t, phases[2], []string{"app"})
}

func TestMixedParallelAndSerial(t *testing.T) {
	// monitoring has no deps; db depends on vpc; app depends on db
	g := New()
	g.AddNode("monitoring")
	g.AddNode("vpc")
	g.AddEdge("db", "vpc")
	g.AddEdge("app", "db")
	phases, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d: %v", len(phases), phases)
	}
	assertPhase(t, phases[0], []string{"monitoring", "vpc"})
	assertPhase(t, phases[1], []string{"db"})
	assertPhase(t, phases[2], []string{"app"})
}

func TestCycleDetected(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	_, err := g.TopoSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestSelfReference(t *testing.T) {
	g := New()
	g.AddEdge("a", "a")
	_, err := g.TopoSort()
	if err == nil {
		t.Fatal("expected cycle error for self-reference, got nil")
	}
}

func TestThreeNodeCycle(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "a")
	_, err := g.TopoSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestDependencies(t *testing.T) {
	g := New()
	g.AddEdge("app", "db")
	g.AddEdge("app", "cache")
	deps := g.Dependencies("app")
	assertPhase(t, deps, []string{"cache", "db"})
}

func TestReverseTopoSort(t *testing.T) {
	// Diamond: app depends on db and cache; db and cache depend on vpc
	// Normal:  [[vpc], [cache, db], [app]]
	// Reverse: [[app], [cache, db], [vpc]]
	g := New()
	g.AddEdge("db", "vpc")
	g.AddEdge("cache", "vpc")
	g.AddEdge("app", "db")
	g.AddEdge("app", "cache")
	phases, err := g.ReverseTopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d: %v", len(phases), phases)
	}
	assertPhase(t, phases[0], []string{"app"})
	assertPhase(t, phases[1], []string{"cache", "db"})
	assertPhase(t, phases[2], []string{"vpc"})
}

func TestReverseTopoSortLinear(t *testing.T) {
	// c depends on b, b depends on a
	// Normal:  [[a], [b], [c]]
	// Reverse: [[c], [b], [a]]
	g := New()
	g.AddEdge("b", "a")
	g.AddEdge("c", "b")
	phases, err := g.ReverseTopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(phases) != 3 {
		t.Fatalf("expected 3 phases, got %d: %v", len(phases), phases)
	}
	assertPhase(t, phases[0], []string{"c"})
	assertPhase(t, phases[1], []string{"b"})
	assertPhase(t, phases[2], []string{"a"})
}

func TestReverseTopoSortCycle(t *testing.T) {
	g := New()
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	_, err := g.ReverseTopoSort()
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func assertPhase(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("phase length mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("phase[%d] = %q, want %q (full: got %v, want %v)", i, got[i], want[i], got, want)
		}
	}
}
