// DAG primitives over the feature dependency graph.
//
// Edge convention: child → parent. A child's Dependency entries enumerate
// the *parents* it depends on. A topological order over this graph yields
// parents before their children — this is the order reconcile and apply
// must traverse so a child only runs after every parent it depends on.
//
// The functions in this file are pure: they accept a graph
// (map[slug][]Dependency) and return either an answer or an error. They
// do not touch the Store or the filesystem. This isolates DAG correctness
// from disk concerns — see ADR-011 D2 for the choice of DFS for cycle
// detection and Kahn for traversal.
//
// Soft vs hard deps: at the topology layer both edges contribute equally
// to ordering. The hard/soft distinction governs apply-gating (M14.2) and
// label composition (M14.3), not topology.
//
// IMPORTANT for downstream callers (M14.3+): when deriving DAG-aware
// reconcile labels, read FeatureStatus.Reconcile.Outcome — never read
// artifacts/reconcile-session.json. The session artifact is an audit
// record of one RunReconcile invocation; status.json is the source of
// current truth post-accept (see ADR-010 D5).

package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrCycle is returned by DAG operations when the graph contains a cycle.
// Errors wrap this sentinel; callers can distinguish with errors.Is.
var ErrCycle = errors.New("dependency graph contains a cycle")

// DetectCycles returns a cycle path (slug list, head==tail) when one is
// found, or a nil slice when the graph is acyclic. A self-edge counts as a
// cycle. Iteration order over the input map is normalised so the reported
// cycle is deterministic across runs.
//
// The error wraps ErrCycle when a cycle is detected; on a clean graph
// returns (nil, nil).
func DetectCycles(features map[string][]Dependency) ([]string, error) {
	roots := sortedSlugs(features)

	const (
		white = 0 // unvisited
		gray  = 1 // on current DFS stack
		black = 2 // fully explored
	)
	color := make(map[string]int, len(features))
	parent := make(map[string]string, len(features))

	var cycle []string
	var visit func(node string) bool
	visit = func(node string) bool {
		color[node] = gray
		// Visit successors (parents in dep graph) in deterministic order.
		nexts := sortedDepSlugs(features[node])
		for _, dep := range nexts {
			switch color[dep] {
			case gray:
				// Found back-edge node→dep. Reconstruct cycle path:
				// start at dep, walk parent[] via node, append node, append dep.
				path := []string{dep}
				cur := node
				for cur != dep && cur != "" {
					path = append(path, cur)
					cur = parent[cur]
				}
				path = append(path, dep)
				// Reverse so it reads dep → ... → node → dep (forward edges).
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				cycle = path
				return true
			case white:
				parent[dep] = node
				if visit(dep) {
					return true
				}
			}
		}
		color[node] = black
		return false
	}

	for _, n := range roots {
		if color[n] == white {
			if visit(n) {
				return cycle, fmt.Errorf("%w: %s", ErrCycle, strings.Join(cycle, " -> "))
			}
		}
	}
	return nil, nil
}

// TopologicalOrder returns a deterministic topological ordering of the
// graph using Kahn's algorithm. Parents appear before children. Sibling
// ties (multiple slugs ready at the same step) are broken lexicographically
// by slug for stable output.
//
// Returns ErrCycle (wrapped) when the graph contains a cycle. Callers that
// want the cycle path should call DetectCycles afterwards.
func TopologicalOrder(features map[string][]Dependency) ([]string, error) {
	// In Kahn we work in the *forward* direction: produce parents first.
	// In our edge convention child→parent, so the "in-degree" from Kahn's
	// perspective (parents-first) is the count of children pointing at me.
	// Easiest: build a reversed adjacency and a "remaining children" count.
	//
	// Equivalent restatement: we want nodes with zero unsatisfied parents
	// to come out first. So in-degree(n) = number of parents of n still
	// unprocessed = len(features[n]) restricted to parents that exist in
	// the graph. (Dangling parents are validation errors handled elsewhere;
	// here we only count parents present in the map so topology works on
	// the closed subgraph.)

	indeg := make(map[string]int, len(features))
	children := make(map[string][]string, len(features))
	for slug := range features {
		indeg[slug] = 0
	}
	for slug, deps := range features {
		seen := make(map[string]struct{}, len(deps))
		for _, d := range deps {
			if _, ok := features[d.Slug]; !ok {
				continue // skip dangling parents in topology
			}
			if _, dup := seen[d.Slug]; dup {
				continue
			}
			seen[d.Slug] = struct{}{}
			indeg[slug]++
			children[d.Slug] = append(children[d.Slug], slug)
		}
	}

	// ready queue (always sorted to break ties deterministically).
	var ready []string
	for _, slug := range sortedSlugs(features) {
		if indeg[slug] == 0 {
			ready = append(ready, slug)
		}
	}

	out := make([]string, 0, len(features))
	for len(ready) > 0 {
		// Pop lexicographically smallest.
		sort.Strings(ready)
		head := ready[0]
		ready = ready[1:]
		out = append(out, head)
		// Sort children for deterministic edge relaxation order.
		kids := append([]string(nil), children[head]...)
		sort.Strings(kids)
		for _, c := range kids {
			indeg[c]--
			if indeg[c] == 0 {
				ready = append(ready, c)
			}
		}
	}

	if len(out) != len(features) {
		return nil, fmt.Errorf("%w: topological order incomplete (%d of %d nodes scheduled)", ErrCycle, len(out), len(features))
	}
	return out, nil
}

// Children returns the direct dependents of parent — slugs whose
// DependsOn lists parent. Result is sorted ascending by slug.
func Children(features map[string][]Dependency, parent string) []string {
	var kids []string
	for slug, deps := range features {
		for _, d := range deps {
			if d.Slug == parent {
				kids = append(kids, slug)
				break
			}
		}
	}
	sort.Strings(kids)
	return kids
}

// sortedSlugs returns all keys of features sorted ascending.
func sortedSlugs(features map[string][]Dependency) []string {
	out := make([]string, 0, len(features))
	for k := range features {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedDepSlugs returns the dep slugs of deps sorted ascending,
// deduplicated. The returned slice is independent of deps.
func sortedDepSlugs(deps []Dependency) []string {
	if len(deps) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(deps))
	out := make([]string, 0, len(deps))
	for _, d := range deps {
		if _, dup := seen[d.Slug]; dup {
			continue
		}
		seen[d.Slug] = struct{}{}
		out = append(out, d.Slug)
	}
	sort.Strings(out)
	return out
}
