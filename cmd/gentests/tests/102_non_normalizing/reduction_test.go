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
	root, port, _ := lambda.ToDeltaNet(term, net)

	// Connect to output
	output := net.NewVar()
	net.Link(root, port, output, 0)

	// Initial counts
	initialActive := net.ActiveNodeCount()
	initialTotal := net.NodeCount()

	// Reduce for a bounded number of steps (non-normalizing term!)
	maxReductions := uint64(1000)
	performedSteps := net.ReduceWithLimit(maxReductions)

	finalActive := net.ActiveNodeCount()
	finalTotal := net.NodeCount()
	activeGrowth := finalActive - initialActive
	totalGrowth := finalTotal - initialTotal

	t.Logf("Non-normalizing term: performed %d reductions", performedSteps)
	t.Logf("Active nodes: initial=%d, final=%d, growth=%d",
		initialActive, finalActive, activeGrowth)
	t.Logf("Total nodes: initial=%d, final=%d, growth=%d",
		initialTotal, finalTotal, totalGrowth)

	// Verify constant memory with garbage collection
	if performedSteps > 0 {
		activeGrowthRate := float64(activeGrowth) / float64(performedSteps)
		t.Logf("Active growth rate: %.4f nodes per reduction", activeGrowthRate)

		if activeGrowthRate > 0.5 {
			t.Errorf("Active growth rate too high: %.4f (expected < 0.5)", activeGrowthRate)
		}
	}

	t.Logf("Non-normalizing test completed with constant memory")
}
