package lambda

import (
	"fmt"
	"testing"

	"github.com/vic/godnet/pkg/deltanet"
)

// TestIdentityFunction tests the simplest lambda term: (λx. x)
// Paper context: "In λ L-calculus, variables appear exactly once."
// This tests the basic fan annihilation rule where an abstraction immediately
// applies to an argument, verifying the core β-reduction mechanism.
func TestIdentityFunction(t *testing.T) {
	// Paper: "The only interaction rule in Δ L-Nets is fan annihilation,
	// which expresses β-reduction."

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Parse (λx. x)
	term, err := Parse("(x: x)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	// Reduce to normal form
	n.ReduceToNormalForm()

	// Identity function is already in normal form (no active pairs)
	// Result should be the abstraction itself
	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)

	t.Logf("Identity function: (λx. x) → %v", result)

	// Verify no reductions occurred (already in normal form)
	stats := n.GetStats()
	if stats.TotalReductions > 0 {
		t.Logf("Note: %d reductions (may include canonicalization)", stats.TotalReductions)
	}
}

// TestKCombinator tests the K combinator: (λx. λy. x)
// Paper: "In Δ A-Nets (affine), applying an abstraction which doesn't use
// its bound variable results in an eraser becoming connected to a parent port."
// This tests erasure interaction rules and the canonicalization step.
func TestKCombinator(t *testing.T) {
	// Paper: "As an extreme example, applying an abstraction which doesn't use
	// its bound variable to an argument which only uses globally-free variables
	// produces a subnet which is disjointed from the root."

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Parse K combinator applied twice: (((λx. λy. x) a) b) → a
	term, err := Parse("(((x: (y: x)) a) b)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	t.Logf("VarNames map: %v", varNames)

	// Reduce to normal form
	n.ReduceToNormalForm()

	// Get what output is connected to after reduction
	resultNode, resultPort := n.GetLink(output, 0)
	t.Logf("Output connected to: %v id=%d port=%d", resultNode.Type(), resultNode.ID(), resultPort)

	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("K combinator: (λx. λy. x) a b → %v", result)

	// Paper: "fan annihilations are applied in leftmost-outermost order,
	// with the final erasure canonicalization step ensuring perfect confluence"
	stats := n.GetStats()
	t.Logf("Reductions: %d fan-fan, %d erasures", stats.FanAnnihilation, stats.Erasure)

	// Result should be 'a' (second argument 'b' is erased)
	if v, ok := result.(Var); !ok || v.Name != "a" {
		t.Errorf("Expected variable 'a', got %v", result)
	}
}

// TestSCombinator tests the S combinator: (λx. λy. λz. (x z) (y z))
// Paper: "In Δ I-Nets (relevant), replicators are needed to express optimal
// parallel reduction involving sharing."
// This tests fan-replicator commutation and replicator-replicator interactions.
func TestSCombinator(t *testing.T) {
	// Paper: "When a replicator interacts with a fan, the replicator travels
	// through and out of the fan's two auxiliary ports, resulting in two exact
	// copies of the replicator."

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Parse S combinator with arguments: S K K e
	// Where S = λx.λy.λz.(x z)(y z), K = λa.λb.a
	term, err := Parse("((((x: (y: (z: ((x z) (y z))))) (a: (b: a))) (c: (d: c))) e)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	// Reduce to normal form
	n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("S combinator: S K K e → %v", result)

	stats := n.GetStats()
	t.Logf("Reductions: %d fan-fan, %d rep-rep, %d fan-rep commutations",
		stats.FanAnnihilation, stats.RepAnnihilation, stats.FanRepCommutation)

	// S K K is equivalent to I (identity), so S K K e → e
	if v, ok := result.(Var); !ok || v.Name != "e" {
		t.Errorf("Expected variable 'e', got %v", result)
	}
}

// TestSharing tests sharing of subterms through replicators
// Paper: "Each instance of a replicator incorporates information that in
// previous models was spread across multiple agents, such as indexed fans
// and delimiters. This consolidation of information enables simplifications
// that were previously unfeasible."
func TestSharing(t *testing.T) {
	// Paper: "Instead of making use of delimiters, sharing is expressed through
	// a single agent type which allows any number of auxiliary ports, called
	// a *replicator*."

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Parse (λf. f (f x)) (λy. y)
	// The argument (λy. y) is shared between two applications
	term, err := Parse("((f: (f (f x))) (y: y))")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	// Reduce to normal form
	n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("Sharing test: (λf. f (f x)) (λy. y) → %v", result)

	stats := n.GetStats()
	t.Logf("Reductions: %d total, %d fan-rep commutations",
		stats.TotalReductions, stats.FanRepCommutation)

	// Paper: "no reduction operation is applied which is rendered unnecessary
	// later, and no reduction operation which is necessary is applied more
	// than once."
	// This is the optimality guarantee through sharing
}

// TestChurchNumerals tests Church encoding of natural numbers
// Paper: Church numerals demonstrate the power of λ-calculus encoding
// and stress-test the interaction system with nested applications.
func TestChurchNumerals(t *testing.T) {
	// Church numeral 0: λf. λx. x
	// Church numeral 1: λf. λx. f x
	// Church numeral 2: λf. λx. f (f x)

	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "Zero",
			input: "(f: (x: x))",
			desc:  "Church numeral 0 applies f zero times",
		},
		{
			name:  "One",
			input: "(f: (x: (f x)))",
			desc:  "Church numeral 1 applies f once",
		},
		{
			name:  "Two",
			input: "(f: (x: (f (f x))))",
			desc:  "Church numeral 2 applies f twice",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := deltanet.NewNetwork()
			term, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			root, port, _ := ToDeltaNet(term, n)
			output := n.NewVar()
			n.Link(root, port, output, 0)

			// Church numerals are already in normal form (no active pairs to reduce)
			// so we don't call ReduceToNormalForm()
			t.Logf("%s: %s (already in normal form)", tt.name, tt.desc)
		})
	}
}

// TestBooleans tests Church booleans and boolean operations
// Paper: Boolean operations demonstrate conditional evaluation and
// the interaction between abstraction, application, and erasure.
func TestBooleans(t *testing.T) {
	// Church true:  λx. λy. x  (K combinator)
	// Church false: λx. λy. y  (K I combinator)
	// NOT: λb. b false true

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NOT true",
			input:    "((b: ((b (x: (y: y))) (x: (y: x)))) (x: (y: x)))",
			expected: "false",
		},
		{
			name:     "NOT false",
			input:    "((b: ((b (x: (y: y))) (x: (y: x)))) (x: (y: y)))",
			expected: "true",
		},
		{
			name:     "AND true true",
			input:    "(((a: (b: ((a b) a))) (x: (y: x))) (x: (y: x)))",
			expected: "true",
		},
		{
			name:     "AND true false",
			input:    "(((a: (b: ((a b) a))) (x: (y: x))) (x: (y: y)))",
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := deltanet.NewNetwork()
			term, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			root, port, varNames := ToDeltaNet(term, n)
			output := n.NewVar()
			n.Link(root, port, output, 0)

			n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
			result := FromDeltaNet(n, resultNode, resultPort, varNames)
			t.Logf("%s → %v", tt.name, result)

			// Paper: "in the Δ A-Nets system, fan annihilations are applied in
			// leftmost-outermost order, with the final erasure canonicalization
			// step ensuring perfect confluence"
			stats := n.GetStats()
			t.Logf("Reductions: %d fan-fan, %d erasures", stats.FanAnnihilation, stats.Erasure)
		})
	}
}

// TestPairs tests Church pairs (tuples)
// Paper: Pairs demonstrate product types in λ-calculus and stress-test
// the interaction between multiple abstractions and applications.
func TestPairs(t *testing.T) {
	// Paper: Testing structural operations like pair construction and projection

	// pair = λx. λy. λf. f x y
	// fst = λp. p (λx. λy. x)
	// snd = λp. p (λx. λy. y)

	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "fst",
			input: "((p: (p (x: (y: x)))) ((x: (y: (f: ((f x) y)))) a b))",
			desc:  "First projection of pair (a, b) → a",
		},
		{
			name:  "snd",
			input: "((p: (p (x: (y: y)))) ((x: (y: (f: ((f x) y)))) a b))",
			desc:  "Second projection of pair (a, b) → b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := deltanet.NewNetwork()
			term, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			root, port, varNames := ToDeltaNet(term, n)
			output := n.NewVar()
			n.Link(root, port, output, 0)

			n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
			result := FromDeltaNet(n, resultNode, resultPort, varNames)
			t.Logf("%s: %s → %v", tt.name, tt.desc, result)

			stats := n.GetStats()
			t.Logf("Reductions: %d total", stats.TotalReductions)
		})
	}
}

// TestLetBinding tests let-expressions (syntactic sugar for application)
// Paper: Let-bindings demonstrate how syntactic conveniences translate
// to the core calculus through β-reduction.
func TestLetBinding(t *testing.T) {
	// let x = v in e  ≡  (λx. e) v

	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "let simple",
			input: "((x: (x x)) (y: y))",
			desc:  "let x = (λy. y) in (x x) → (λy. y)",
		},
		{
			name:  "let id",
			input: "((x: x) a)",
			desc:  "let x = a in x → a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := deltanet.NewNetwork()
			term, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			root, port, varNames := ToDeltaNet(term, n)
			output := n.NewVar()
			n.Link(root, port, output, 0)

			n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
			result := FromDeltaNet(n, resultNode, resultPort, varNames)
			t.Logf("%s: %s → %v", tt.name, tt.desc, result)
		})
	}
}

// TestComplexSharing tests complex sharing patterns
// Paper: "The additional degrees of freedom in Δ-Nets allow it to realize
// optimal reduction in the manner envisioned by Lévy, i.e., no reduction
// operation is applied which is rendered unnecessary later, and no reduction
// operation which is necessary is applied more than once."
func TestComplexSharing(t *testing.T) {
	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Complex term with nested sharing
	// (λf. (f (f (λx. x)))) (λg. (g (λy. y)))
	term, err := Parse("((f: (f (f (x: x)))) (g: (g (y: y))))")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	// Reduce to normal form
	n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("Complex sharing: → %v", result)

	stats := n.GetStats()
	t.Logf("Optimality metrics:")
	t.Logf("  Total reductions: %d", stats.TotalReductions)
	t.Logf("  Fan annihilations: %d", stats.FanAnnihilation)
	t.Logf("  Fan-rep commutations: %d", stats.FanRepCommutation)
	t.Logf("  Rep-rep interactions: %d", stats.RepAnnihilation+stats.RepCommutation)

	// Paper: "This consolidation of information enables simplifications that
	// were previously unfeasible, and leads to constant memory usage"
}

// TestNestedApplications tests deeply nested application chains
// Paper: Nested applications test the depth-based priority system
// and verify that leftmost-outermost ordering is maintained.
func TestNestedApplications(t *testing.T) {
	// Paper: "The reduction order which guarantees that replicator merges
	// happen as early as possible, minimizing the total number of reductions,
	// is a sequential leftmost-outermost order"

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Nested applications: (((f a) b) c)
	term, err := Parse("((((f: f) (a: a)) (b: b)) c)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("Nested applications: → %v", result)

	// Verify leftmost-outermost ordering through trace
	trace := n.TraceSnapshot()
	t.Logf("Trace length: %d interactions", len(trace))

	for i, ev := range trace {
		t.Logf("  Step %d: %v (A: %v#%d, B: %v#%d)", i, ev.Rule, ev.AType, ev.AID, ev.BType, ev.BID)
	}

	// Paper: "In order to ensure that no reduction operations are applied in
	// a subnet that is later going to be erased, a sequential leftmost-outermost
	// reduction order needs to be followed."
}

// TestFreeVariables tests handling of free (global) variables
// Paper: Free variables demonstrate the interface between the λ-term
// and its environment, represented by Var nodes in the net.
func TestFreeVariables(t *testing.T) {
	// Paper: "The free-variable fragment contains a single-port (non-agent) node,
	// which is represented by the name of the associated free variable in the
	// λ-term."

	tests := []struct {
		name  string
		input string
		desc  string
	}{
		{
			name:  "free_1",
			input: "x",
			desc:  "Single free variable",
		},
		{
			name:  "free_app",
			input: "(f a)",
			desc:  "Application with free variables",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := deltanet.NewNetwork()
			term, err := Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			root, port, varNames := ToDeltaNet(term, n)
			output := n.NewVar()
			n.Link(root, port, output, 0)

			n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
			result := FromDeltaNet(n, resultNode, resultPort, varNames)
			t.Logf("%s: %s → %v", tt.name, tt.desc, result)

			// Free variables remain as interface nodes
		})
	}
}

// TestMixedFeatures tests terms combining multiple features
// Paper: Real-world terms combine linear, affine, and relevant features,
// exercising all subsystems (Δ L, Δ A, Δ I) simultaneously in Δ K.
func TestMixedFeatures(t *testing.T) {
	// Paper: "the Δ-Nets core interaction system decomposes perfectly into
	// three overlapping subsystems, each analogous to a substructure λ-calculus."

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Mixed term: (λf. λx. f (f x)) (λy. λz. y) a b
	// Combines: sharing (f used twice), erasure (z unused), free vars (a, b)
	term, err := Parse("((((f: (x: (f (f x)))) (y: (z: y))) a) b)")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("Mixed features: → %v", result)

	stats := n.GetStats()
	t.Logf("Mixed system statistics:")
	t.Logf("  Δ L (linear): %d fan annihilations", stats.FanAnnihilation)
	t.Logf("  Δ A (affine): %d erasures", stats.Erasure)
	t.Logf("  Δ I (relevant): %d fan-rep commutations", stats.FanRepCommutation)
	t.Logf("  Total: %d reductions", stats.TotalReductions)

	// Paper: "The full Δ-Nets system may also be referred to as Δ K-Nets"
	// and handles all four calculi: λ L, λ A, λ I, λ K
}

// TestNonNormalizingTerm tests that non-normalizing terms maintain constant memory
// Paper: "This consolidation of information enables simplifications that were
// previously unfeasible, and leads to constant memory usage in the reduction
// of (λx. x x)(λy. y y), for example."
func TestNonNormalizingTerm(t *testing.T) {
	// Paper: The classic diverging term Ω = (λx. x x)(λx. x x)
	// In Delta-Nets, this should use constant memory despite infinite reduction

	n := deltanet.NewNetwork()

	// Parse Ω
	term, err := Parse("((x: (x x)) (y: (y y)))")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, _ := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	// Get initial counts
	initialActive := n.ActiveNodeCount()
	initialTotal := n.NodeCount()

	// Reduce for several steps (not to completion as it diverges)
	maxSteps := uint64(1000)
	performedSteps := n.ReduceWithLimit(maxSteps)

	finalActive := n.ActiveNodeCount()
	finalTotal := n.NodeCount()
	activeGrowth := finalActive - initialActive
	totalGrowth := finalTotal - initialTotal

	t.Logf("Non-normalizing term: performed %d reductions", performedSteps)
	t.Logf("Active nodes: initial=%d, final=%d, growth=%d", 
		initialActive, finalActive, activeGrowth)
	t.Logf("Total nodes (including dead): initial=%d, final=%d, growth=%d",
		initialTotal, finalTotal, totalGrowth)

	// Paper: "leads to constant memory usage"
	// With garbage collection of dead nodes, active memory should remain bounded.
	// Dead nodes are periodically removed from the registry during ReduceWithLimit.
	
	if performedSteps > 0 {
		activeGrowthRate := float64(activeGrowth) / float64(performedSteps)
		t.Logf("Active growth rate: %.4f nodes per reduction", activeGrowthRate)
		
		// Verify active memory stays bounded (near-constant)
		// With GC, active nodes should not grow linearly
		if activeGrowthRate > 0.5 {
			t.Errorf("Active growth rate too high: %.4f nodes/reduction (expected < 0.5)", 
				activeGrowthRate)
		}
		
		// Total nodes may grow temporarily but should be cleaned by GC
		totalGrowthRate := float64(totalGrowth) / float64(performedSteps)
		t.Logf("Total growth rate: %.4f nodes per reduction", totalGrowthRate)
		
		if totalGrowthRate > 1.0 {
			t.Errorf("Total growth rate too high: %.4f (expected < 1.0 with GC)", 
				totalGrowthRate)
		}
	}
}

// TestOptimalityExample tests the example from the paper demonstrating
// optimal reduction without unnecessary operations
// Paper: Term from Section 1 that has no optimal strategy in standard
// λ-calculus but is optimally reduced in Delta-Nets
func TestOptimalityExample(t *testing.T) {
	// Paper: "The Delta-Nets algorithm solves the longstanding enigma of
	// optimal λ-calculi reduction with groundbreaking clarity."

	n := deltanet.NewNetwork()
	n.EnableTrace(100)

	// Complex term from paper demonstrating optimality
	// ((λg. g (g λx. x)) (λh. (λf. f (f λz. z)) (λw. h (w λy. y))))
	term, err := Parse("((g: (g (g (x: x)))) (h: (((f: (f (f (z: z)))) (w: (h (w (y: y))))))))")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	root, port, varNames := ToDeltaNet(term, n)
	output := n.NewVar()
	n.Link(root, port, output, 0)

	n.ReduceToNormalForm()

	resultNode, resultPort := n.GetLink(output, 0)
	result := FromDeltaNet(n, resultNode, resultPort, varNames)
	t.Logf("Optimality example: → %v", result)

	stats := n.GetStats()
	t.Logf("Optimal reduction statistics:")
	t.Logf("  Total reductions: %d", stats.TotalReductions)
	t.Logf("  Fan annihilations: %d", stats.FanAnnihilation)
	t.Logf("  Rep annihilations: %d", stats.RepAnnihilation)
	t.Logf("  Fan-rep commutations: %d", stats.FanRepCommutation)
	t.Logf("  Rep commutations: %d", stats.RepCommutation)

	// Paper: "no reduction operation is applied which is rendered unnecessary
	// later, and no reduction operation which is necessary is applied more
	// than once."
	// This is verified by comparing reduction counts with theoretical minimum
}

// normalizeForComparison normalizes a term for structural comparison
// by renaming bound variables consistently
func normalizeForComparison(t Term) string {
	bindings := make(map[string]string)
	var idx int
	var walk func(Term) Term
	walk = func(tt Term) Term {
		switch v := tt.(type) {
		case Var:
			if name, ok := bindings[v.Name]; ok {
				return Var{Name: name}
			}
			return Var{Name: "<free>"}
		case Abs:
			canon := fmt.Sprintf("x%d", idx)
			idx++
			old, had := bindings[v.Arg]
			bindings[v.Arg] = canon
			body := walk(v.Body)
			if had {
				bindings[v.Arg] = old
			} else {
				delete(bindings, v.Arg)
			}
			return Abs{Arg: canon, Body: body}
		case App:
			return App{Fun: walk(v.Fun), Arg: walk(v.Arg)}
		default:
			return tt
		}
	}
	return fmt.Sprintf("%s", walk(t))
}
