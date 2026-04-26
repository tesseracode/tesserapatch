package store

import (
	"errors"
	"reflect"
	"testing"
)

func mkDeps(parents ...string) []Dependency {
	out := make([]Dependency, 0, len(parents))
	for _, p := range parents {
		out = append(out, Dependency{Slug: p, Kind: DependencyKindHard})
	}
	return out
}

func TestDetectCycles_EmptyGraph(t *testing.T) {
	cyc, err := DetectCycles(map[string][]Dependency{})
	if err != nil || cyc != nil {
		t.Fatalf("empty graph: cyc=%v err=%v", cyc, err)
	}
}

func TestDetectCycles_SingleIsolatedNode(t *testing.T) {
	cyc, err := DetectCycles(map[string][]Dependency{"a": nil})
	if err != nil || cyc != nil {
		t.Fatalf("isolated node: cyc=%v err=%v", cyc, err)
	}
}

func TestDetectCycles_SelfEdge(t *testing.T) {
	g := map[string][]Dependency{"a": mkDeps("a")}
	cyc, err := DetectCycles(g)
	if err == nil {
		t.Fatalf("self-edge must be a cycle, got nil err (cyc=%v)", cyc)
	}
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("err must wrap ErrCycle, got %v", err)
	}
	if len(cyc) < 2 || cyc[0] != "a" {
		t.Fatalf("self-cycle path expected to start at a, got %v", cyc)
	}
}

func TestDetectCycles_TwoNode(t *testing.T) {
	g := map[string][]Dependency{
		"a": mkDeps("b"),
		"b": mkDeps("a"),
	}
	cyc, err := DetectCycles(g)
	if err == nil {
		t.Fatal("2-node cycle must be detected")
	}
	if len(cyc) < 3 {
		t.Fatalf("cycle path too short: %v", cyc)
	}
}

func TestDetectCycles_ThreeNode(t *testing.T) {
	g := map[string][]Dependency{
		"a": mkDeps("b"),
		"b": mkDeps("c"),
		"c": mkDeps("a"),
	}
	if _, err := DetectCycles(g); err == nil {
		t.Fatal("3-node cycle must be detected")
	}
}

func TestDetectCycles_LinearAcyclic(t *testing.T) {
	g := map[string][]Dependency{
		"a": mkDeps("b"),
		"b": mkDeps("c"),
		"c": nil,
	}
	cyc, err := DetectCycles(g)
	if err != nil || cyc != nil {
		t.Fatalf("linear acyclic: cyc=%v err=%v", cyc, err)
	}
}

func TestDetectCycles_Diamond(t *testing.T) {
	// a depends on b and c; b and c both depend on d. No cycle.
	g := map[string][]Dependency{
		"a": mkDeps("b", "c"),
		"b": mkDeps("d"),
		"c": mkDeps("d"),
		"d": nil,
	}
	cyc, err := DetectCycles(g)
	if err != nil || cyc != nil {
		t.Fatalf("diamond should be acyclic: cyc=%v err=%v", cyc, err)
	}
}

func TestTopologicalOrder_LinearChain(t *testing.T) {
	g := map[string][]Dependency{
		"a": mkDeps("b"),
		"b": mkDeps("c"),
		"c": nil,
	}
	order, err := TopologicalOrder(g)
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	want := []string{"c", "b", "a"} // parents first
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("topo order: got %v want %v", order, want)
	}
}

func TestTopologicalOrder_Diamond(t *testing.T) {
	g := map[string][]Dependency{
		"a": mkDeps("b", "c"),
		"b": mkDeps("d"),
		"c": mkDeps("d"),
		"d": nil,
	}
	order, err := TopologicalOrder(g)
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	// d must come first; b and c (siblings) must precede a.
	pos := map[string]int{}
	for i, s := range order {
		pos[s] = i
	}
	if pos["d"] != 0 {
		t.Fatalf("d must be first, got %v", order)
	}
	if !(pos["b"] < pos["a"] && pos["c"] < pos["a"]) {
		t.Fatalf("a must come after b and c: %v", order)
	}
	// Sibling tie deterministic by slug: b before c.
	if pos["b"] >= pos["c"] {
		t.Fatalf("sibling tie: b should precede c lexicographically, got %v", order)
	}
}

func TestTopologicalOrder_Deterministic(t *testing.T) {
	g := map[string][]Dependency{
		"alpha":   mkDeps("zeta"),
		"beta":    mkDeps("zeta"),
		"gamma":   mkDeps("zeta"),
		"delta":   mkDeps("zeta"),
		"zeta":    nil,
		"epsilon": nil,
	}
	first, err := TopologicalOrder(g)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 50; i++ {
		got, err := TopologicalOrder(g)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("nondeterministic: first=%v iter%d=%v", first, i, got)
		}
	}
}

func TestTopologicalOrder_CycleErrors(t *testing.T) {
	g := map[string][]Dependency{
		"a": mkDeps("b"),
		"b": mkDeps("a"),
	}
	_, err := TopologicalOrder(g)
	if err == nil || !errors.Is(err, ErrCycle) {
		t.Fatalf("topo with cycle should wrap ErrCycle, got %v", err)
	}
}

func TestTopologicalOrder_EmptyAndSingle(t *testing.T) {
	if order, err := TopologicalOrder(map[string][]Dependency{}); err != nil || len(order) != 0 {
		t.Fatalf("empty: order=%v err=%v", order, err)
	}
	order, err := TopologicalOrder(map[string][]Dependency{"only": nil})
	if err != nil || !reflect.DeepEqual(order, []string{"only"}) {
		t.Fatalf("single: order=%v err=%v", order, err)
	}
}

func TestChildren_Deterministic(t *testing.T) {
	g := map[string][]Dependency{
		"child-c": mkDeps("parent"),
		"child-a": mkDeps("parent"),
		"child-b": mkDeps("parent"),
		"other":   nil,
	}
	got := Children(g, "parent")
	want := []string{"child-a", "child-b", "child-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Children: got %v want %v", got, want)
	}
	if kids := Children(g, "nobody"); kids != nil {
		t.Fatalf("expected nil for unknown parent, got %v", kids)
	}
}
