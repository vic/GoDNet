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
