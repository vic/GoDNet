package main

import (
	"fmt"
	"io"
	"os"

	"time"

	"github.com/vic/godnet/pkg/deltanet"
	"github.com/vic/godnet/pkg/lambda"
)

func main() {
	var input []byte
	var err error

	if len(os.Args) > 1 {
		input, err = os.ReadFile(os.Args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
	} else {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			os.Exit(1)
		}
	}

	term, err := lambda.Parse(string(input))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		os.Exit(1)
	}

	net := deltanet.NewNetwork()
	root, port, varNames := lambda.ToDeltaNet(term, net)

	// Connect root to a dummy interface node to allow reduction at the root
	output := net.NewVar()
	net.Link(root, port, output, 0)

	start := time.Now()
	net.ReduceAll()
	elapsed := time.Since(start)

	// Read back from the output node
	resNode, resPort := net.GetLink(output, 0)
	res := lambda.FromDeltaNet(net, resNode, resPort, varNames)
	fmt.Println(res)

	stats := net.GetStats()
	seconds := elapsed.Seconds()

	fmt.Fprintf(os.Stderr, "\nStats:\n")
	fmt.Fprintf(os.Stderr, "Time: %v\n", elapsed)
	fmt.Fprintf(os.Stderr, "Total Reductions: %d", stats.TotalReductions)
	if seconds > 0 {
		fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.TotalReductions)/seconds)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "\nBreakdown:\n")
	fmt.Fprintf(os.Stderr, "  Fan Annihilation:        %6d", stats.FanAnnihilation)
	if seconds > 0 {
		fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.FanAnnihilation)/seconds)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "  Replicator Annihilation: %6d", stats.RepAnnihilation)
	if seconds > 0 {
		fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.RepAnnihilation)/seconds)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "  Replicator Commutation:  %6d", stats.RepCommutation)
	if seconds > 0 {
		fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.RepCommutation)/seconds)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "  Fan-Rep Commutation:     %6d", stats.FanRepCommutation)
	if seconds > 0 {
		fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.FanRepCommutation)/seconds)
	}
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "  Erasure:                 %6d", stats.Erasure)
	if seconds > 0 {
		fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.Erasure)/seconds)
	}
	fmt.Fprintf(os.Stderr, "\n")

	if stats.RepDecay > 0 {
		fmt.Fprintf(os.Stderr, "  Replicator Decay:        %6d", stats.RepDecay)
		if seconds > 0 {
			fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.RepDecay)/seconds)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if stats.RepMerge > 0 {
		fmt.Fprintf(os.Stderr, "  Replicator Merge:        %6d", stats.RepMerge)
		if seconds > 0 {
			fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.RepMerge)/seconds)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	if stats.AuxFanRep > 0 {
		fmt.Fprintf(os.Stderr, "  Aux Fan-Rep:             %6d", stats.AuxFanRep)
		if seconds > 0 {
			fmt.Fprintf(os.Stderr, " (%.2f ops/sec)", float64(stats.AuxFanRep)/seconds)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
}
