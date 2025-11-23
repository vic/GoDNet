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
	fmt.Fprintf(os.Stderr, "\nStats:\n")
	fmt.Fprintf(os.Stderr, "Time: %v\n", elapsed)
	fmt.Fprintf(os.Stderr, "Total Reductions: %d\n", stats.TotalReductions)
	if elapsed.Seconds() > 0 {
		fmt.Fprintf(os.Stderr, "Reductions/sec: %.2f\n", float64(stats.TotalReductions)/elapsed.Seconds())
	}
	fmt.Fprintf(os.Stderr, "Fan Annihilation: %d\n", stats.FanAnnihilation)
	fmt.Fprintf(os.Stderr, "Replicator Annihilation: %d\n", stats.RepAnnihilation)
	fmt.Fprintf(os.Stderr, "Replicator Commutation: %d\n", stats.RepCommutation)
	fmt.Fprintf(os.Stderr, "Fan-Replicator Commutation: %d\n", stats.FanRepCommutation)
	fmt.Fprintf(os.Stderr, "Erasure: %d\n", stats.Erasure)
}
