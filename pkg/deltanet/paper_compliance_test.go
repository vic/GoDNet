package deltanet

import (
	"testing"
	"time"
)

// TestLMO_ErasureOfDivergingTerm tests that a diverging term (Omega) is erased
// if it is an argument to a function that ignores it (K combinator),
// ensuring that the reduction strategy is Leftmost-Outermost (or at least Outermost).
// Term: (\x. y) ((\z. z z) (\z. z z)) -> y
func TestLMO_ErasureOfDivergingTerm(t *testing.T) {
	net := NewNetwork()

	// Construct (\x. y)
	// Abs Fan: 0=Result, 1=Body, 2=Var
	abs := net.NewFan()
	y := net.NewVar()
	era := net.NewEraser()
	net.LinkAt(abs, 1, y, 0, 0)
	net.LinkAt(abs, 2, era, 0, 0)

	// Construct Omega: (\z. z z) (\z. z z)
	buildSelfApp := func(depth uint64) Node {
		// Abs Fan: 0=Result, 1=Body, 2=Var
		abs := net.NewFan()

		// Body: z z (App)
		// App Fan: 0=Fun, 1=Result, 2=Arg
		app := net.NewFan()

		// Abs.1 (Body) -> App.1 (Result)
		net.LinkAt(abs, 1, app, 1, depth)

		// Var z is shared: Replicator
		// Rep: 0=Input, 1=Fun, 2=Arg
		rep := net.NewReplicator(0, []int{0, 0})
		// Rep.0 -> Abs.2 (Var)
		net.LinkAt(rep, 0, abs, 2, depth)
		// Rep.1 -> App.0 (Fun)
		net.LinkAt(rep, 1, app, 0, depth)
		// Rep.2 -> App.2 (Arg)
		net.LinkAt(rep, 2, app, 2, depth)

		return abs
	}

	omegaFun := buildSelfApp(1)
	omegaArg := buildSelfApp(2)

	// Omega App: 0=Fun, 1=Result, 2=Arg
	omegaApp := net.NewFan()
	// OmegaApp.0 -> OmegaFun.0 (Redex!)
	// Depth 1
	net.LinkAt(omegaApp, 0, omegaFun, 0, 1)
	// OmegaApp.2 -> OmegaArg.0
	net.LinkAt(omegaApp, 2, omegaArg, 0, 2)

	// Main App: (\x. y) Omega
	// App Fan: 0=Fun, 1=Result, 2=Arg
	mainApp := net.NewFan()
	// MainApp.0 -> Abs.0 (Redex!)
	// Depth 0
	net.LinkAt(mainApp, 0, abs, 0, 0)
	// MainApp.2 -> OmegaApp.1 (Arg connects to Result of Omega)
	net.LinkAt(mainApp, 2, omegaApp, 1, 1)

	// Root interface
	root := net.NewVar()
	// MainApp.1 -> Root
	net.LinkAt(mainApp, 1, root, 0, 0)

	// Reduce
	// Use a channel to detect timeout/hang
	done := make(chan bool)
	go func() {
		net.ReduceToNormalForm()
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Reduction timed out - likely looping (LMO failure)")
	}

	// Check result: Root should be connected to y
	if !net.IsConnected(root, 0, y, 0) {
		t.Errorf("Did not reduce to y")
	}
}
