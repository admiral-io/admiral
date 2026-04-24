package dag

import (
	"fmt"
	"sort"
	"strings"
)

// Graph is a directed acyclic graph of string-named nodes.
type Graph struct {
	nodes map[string]struct{}
	edges map[string]map[string]struct{} // from → set(to)
}

// New creates an empty graph.
func New() *Graph {
	return &Graph{
		nodes: make(map[string]struct{}),
		edges: make(map[string]map[string]struct{}),
	}
}

// AddNode adds a node to the graph. Duplicate adds are no-ops.
func (g *Graph) AddNode(name string) {
	g.nodes[name] = struct{}{}
}

// AddEdge adds a dependency edge: `from` depends on `to`.
// Both nodes are implicitly added.
func (g *Graph) AddEdge(from, to string) {
	g.AddNode(from)
	g.AddNode(to)
	if g.edges[from] == nil {
		g.edges[from] = make(map[string]struct{})
	}
	g.edges[from][to] = struct{}{}
}

// Dependencies returns the direct dependencies of a node (what it depends on).
func (g *Graph) Dependencies(name string) []string {
	deps := g.edges[name]
	out := make([]string, 0, len(deps))
	for d := range deps {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// TopoSort returns nodes grouped into phases using Kahn's algorithm.
// Phase 0 contains nodes with no dependencies. Phase N contains nodes
// whose dependencies are all satisfied by phases 0..N-1.
// Returns an error if the graph contains a cycle.
func (g *Graph) TopoSort() ([][]string, error) {
	// Build in-degree map (count of edges pointing into each node).
	inDegree := make(map[string]int, len(g.nodes))
	for n := range g.nodes {
		inDegree[n] = 0
	}
	for from, tos := range g.edges {
		// Only count edges where both endpoints are known nodes.
		for to := range tos {
			if _, ok := g.nodes[to]; !ok {
				continue
			}
			_ = from
			inDegree[to] += 0 // ensure to exists
		}
	}
	// Recount properly: for edge from→to, "from" depends on "to",
	// so "from" has in-degree incremented (it is blocked by "to").
	// Wait — our edge semantics: AddEdge(from, to) means "from depends on to".
	// In Kahn's algorithm, we process nodes with no incoming edges first.
	// "from depends on to" means there's an edge to→from in the execution DAG.
	// So in-degree of "from" increases by 1 for each dependency.
	for n := range g.nodes {
		inDegree[n] = len(g.edges[n])
	}

	// Seed with nodes that have no dependencies.
	var queue []string
	for n, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, n)
		}
	}
	sort.Strings(queue)

	// Build reverse edges: to → set(from). "to" blocks "from".
	reverse := make(map[string]map[string]struct{})
	for from, tos := range g.edges {
		for to := range tos {
			if reverse[to] == nil {
				reverse[to] = make(map[string]struct{})
			}
			reverse[to][from] = struct{}{}
		}
	}

	var phases [][]string
	processed := 0

	for len(queue) > 0 {
		phase := queue
		queue = nil
		processed += len(phase)
		phases = append(phases, phase)

		var next []string
		for _, n := range phase {
			for blocked := range reverse[n] {
				inDegree[blocked]--
				if inDegree[blocked] == 0 {
					next = append(next, blocked)
				}
			}
		}
		sort.Strings(next)
		queue = next
	}

	if processed != len(g.nodes) {
		cycle := make([]string, 0)
		for n, deg := range inDegree {
			if deg > 0 {
				cycle = append(cycle, n)
			}
		}
		sort.Strings(cycle)
		return nil, fmt.Errorf("dependency cycle detected among: %s", strings.Join(cycle, ", "))
	}

	return phases, nil
}

// ReverseTopoSort returns nodes grouped into phases in reverse topological
// order. This is the execution order for destroy operations: dependents are
// processed before their dependencies.
func (g *Graph) ReverseTopoSort() ([][]string, error) {
	phases, err := g.TopoSort()
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(phases)-1; i < j; i, j = i+1, j-1 {
		phases[i], phases[j] = phases[j], phases[i]
	}
	return phases, nil
}
