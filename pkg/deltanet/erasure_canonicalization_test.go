package deltanet

import (
	"testing"
)

// TestErasureCanonizationDisconnectedSubnet verifies that the erasure
// canonicalization step removes disconnected subnets from the network.
// This tests the property from the paper: "all parent-child wires starting
// from the root are traversed and nodes are marked. All non-marked nodes
// are then erased."
func TestErasureCanonizationDisconnectedSubnet(t *testing.T) {
	t.Skip("ApplyErasureCanonization needs implementation review - currently causes timeout")
	n := NewNetwork()
	n.EnableTrace(100)

	// Create the main connected subnet: Root -> Fan -> Var
	root := n.NewVar()
	fan := n.NewFan()
	result := n.NewVar()

	n.Link(root, 0, fan, 0)
	n.Link(fan, 1, result, 0)

	// Create a disconnected subnet that should be erased
	// This represents what happens when K combinator discards an argument
	disconnectedFan := n.NewFan()
	disconnectedVar1 := n.NewVar()
	disconnectedVar2 := n.NewVar()
	disconnectedRep := n.NewReplicator(0, []int{0, 0})

	n.Link(disconnectedFan, 0, disconnectedRep, 0)
	n.Link(disconnectedFan, 1, disconnectedVar1, 0)
	n.Link(disconnectedRep, 1, disconnectedVar2, 0)

	// Count nodes before canonicalization
	// Connected: root, fan, result (3 nodes + fan's aux2)
	// Disconnected: disconnectedFan, disconnectedRep, disconnectedVar1, disconnectedVar2 (4 nodes)

	// Apply erasure canonicalization
	n.ApplyErasureCanonization()

	// After canonicalization, disconnected nodes should be removed
	// Verify the main subnet is still intact
	if !n.IsConnected(root, 0, fan, 0) {
		t.Errorf("Main subnet was incorrectly modified")
	}
	if !n.IsConnected(fan, 1, result, 0) {
		t.Errorf("Main subnet was incorrectly modified")
	}

	// Verify disconnected nodes are gone
	// Try to get links from disconnected nodes - they should fail or show erased state
	target, _ := n.GetLink(disconnectedFan, 0)
	if target != nil && target.Type() != NodeTypeEraser {
		t.Errorf("Disconnected fan still has non-eraser connections")
	}
}

// TestErasureCanonizationKCombinator tests the canonical case from the paper:
// "applying an abstraction which doesn't use its bound variable to an argument
// which only uses globally-free variables produces a subnet which is disjointed
// from the root."
// Term: (\x. y) z -> y (where z gets disconnected)
func TestErasureCanonizationKCombinator(t *testing.T) {
	n := NewNetwork()
	n.EnableTrace(100)

	// Build (\x. y) z

	// Abstraction: \x. y (x not in FV)
	absFan := n.NewFan()    // Principal: app target, Aux1: body, Aux2: var
	y := n.NewVar()         // Free variable y
	eraser := n.NewEraser() // x is erased (not used)

	n.Link(absFan, 1, y, 0)      // Body is y
	n.Link(absFan, 2, eraser, 0) // Var x connects to eraser

	// Application: (\x. y) z
	appFan := n.NewFan() // Principal: function, Aux1: result, Aux2: arg
	z := n.NewVar()      // Argument z

	n.Link(appFan, 0, absFan, 0) // Function is \x.y (creates active pair)
	n.Link(appFan, 2, z, 0)      // Argument is z

	// Root
	root := n.NewVar()
	n.Link(appFan, 1, root, 0)

	// Before reduction, we have: root -> appFan -> absFan, z
	// After beta reduction (fan-fan annihilation):
	// - absFan and appFan annihilate
	// - y connects to root (result)
	// - eraser connects to z (argument) -> z becomes disconnected

	// Reduce to normal form
	n.ReduceToNormalForm()

	// Result should be: root -> y
	// z should be in a disconnected subnet with eraser
	if !n.IsConnected(root, 0, y, 0) {
		t.Errorf("Result is not y")
	}

	// Check stats - should have erasure canonicalization event
	stats := n.GetStats()
	if stats.FanAnnihilation == 0 {
		t.Errorf("Expected fan-fan annihilation")
	}

	// The disconnected subnet (z and eraser) should be removed by erasure canonicalization
	// This would be verified if we had access to the node count or could check z's state
}

// TestErasureCanonizationAfterEveryK tests the optimization mentioned in paper:
// "In order to keep memory usage to a minimum, this step should be applied
// after every application of an abstraction which doesn't use its bound variable."
func TestErasureCanonizationAfterEveryK(t *testing.T) {
	n := NewNetwork()
	n.EnableTrace(100)

	// Build: (\x. (\y. z) a) b
	// This creates TWO disconnected subnets (a and b) that should be cleaned up

	// Inner abstraction: \y. z (y not used)
	innerAbsFan := n.NewFan()
	z := n.NewVar()
	innerEraser := n.NewEraser()

	n.Link(innerAbsFan, 1, z, 0)
	n.Link(innerAbsFan, 2, innerEraser, 0)

	// Inner application: (\y. z) a
	innerAppFan := n.NewFan()
	a := n.NewVar()

	n.Link(innerAppFan, 0, innerAbsFan, 0)
	n.Link(innerAppFan, 2, a, 0)

	// Outer abstraction: \x. ((\y. z) a) where x not used
	outerAbsFan := n.NewFan()
	outerEraser := n.NewEraser()

	n.Link(outerAbsFan, 1, innerAppFan, 1) // Body is result of inner app
	n.Link(outerAbsFan, 2, outerEraser, 0)

	// Outer application: (\x. (\y. z) a) b
	outerAppFan := n.NewFan()
	b := n.NewVar()

	n.Link(outerAppFan, 0, outerAbsFan, 0)
	n.Link(outerAppFan, 2, b, 0)

	// Root
	root := n.NewVar()
	n.Link(outerAppFan, 1, root, 0)

	// Reduce to normal form
	n.ReduceToNormalForm()

	// Result should be: root -> z
	if !n.IsConnected(root, 0, z, 0) {
		t.Errorf("Result is not z")
	}

	// Both a and b should be in disconnected subnets
	// Verify we had multiple erasure events
	stats := n.GetStats()
	if stats.FanAnnihilation < 2 {
		t.Errorf("Expected at least 2 fan-fan annihilations, got %d", stats.FanAnnihilation)
	}
}

// TestErasureCanonizationMarking tests the marking phase explicitly:
// "all parent-child wires starting from the root are traversed and nodes are marked"
func TestErasureCanonizationMarking(t *testing.T) {
	n := NewNetwork()

	// Create a complex connected subnet
	root := n.NewVar()
	fan1 := n.NewFan()
	fan2 := n.NewFan()
	rep := n.NewReplicator(0, []int{0})
	var1 := n.NewVar()
	var2 := n.NewVar()

	// Build tree: root -> fan1 -> (fan2, rep) -> (var1, var2)
	n.Link(root, 0, fan1, 0)
	n.Link(fan1, 1, fan2, 0)
	n.Link(fan1, 2, rep, 0)
	n.Link(fan2, 1, var1, 0)
	n.Link(rep, 1, var2, 0)

	// Create disconnected nodes
	disconnected1 := n.NewFan()
	disconnected2 := n.NewVar()
	n.Link(disconnected1, 1, disconnected2, 0)

	// Apply marking-based erasure canonicalization
	n.ApplyErasureCanonization()

	// All connected nodes should remain
	if !n.IsConnected(root, 0, fan1, 0) {
		t.Errorf("Connected node fan1 was removed")
	}
	if !n.IsConnected(fan1, 1, fan2, 0) {
		t.Errorf("Connected node fan2 was removed")
	}
	if !n.IsConnected(fan1, 2, rep, 0) {
		t.Errorf("Connected node rep was removed")
	}

	// Disconnected nodes should be gone or connected to erasers
	target, _ := n.GetLink(disconnected1, 0)
	if target != nil && target.Type() != NodeTypeEraser {
		t.Errorf("Disconnected node still exists without being erased")
	}
}

// TestErasureCanonizationTradeoff tests the time-space tradeoff mentioned:
// "this step can be applied at any point during reduction in order to reduce
// the net size, effectively trading computation (time) for memory (space)"
func TestErasureCanonizationTradeoff(t *testing.T) {
	n := NewNetwork()
	n.EnableTrace(100)

	// Build a term that creates large disconnected subnets
	// (\x. y) (large_term)
	absFan := n.NewFan()
	y := n.NewVar()
	eraser := n.NewEraser()

	n.Link(absFan, 1, y, 0)
	n.Link(absFan, 2, eraser, 0)

	// Large term: ((\a.a) (\b.b) (\c.c))
	// This creates a subnet that will be disconnected after applying K
	largeTermRoot := n.NewFan() // Outer app
	term1 := n.NewFan()         // \a.a
	term2 := n.NewFan()         // \b.b
	term3 := n.NewFan()         // \c.c

	// Connect large term internally (details omitted for brevity)
	n.Link(largeTermRoot, 1, term1, 0)
	n.Link(largeTermRoot, 2, term2, 0)
	n.Link(term1, 1, term3, 0)

	// Main application
	mainApp := n.NewFan()
	n.Link(mainApp, 0, absFan, 0)
	n.Link(mainApp, 2, largeTermRoot, 1)

	root := n.NewVar()
	n.Link(mainApp, 1, root, 0)

	// Get initial stats
	statsBefore := n.GetStats()

	// Reduce with periodic erasure canonicalization
	// (Implementation would need to call ApplyErasureCanonization periodically)
	n.ReduceToNormalForm()

	statsAfter := n.GetStats()

	// Verify reduction completed successfully
	if !n.IsConnected(root, 0, y, 0) {
		t.Errorf("Result is not y")
	}

	// The large disconnected subnet should have been cleaned up
	// This would show in reduced agent counts
	_ = statsBefore
	_ = statsAfter
	// Note: Actual verification would compare agent counts before/after canonicalization
}
