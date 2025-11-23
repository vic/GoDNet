package deltanet

import "testing"

func newFanWithSinks(net *Network) Node {
	fan := net.NewFan()
	net.Link(fan, 1, net.NewVar(), 0)
	net.Link(fan, 2, net.NewVar(), 0)
	return fan
}

func newReplicatorWithSinks(net *Network, level int, deltas []int) Node {
	rep := net.NewReplicator(level, deltas)
	for i := 1; i < len(rep.Ports()); i++ {
		net.Link(rep, i, net.NewVar(), 0)
	}
	return rep
}

func newEraserWithFanSink(net *Network) (Node, Node) {
	eras := net.NewEraser()
	fan := newFanWithSinks(net)
	net.Link(eras, 0, fan, 0)
	return eras, fan
}

func tracedNet(capacity int) *Network {
	n := NewNetwork()
	n.EnableTrace(capacity)
	n.workers = 1 // Force sequential execution for deterministic order tests
	return n
}

func firstTraceEvent(t *testing.T, net *Network) TraceEvent {
	t.Helper()
	events := net.TraceSnapshot()
	if len(events) == 0 {
		t.Fatalf("expected at least one trace event")
	}
	return events[0]
}

func assertEventMatchesPair(t *testing.T, ev TraceEvent, aID, bID uint64) {
	t.Helper()
	if !((ev.AID == aID && ev.BID == bID) || (ev.AID == bID && ev.BID == aID)) {
		t.Fatalf("event pair mismatch: got (%d,%d) want ids %d and %d", ev.AID, ev.BID, aID, bID)
	}
}
