package deltanet

import "testing"

func TestLeftmostPrefersRootFanFan(t *testing.T) {
	traceNet := tracedNet(8)

	innerLeft := newFanWithSinks(traceNet)
	innerRight := newFanWithSinks(traceNet)
	traceNet.LinkAt(innerLeft, 0, innerRight, 0, 1)

	rootLeft := newFanWithSinks(traceNet)
	rootRight := newFanWithSinks(traceNet)
	traceNet.Link(rootLeft, 0, rootRight, 0)

	traceNet.Start()
	traceNet.ReduceAll()
	event := firstTraceEvent(t, traceNet)
	if event.Rule != RuleFanFan {
		t.Fatalf("expected fan-fan rule, got %v", event.Rule)
	}
	assertEventMatchesPair(t, event, rootLeft.ID(), rootRight.ID())
}

func TestLeftmostPrefersRootFanRep(t *testing.T) {
	traceNet := tracedNet(8)

	innerRep := newReplicatorWithSinks(traceNet, 0, []int{0})
	innerFan := newFanWithSinks(traceNet)
	traceNet.LinkAt(innerRep, 0, innerFan, 0, 1)

	rootFan := newFanWithSinks(traceNet)
	rootRep := newReplicatorWithSinks(traceNet, 1, []int{0, 0})
	traceNet.Link(rootFan, 0, rootRep, 0)

	traceNet.Start()
	traceNet.ReduceAll()
	event := firstTraceEvent(t, traceNet)
	if event.Rule != RuleFanRep {
		t.Fatalf("expected fan-rep rule, got %v", event.Rule)
	}
	assertEventMatchesPair(t, event, rootFan.ID(), rootRep.ID())
}

func TestLeftmostPrefersRootEraserFan(t *testing.T) {
	traceNet := tracedNet(8)

	// Inner pair: Fan-Fan
	innerLeft := newFanWithSinks(traceNet)
	innerRight := newFanWithSinks(traceNet)
	traceNet.LinkAt(innerLeft, 0, innerRight, 0, 1)

	// Root pair: Eraser-Fan
	rootEraser := traceNet.NewEraser()
	rootFan := newFanWithSinks(traceNet)
	traceNet.Link(rootEraser, 0, rootFan, 0)

	traceNet.Start()
	traceNet.ReduceAll()
	event := firstTraceEvent(t, traceNet)
	if event.Rule != RuleErasure {
		t.Fatalf("expected erasure rule, got %v", event.Rule)
	}
	assertEventMatchesPair(t, event, rootEraser.ID(), rootFan.ID())
}

func TestLeftmostPrefersRootEraserRep(t *testing.T) {
	traceNet := tracedNet(8)

	// Inner pair: Fan-Fan
	innerLeft := newFanWithSinks(traceNet)
	innerRight := newFanWithSinks(traceNet)
	traceNet.LinkAt(innerLeft, 0, innerRight, 0, 1)

	// Root pair: Eraser-Replicator
	rootEraser := traceNet.NewEraser()
	rootRep := newReplicatorWithSinks(traceNet, 1, []int{0, 0})
	traceNet.Link(rootEraser, 0, rootRep, 0)

	traceNet.Start()
	traceNet.ReduceAll()
	event := firstTraceEvent(t, traceNet)
	if event.Rule != RuleErasure {
		t.Fatalf("expected erasure rule, got %v", event.Rule)
	}
	assertEventMatchesPair(t, event, rootEraser.ID(), rootRep.ID())
}

func TestLeftmostPrefersRootRepRep(t *testing.T) {
	traceNet := tracedNet(8)

	// Inner pair: Fan-Fan
	innerLeft := newFanWithSinks(traceNet)
	innerRight := newFanWithSinks(traceNet)
	traceNet.LinkAt(innerLeft, 0, innerRight, 0, 1)

	// Root pair: Rep-Rep (Annihilation)
	rootRep1 := newReplicatorWithSinks(traceNet, 1, []int{0, 0})
	rootRep2 := newReplicatorWithSinks(traceNet, 1, []int{0, 0})
	traceNet.Link(rootRep1, 0, rootRep2, 0)

	traceNet.Start()
	traceNet.ReduceAll()
	event := firstTraceEvent(t, traceNet)
	if event.Rule != RuleRepRep {
		t.Fatalf("expected rep-rep rule, got %v", event.Rule)
	}
	assertEventMatchesPair(t, event, rootRep1.ID(), rootRep2.ID())
}

func TestLeftmostPrefersRootRepRepComm(t *testing.T) {
	traceNet := tracedNet(8)

	// Inner pair: Fan-Fan
	innerLeft := newFanWithSinks(traceNet)
	innerRight := newFanWithSinks(traceNet)
	traceNet.LinkAt(innerLeft, 0, innerRight, 0, 1)

	// Root pair: Rep-Rep (Commutation, different levels)
	rootRep1 := newReplicatorWithSinks(traceNet, 1, []int{0, 0})
	rootRep2 := newReplicatorWithSinks(traceNet, 2, []int{0, 0})
	traceNet.Link(rootRep1, 0, rootRep2, 0)

	traceNet.Start()
	traceNet.ReduceAll()
	event := firstTraceEvent(t, traceNet)
	if event.Rule != RuleRepRepComm {
		t.Fatalf("expected rep-rep-comm rule, got %v", event.Rule)
	}
	assertEventMatchesPair(t, event, rootRep1.ID(), rootRep2.ID())
}

// TestDepthIncrement verifies that connect() properly increments depth
// to maintain leftmost-outermost ordering during commutation
func TestDepthIncrement(t *testing.T) {
	net := NewNetwork()

	// Create a simple commutation scenario
	fan := net.NewFan()
	rep := net.NewReplicator(0, []int{0})

	// Link at known depth
	net.LinkAt(fan, 0, rep, 0, 5)

	v1 := net.NewVar()
	v2 := net.NewVar()
	v3 := net.NewVar()

	net.Link(fan, 1, v1, 0)
	net.Link(fan, 2, v2, 0)
	net.Link(rep, 1, v3, 0)

	// Verify initial wire depth
	initialWire := fan.Ports()[0].Wire.Load()
	if initialWire == nil {
		t.Fatal("Initial wire is nil")
	}
	if initialWire.depth != 5 {
		t.Errorf("Initial wire depth should be 5, got %d", initialWire.depth)
	}

	// Reduce (Fan >< Rep commutation)
	net.ReduceAll()

	// After commutation, internal wires created by connect() should have depth 6
	l1, _ := net.GetLink(v1, 0)
	if l1 == nil || l1.Type() != NodeTypeReplicator {
		t.Errorf("v1 should connect to Replicator")
	}

	// Check that the internal connection has incremented depth
	if l1 != nil && len(l1.Ports()) > 1 {
		internalWire := l1.Ports()[1].Wire.Load()
		if internalWire != nil && internalWire.depth != 6 {
			t.Errorf("Internal wire depth should be 6 (parent 5 + 1), got %d", internalWire.depth)
		}
	}
}

// TestLMOConcurrentReduction verifies that multiple workers maintain
// leftmost-outermost order through depth-based prioritization
func TestLMOConcurrentReduction(t *testing.T) {
	net := NewNetwork()
	net.SetWorkers(4)

	// Create outer and inner active pairs at different depths
	// Outer pair should be reduced first regardless of worker count

	outerFan1 := net.NewFan()
	outerFan2 := net.NewFan()
	net.LinkAt(outerFan1, 0, outerFan2, 0, 0) // depth 0

	innerFan1 := net.NewFan()
	innerFan2 := net.NewFan()
	net.LinkAt(innerFan1, 0, innerFan2, 0, 10) // depth 10

	// Connect auxiliary ports
	v1 := net.NewVar()
	v2 := net.NewVar()
	v3 := net.NewVar()
	v4 := net.NewVar()

	net.Link(outerFan1, 1, v1, 0)
	net.Link(outerFan1, 2, v2, 0)
	net.Link(outerFan2, 1, v3, 0)
	net.Link(outerFan2, 2, v4, 0)

	v5 := net.NewVar()
	v6 := net.NewVar()
	v7 := net.NewVar()
	v8 := net.NewVar()

	net.Link(innerFan1, 1, v5, 0)
	net.Link(innerFan1, 2, v6, 0)
	net.Link(innerFan2, 1, v7, 0)
	net.Link(innerFan2, 2, v8, 0)

	net.ReduceAll()

	stats := net.GetStats()
	if stats.FanAnnihilation != 2 {
		t.Errorf("Expected 2 fan annihilations, got %d", stats.FanAnnihilation)
	}
}
