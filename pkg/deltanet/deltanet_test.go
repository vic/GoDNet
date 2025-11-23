package deltanet

import (
	"testing"
)

// TestFanAnnihilation tests Beta-reduction (Fan-Fan interaction).
func TestFanAnnihilation(t *testing.T) {
	net := NewNetwork()

	// Create two fans facing each other
	f1 := net.NewFan()
	f2 := net.NewFan()

	// Link Principal ports
	net.Link(f1, 0, f2, 0)

	// Create wires for aux ports
	in1 := net.NewVar()
	in2 := net.NewVar()
	out1 := net.NewVar()
	out2 := net.NewVar()

	net.Link(f1, 1, in1, 0)
	net.Link(f1, 2, in2, 0)
	net.Link(f2, 1, out1, 0)
	net.Link(f2, 2, out2, 0)

	// Reduce
	net.ReduceAll()

	// Verify connections: in1 <-> out1, in2 <-> out2
	if !net.IsConnected(in1, 0, out1, 0) {
		t.Errorf("Fan annihilation failed: Port 1s not connected")
	}
	if !net.IsConnected(in2, 0, out2, 0) {
		t.Errorf("Fan annihilation failed: Port 2s not connected")
	}
}

// TestEraserFanInteraction tests Eraser eating a Fan.
func TestEraserFanInteraction(t *testing.T) {
	net := NewNetwork()

	era := net.NewEraser()
	fan := net.NewFan()

	net.Link(era, 0, fan, 0)

	w1 := net.NewVar()
	w2 := net.NewVar()
	net.Link(fan, 1, w1, 0)
	net.Link(fan, 2, w2, 0)

	net.ReduceAll()

	// w1 and w2 should now be connected to NEW Erasers
	target1, _ := net.GetLink(w1, 0)
	if target1 == nil || target1.Type() != NodeTypeEraser {
		t.Errorf("Port 1 not connected to Eraser, got %v", target1)
	}

	target2, _ := net.GetLink(w2, 0)
	if target2 == nil || target2.Type() != NodeTypeEraser {
		t.Errorf("Port 2 not connected to Eraser, got %v", target2)
	}
}

// TestFanReplicatorCommutation tests Fan passing through Replicator.
func TestFanReplicatorCommutation(t *testing.T) {
	net := NewNetwork()

	fan := net.NewFan()
	// Replicator Level 1, 2 aux ports, deltas [0, 0]
	rep := net.NewReplicator(1, []int{0, 0})

	net.Link(fan, 0, rep, 0)

	// Fan Aux
	fAux1 := net.NewVar()
	fAux2 := net.NewVar()
	net.Link(fan, 1, fAux1, 0)
	net.Link(fan, 2, fAux2, 0)

	// Rep Aux
	rAux1 := net.NewVar()
	rAux2 := net.NewVar()
	net.Link(rep, 1, rAux1, 0)
	net.Link(rep, 2, rAux2, 0)

	net.ReduceAll()

	// Expected Topology:
	// fAux1 connected to Rep(Copy1)
	// fAux2 connected to Rep(Copy2)
	// rAux1 connected to Fan(CopyA)
	// rAux2 connected to Fan(CopyB)
	// And the internal connections between Rep copies and Fan copies.

	l1, _ := net.GetLink(fAux1, 0)
	if l1 == nil || l1.Type() != NodeTypeReplicator {
		t.Errorf("Fan Aux 1 should connect to Replicator, got %v", l1)
	}

	l2, _ := net.GetLink(fAux2, 0)
	if l2 == nil || l2.Type() != NodeTypeReplicator {
		t.Errorf("Fan Aux 2 should connect to Replicator, got %v", l2)
	}

	l3, _ := net.GetLink(rAux1, 0)
	if l3 == nil || l3.Type() != NodeTypeFan {
		t.Errorf("Rep Aux 1 should connect to Fan, got %v", l3)
	}
}

// TestReplicatorReplicatorAnnihilation tests two identical replicators annihilating.
func TestReplicatorReplicatorAnnihilation(t *testing.T) {
	net := NewNetwork()

	r1 := net.NewReplicator(5, []int{1, 2})
	r2 := net.NewReplicator(5, []int{1, 2})

	net.Link(r1, 0, r2, 0)

	in1 := net.NewVar()
	in2 := net.NewVar()
	out1 := net.NewVar()
	out2 := net.NewVar()

	net.Link(r1, 1, in1, 0)
	net.Link(r1, 2, in2, 0)
	net.Link(r2, 1, out1, 0)
	net.Link(r2, 2, out2, 0)

	net.ReduceAll()

	if !net.IsConnected(in1, 0, out1, 0) {
		t.Errorf("Rep annihilation failed: Port 1s not connected")
	}
	if !net.IsConnected(in2, 0, out2, 0) {
		t.Errorf("Rep annihilation failed: Port 2s not connected")
	}
}

// TestReplicatorReplicatorCommutation tests two replicators with different levels commuting.
func TestReplicatorReplicatorCommutation(t *testing.T) {
	net := NewNetwork()

	// R1 (Level 1) <-> R2 (Level 2)
	r1 := net.NewReplicator(1, []int{0}) // 1 aux port
	r2 := net.NewReplicator(2, []int{0}) // 1 aux port

	net.Link(r1, 0, r2, 0)

	in := net.NewVar()
	out := net.NewVar()

	net.Link(r1, 1, in, 0)
	net.Link(r2, 1, out, 0)

	net.ReduceAll()

	// Result:
	// R1 replicates R2 -> R2 copies connected to R1 neighbors.
	// R2 replicates R1 -> R1 copies connected to R2 neighbors.
	// Since both have 1 aux port:
	// in -> R2_copy -> R1_copy -> out
	// Wait, let's trace.
	// R1 (Level 1) <-> R2 (Level 2).
	// R1 replicates R2.
	// R1 has 1 aux port (connected to 'in').
	// So we create 1 copy of R2 (R2').
	// R2' principal connects to 'in'.
	// R2 replicates R1.
	// R2 has 1 aux port (connected to 'out').
	// So we create 1 copy of R1 (R1').
	// R1' principal connects to 'out'.
	// Internal connection: R1' aux connects to R2' aux.

	// So: in <-> R2'(0). R2'(1) <-> R1'(1). R1'(0) <-> out.

	l1, _ := net.GetLink(in, 0)
	if l1 == nil || l1.Type() != NodeTypeReplicator || l1.Level() != 2 {
		t.Errorf("Input should connect to Replicator Level 2, got %v", l1)
	}

	l2, _ := net.GetLink(out, 0)
	if l2 == nil || l2.Type() != NodeTypeReplicator || l2.Level() != 1 {
		t.Errorf("Output should connect to Replicator Level 1, got %v", l2)
	}
}

// TestEraserEraserInteraction tests Eraser annihilating Eraser.
func TestEraserEraserInteraction(t *testing.T) {
	net := NewNetwork()
	e1 := net.NewEraser()
	e2 := net.NewEraser()
	net.Link(e1, 0, e2, 0)
	net.ReduceAll()
	// Success if no hang/crash.
}

// TestEraserReplicatorInteraction tests Eraser erasing a Replicator.
func TestEraserReplicatorInteraction(t *testing.T) {
	net := NewNetwork()
	e := net.NewEraser()
	r := net.NewReplicator(0, []int{0, 0}) // 2 aux ports
	net.Link(e, 0, r, 0)

	v1 := net.NewVar()
	v2 := net.NewVar()
	net.Link(r, 1, v1, 0)
	net.Link(r, 2, v2, 0)

	net.ReduceAll()

	verifyEraserConnection(t, net, v1)
	verifyEraserConnection(t, net, v2)
}

// TestReplicatorEraserInteraction tests Replicator being erased by Eraser (Symmetric).
func TestReplicatorEraserInteraction(t *testing.T) {
	net := NewNetwork()
	r := net.NewReplicator(0, []int{0})
	e := net.NewEraser()
	net.Link(r, 0, e, 0)

	v1 := net.NewVar()
	net.Link(r, 1, v1, 0)

	net.ReduceAll()

	verifyEraserConnection(t, net, v1)
}

// TestFanEraserInteraction tests Fan being erased by Eraser (Symmetric).
func TestFanEraserInteraction(t *testing.T) {
	net := NewNetwork()
	f := net.NewFan()
	e := net.NewEraser()
	net.Link(f, 0, e, 0)

	v1 := net.NewVar()
	v2 := net.NewVar()
	net.Link(f, 1, v1, 0)
	net.Link(f, 2, v2, 0)

	net.ReduceAll()

	verifyEraserConnection(t, net, v1)
	verifyEraserConnection(t, net, v2)
}

// TestReplicatorFanInteraction tests Replicator commuting with Fan (Symmetric).
func TestReplicatorFanInteraction(t *testing.T) {
	net := NewNetwork()
	rep := net.NewReplicator(1, []int{0, 0})
	fan := net.NewFan()

	net.Link(rep, 0, fan, 0)

	rAux1 := net.NewVar()
	rAux2 := net.NewVar()
	net.Link(rep, 1, rAux1, 0)
	net.Link(rep, 2, rAux2, 0)

	fAux1 := net.NewVar()
	fAux2 := net.NewVar()
	net.Link(fan, 1, fAux1, 0)
	net.Link(fan, 2, fAux2, 0)

	net.ReduceAll()

	// Check topology:
	// rAux1 -> Fan
	// rAux2 -> Fan
	// fAux1 -> Rep
	// fAux2 -> Rep

	l1, _ := net.GetLink(rAux1, 0)
	if l1 == nil || l1.Type() != NodeTypeFan {
		t.Errorf("Rep Aux 1 should connect to Fan, got %v", l1)
	}

	l2, _ := net.GetLink(fAux1, 0)
	if l2 == nil || l2.Type() != NodeTypeReplicator {
		t.Errorf("Fan Aux 1 should connect to Replicator, got %v", l2)
	}
}

// TestComplexNormalization tests that new active pairs are handled correctly.
func TestComplexNormalization(t *testing.T) {
	net := NewNetwork()
	// Setup: F1(0) <-> R(0). R(1) <-> F2(0).
	// F1 >< R reduces first.
	// This creates a copy of F1 (F1') connected to R's neighbor at port 1 (F2).
	// So F1'(0) <-> F2(0) becomes active.
	// Then F1' >< F2 reduces (Fan-Fan annihilation).

	f1 := net.NewFan()
	r := net.NewReplicator(0, []int{0, 0})
	f2 := net.NewFan()

	net.Link(f1, 0, r, 0)
	net.Link(r, 1, f2, 0)

	// Aux ports
	v1 := net.NewVar()
	v2 := net.NewVar()
	net.Link(f1, 1, v1, 0)
	net.Link(f1, 2, v2, 0)

	v3 := net.NewVar()
	net.Link(r, 2, v3, 0)

	v4 := net.NewVar()
	v5 := net.NewVar()
	net.Link(f2, 1, v4, 0)
	net.Link(f2, 2, v5, 0)

	net.ReduceAll()

	// If successful, we should see connections between the vars.
	// F1 >< R:
	// R copies F1. R_copy1 connects to v1, R_copy2 connects to v2.
	// F1 copies R. F1_copy1 connects to F2(0). F1_copy2 connects to v3.
	//
	// F1_copy1 >< F2:
	// F1_copy1 is a Fan. F2 is a Fan.
	// Annihilation.
	// F1_copy1 aux ports connect to F2 aux ports.
	// F1_copy1 aux ports come from R copies?
	// Wait.
	// F1 >< R:
	// F1 has aux v1, v2.
	// R has aux F2, v3.
	//
	// R copies F1 (R_v1, R_v2).
	// R_v1 principal -> v1. Aux -> F1 copies aux 1.
	// R_v2 principal -> v2. Aux -> F1 copies aux 2.
	//
	// F1 copies R (F1_F2, F1_v3).
	// F1_F2 principal -> F2. Aux 1 -> R_v1 aux 1. Aux 2 -> R_v2 aux 1.
	// F1_v3 principal -> v3. Aux 1 -> R_v1 aux 2. Aux 2 -> R_v2 aux 2.
	//
	// Now F1_F2 >< F2 (Fan >< Fan).
	// F1_F2 aux 1 (connected to R_v1 aux 1) connects to F2 aux 1 (v4).
	// F1_F2 aux 2 (connected to R_v2 aux 1) connects to F2 aux 2 (v5).
	//
	// So:
	// R_v1 aux 1 <-> v4.
	// R_v2 aux 1 <-> v5.
	//
	// R_v1 is a Replicator copy. Principal -> v1.
	// R_v2 is a Replicator copy. Principal -> v2.
	//
	// So we have:
	// v1 <-> R_v1(0). R_v1(1) <-> v4. R_v1(2) <-> ... (connected to F1_v3 aux 1)
	// v2 <-> R_v2(0). R_v2(1) <-> v5. R_v2(2) <-> ... (connected to F1_v3 aux 2)
	//
	// F1_v3 is a Fan copy. Principal -> v3.
	// F1_v3(1) <-> R_v1(2).
	// F1_v3(2) <-> R_v2(2).
	//
	// Topology check:
	// v1 should be connected to a Replicator.
	// That Replicator's port 1 should be connected to v4.
	// That Replicator's port 2 should be connected to a Fan (F1_v3).
	// That Fan's principal should be connected to v3.

	l, _ := net.GetLink(v1, 0)
	if l == nil || l.Type() != NodeTypeReplicator {
		t.Errorf("v1 should connect to Replicator, got %v", l)
		return
	}
	// Check l's port 1
	l_p1, _ := net.GetLink(l, 1)
	// l_p1 should be v4 (which is a Var, so we check if it IS v4's node)
	// But v4 is a Var node.
	// Wait, GetLink returns the Node.
	if l_p1 != v4 {
		t.Errorf("v1's Replicator port 1 should connect to v4, got %v", l_p1)
	}
}

// TestReplicatorDeltaShift verifies that Replicator commutation correctly shifts levels by delta.
func TestReplicatorDeltaShift(t *testing.T) {
	net := NewNetwork()

	// R1: Level 10, Delta [5]
	r1 := net.NewReplicator(10, []int{5})
	// R2: Level 20, Delta [0]
	r2 := net.NewReplicator(20, []int{0})

	// Connect R1 >< R2
	net.Link(r1, 0, r2, 0)

	// Aux ports
	in := net.NewVar()
	out := net.NewVar()
	net.Link(r1, 1, in, 0)
	net.Link(r2, 1, out, 0)

	net.ReduceAll()

	// R1 (Level 10) < R2 (Level 20).
	// R1 replicates R2.
	// R2 copy level = R2.Level + R1.Delta = 20 + 5 = 25.
	// R2 copy connects to 'in' (R1's neighbor).
	
	// R2 replicates R1.
	// R1 copy level = R1.Level = 10.
	// R1 copy connects to 'out' (R2's neighbor).

	// Check 'in' connection
	l1, _ := net.GetLink(in, 0)
	if l1 == nil {
		t.Fatal("in not connected")
	}
	if l1.Type() != NodeTypeReplicator {
		t.Errorf("in should connect to Replicator, got %v", l1.Type())
	}
	if l1.Level() != 25 {
		t.Errorf("Expected R2 copy level to be 25 (20+5), got %d", l1.Level())
	}

	// Check 'out' connection
	l2, _ := net.GetLink(out, 0)
	if l2 == nil {
		t.Fatal("out not connected")
	}
	if l2.Type() != NodeTypeReplicator {
		t.Errorf("out should connect to Replicator, got %v", l2.Type())
	}
	if l2.Level() != 10 {
		t.Errorf("Expected R1 copy level to be 10, got %d", l2.Level())
	}
}

// TestReplicatorMultiDelta verifies that Replicator commutation handles multiple deltas correctly.
func TestReplicatorMultiDelta(t *testing.T) {
	net := NewNetwork()
	
	// R1: Level 10, Deltas [5, 10]
	r1 := net.NewReplicator(10, []int{5, 10})
	// R2: Level 20, Deltas [0]
	r2 := net.NewReplicator(20, []int{0})
	
	net.Link(r1, 0, r2, 0)
	
	// R1 aux
	in1 := net.NewVar()
	in2 := net.NewVar()
	net.Link(r1, 1, in1, 0)
	net.Link(r1, 2, in2, 0)
	
	// R2 aux
	out := net.NewVar()
	net.Link(r2, 1, out, 0)
	
	net.ReduceAll()
	
	// Check in1 -> R2 copy with level 25
	l1, _ := net.GetLink(in1, 0)
	if l1 == nil || l1.Type() != NodeTypeReplicator {
		t.Errorf("in1 should connect to Replicator")
	} else if l1.Level() != 25 {
		t.Errorf("in1 Replicator level: expected 25, got %d", l1.Level())
	}
	
	// Check in2 -> R2 copy with level 30
	l2, _ := net.GetLink(in2, 0)
	if l2 == nil || l2.Type() != NodeTypeReplicator {
		t.Errorf("in2 should connect to Replicator")
	} else if l2.Level() != 30 {
		t.Errorf("in2 Replicator level: expected 30, got %d", l2.Level())
	}
}

// TestEraserPropagation verifies that an Eraser recursively destroys a structure.
func TestEraserPropagation(t *testing.T) {
	net := NewNetwork()
	
	// E >< F1
	//      | \
	//      F2 F3
	
	e := net.NewEraser()
	f1 := net.NewFan()
	f2 := net.NewFan()
	f3 := net.NewFan()
	
	net.Link(e, 0, f1, 0)
	net.Link(f1, 1, f2, 0)
	net.Link(f1, 2, f3, 0)
	
	// Vars at the leaves
	v1 := net.NewVar()
	v2 := net.NewVar()
	v3 := net.NewVar()
	v4 := net.NewVar()
	
	net.Link(f2, 1, v1, 0)
	net.Link(f2, 2, v2, 0)
	net.Link(f3, 1, v3, 0)
	net.Link(f3, 2, v4, 0)
	
	net.ReduceAll()
	
	// All vars should be connected to Erasers
	verifyEraserConnection(t, net, v1)
	verifyEraserConnection(t, net, v2)
	verifyEraserConnection(t, net, v3)
	verifyEraserConnection(t, net, v4)
}

func verifyEraserConnection(t *testing.T, net *Network, n Node) {
	l, _ := net.GetLink(n, 0)
	if l == nil || l.Type() != NodeTypeEraser {
		t.Errorf("Node should be connected to Eraser, got %v", l)
	}
}
