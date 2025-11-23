package deltanet

import (
	"testing"
)

func TestUnpairedReplicatorDecay(t *testing.T) {
	n := NewNetwork()
	n.EnableTrace(100)

	// Create a Replicator with 1 aux port, delta 0
	// This is the identity replicator that should decay
	rep := n.NewReplicator(0, []int{0})

	// Create two vars to connect
	v1 := n.NewVar()
	v2 := n.NewVar()

	// Connect v1 to Rep Principal
	n.Link(v1, 0, rep, 0)
	// Connect v2 to Rep Aux 0
	n.Link(v2, 0, rep, 1)

	// Run Canonicalization (which should trigger decay)
	// We assume ReduceAll or a specific method handles this.
	// For now, let's assume we'll add a method for this specific check or it happens during reduction if we trigger it.
	// Since it's a static rule on a single node, it might need a trigger.
	// Let's assume we call a new method `ApplyCanonicalRules`.
	n.ApplyCanonicalRules()

	// Check if Rep is gone and v1 is connected to v2
	// Wait for async ops
	n.wg.Wait()

	// Verify connection
	if !n.IsConnected(v1, 0, v2, 0) {
		t.Errorf("Replicator did not decay: v1 and v2 are not connected")
	}

	// Verify trace
	trace := n.TraceSnapshot()
	found := false
	for _, ev := range trace {
		if ev.Rule == RuleRepDecay {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RuleRepDecay not found in trace")
	}
}

func TestUnpairedReplicatorMerging(t *testing.T) {
	n := NewNetwork()
	n.EnableTrace(100)

	// Create Rep A: Level 0, Deltas [1] (Not identity, won't decay)
	repA := n.NewReplicator(0, []int{1})
	// Create Rep B: Level 1, Deltas [-1] (Not identity, won't decay)
	repB := n.NewReplicator(1, []int{-1})

	// Connect Rep A Aux 0 to Rep B Principal
	// This satisfies the condition: B is connected to A via aux port.
	// Level diff: 1 - 0 = 1. Delta of port is 1. 0 + 1 = 1. OK.
	n.Link(repA, 1, repB, 0)

	// Connect vars to outside
	v1 := n.NewVar()
	v2 := n.NewVar()
	n.Link(v1, 0, repA, 0)
	n.Link(v2, 0, repB, 1)

	// Trigger merging
	n.ApplyCanonicalRules()
	// n.wg.Wait()

	// Expectation: A and B merge into a single Replicator.
	// v1 should be connected to the new Replicator's Principal
	// v2 should be connected to the new Replicator's Aux 0

	// Get what v1 is connected to
	target, _ := n.GetLink(v1, 0)
	if target == nil {
		t.Fatalf("v1 is disconnected")
	}
	if target.Type() != NodeTypeReplicator {
		t.Errorf("v1 connected to %v, expected Replicator", target.Type())
	}
	if target.ID() == repA.ID() || target.ID() == repB.ID() {
		// Ideally it's a new node or one of them reused.
		// If it's one of them, the other should be gone.
		// Let's check if we have a single path.
	}

	// Check path v1 -> Rep -> v2
	// New Rep should have 1 aux port (from Rep B)
	if len(target.Ports()) < 2 {
		t.Fatalf("Target replicator has insufficient ports")
	}
	target2, _ := n.GetLink(target, 1) // Aux 0
	if target2 == nil {
		t.Fatalf("Replicator Aux 0 disconnected")
	}
	if target2.ID() != v2.ID() {
		t.Errorf("Replicator Aux 0 connected to %v, expected v2", target2.ID())
	}

	// Verify trace
	trace := n.TraceSnapshot()
	found := false
	for _, ev := range trace {
		if ev.Rule == RuleRepMerge {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("RuleRepMerge not found in trace")
	}
}

func TestPhase2AuxFanReplication(t *testing.T) {
	n := NewNetwork()
	n.EnableTrace(100)

	// Setup: Fan connected to Replicator
	// Fan Principal -> Rep Principal (Active Pair)
	// This normally triggers Fan-Rep commutation.
	// In Phase 2, it should trigger Aux Fan Replication.

	fan := n.NewFan()
	rep := n.NewReplicator(0, []int{0, 0}) // 2 aux ports

	// Connect Fan Principal to Rep Principal
	n.Link(fan, 0, rep, 0)

	// Connect other ports to vars to keep them alive
	v1 := n.NewVar(); n.Link(fan, 1, v1, 0)
	v2 := n.NewVar(); n.Link(fan, 2, v2, 0)
	v3 := n.NewVar(); n.Link(rep, 1, v3, 0)
	v4 := n.NewVar(); n.Link(rep, 2, v4, 0)

	// Force Phase 2
	n.SetPhase(2)

	// Reduce
	n.ReduceAll()

	// Verify trace
	trace := n.TraceSnapshot()
	found := false
	for _, ev := range trace {
		if ev.Rule == RuleAuxFanRep {
			found = true
			break
		}
		if ev.Rule == RuleFanRep {
			t.Errorf("Found RuleFanRep in Phase 2, expected RuleAuxFanRep")
		}
	}
	if !found {
		t.Errorf("RuleAuxFanRep not found in trace")
	}
}
