package deltanet

import (
	"testing"
	"time"
)

// TestReplicatorUnpairedMerge verifies that two consecutive unpaired replicators
// that satisfy the paper's constraint (0 <= lB - lA <= d) get merged by the
// canonicalization pass. We construct a small net that should trigger a merge
// and assert the network statistics and topology reflect the merge.
func TestReplicatorUnpairedMerge(t *testing.T) {
	net := NewNetwork()

	// Create two replicators A and B such that B is connected to an aux port of A.
	// Choose levels and delta so that 0 <= lB - lA <= d
	// A: level 3, deltas [2]
	// B: level 5 so that B.Level == A.Level + delta (3 + 2 == 5)
	a := net.NewReplicator(3, []int{2})
	b := net.NewReplicator(5, []int{0})

	// Connect A aux 1 directly to B principal so reduceRepMerge can detect the pattern
	net.Link(a, 1, b, 0)

	// Connect a principal to a Var (root area) so it's part of the net
	root := net.NewVar()
	net.Link(a, 0, root, 0)

	// Also attach a neighbor to B so merge has context
	rightVar := net.NewVar()
	net.Link(b, 1, rightVar, 0)

	// Run reduction which includes canonicalization passes
	done := make(chan bool)
	go func() {
		net.ReduceToNormalForm()
		done <- true
	}()

	select {
	case <-done:
		// proceed
	case <-time.After(2 * time.Second):
		t.Fatal("Reduction timed out - likely a hang")
	}

	// After reduction and canonicalization, there should be evidence of a merge.
	// The implementation exposes a stat counter; check that at least one merge occurred.
	stats := net.GetStats()
	if stats.RepMerge == 0 {
		t.Errorf("Expected at least one replicator merge, got 0")
	}
}

// TestAuxFanReplication ensures the aux-fan replication rule is applied in the
// second phase and produces the expected topology: fan-out replicators eliminated
// and appropriate copies created. We construct a fan connected to a replicator
// and check that aux-fan replication statistic increases.
func TestAuxFanReplicationStat(t *testing.T) {
	net := NewNetwork()

	// Create a fan (application) and a replicator in the auxiliary position so
	// that aux-fan replication should be triggered when reaching phase two.
	fan := net.NewFan()
	rep := net.NewReplicator(1, []int{0, 0})

	// Connect fan principal to rep principal to create active pair (fan.0 <-> rep.0)
	net.Link(fan, 0, rep, 0)

	// Hook up aux ports to vars to keep structure
	v1 := net.NewVar()
	v2 := net.NewVar()
	net.Link(fan, 1, v1, 0)
	net.Link(fan, 2, v2, 0)

	r1 := net.NewVar()
	r2 := net.NewVar()
	net.Link(rep, 1, r1, 0)
	net.Link(rep, 2, r2, 0)

	// Switch to phase 2 to trigger aux-fan replication behavior and reduce
	net.SetPhase(2)
	net.ReduceAll()

	stats := net.GetStats()
	if stats.AuxFanRep == 0 {
		t.Errorf("Expected aux-fan replication to have occurred, got 0")
	}
}
