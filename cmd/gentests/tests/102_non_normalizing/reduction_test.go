package gentests

import _ "embed"
import "testing"
import "github.com/vic/godnet/pkg/lambda"
import "github.com/vic/godnet/pkg/deltanet"

//go:embed input.nix
var input string

func Test_102_non_normalizing_ConstantMemory(t *testing.T) {
	// Parse input
	term, err := lambda.Parse(input)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Convert to Net
	net := deltanet.NewNetwork()
	root, port := lambda.ToDeltaNet(term, net)

	// Connect to output
	output := net.NewVar()
	net.Link(root, port, output, 0)

	// Initial node count
	initialNodes := net.NodeCount()

	// Reduce for a few steps
	for i := 0; i < 10; i++ {
		prevOps := net.GetStats().TotalReductions
		net.ReduceAll()
		currOps := net.GetStats().TotalReductions
		if currOps == prevOps {
			// Converged
			break
		}

		// Check node count doesn't grow excessively
		currentNodes := net.NodeCount()
		if currentNodes > initialNodes+200 { // Allow some growth
			t.Errorf("Node count grew too much: initial %d, current %d at step %d", initialNodes, currentNodes, i)
		}
	}

	t.Logf("Non-normalizing test completed with constant memory")
}
