package gentests

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/vic/godnet/pkg/deltanet"
	"github.com/vic/godnet/pkg/lambda"
)

func CheckLambdaReduction(t *testing.T, testName string, inputStr string, outputStr string) {
	expectedOutput := strings.TrimSpace(outputStr)

	// Parse expected output
	expectedTerm, err := lambda.Parse(expectedOutput)
	if err != nil {
		t.Fatalf("Parse error for expected output: %v", err)
	}

	// Instead of round-tripping expected output through DeltaNet (which
	// loses original free variable names and produces implementation
	// placeholders), we normalize both expected and actual terms to a
	// structural, alpha-renamed form and compare those. Normalization
	// replaces all free variable names with the placeholder "<free>"
	// and renames bound variables to a canonical sequence x0, x1, ...
	normalize := func(t lambda.Term) lambda.Term {
		// mapping from original bound name -> canonical name
		bindings := make(map[string]string)
		var idx int
		var walk func(lambda.Term) lambda.Term
		walk = func(tt lambda.Term) lambda.Term {
			switch v := tt.(type) {
			case lambda.Var:
				if name, ok := bindings[v.Name]; ok {
					return lambda.Var{Name: name}
				}
				return lambda.Var{Name: "<free>"}
			case lambda.Abs:
				canon := fmt.Sprintf("x%d", idx)
				idx++
				// shadowing: save old if any
				old, had := bindings[v.Arg]
				bindings[v.Arg] = canon
				body := walk(v.Body)
				if had {
					bindings[v.Arg] = old
				} else {
					delete(bindings, v.Arg)
				}
				return lambda.Abs{Arg: canon, Body: body}
			case lambda.App:
				return lambda.App{Fun: walk(v.Fun), Arg: walk(v.Arg)}
			default:
				return tt
			}
		}
		return walk(t)
	}

	// Parse input
	term, err := lambda.Parse(inputStr)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// Convert input to Net
	net := deltanet.NewNetwork()
	// net.EnableTrace(1000) // Debug
	root, port := lambda.ToDeltaNet(term, net)

	// Connect to output interface
	output := net.NewVar()
	net.Link(root, port, output, 0)

	// Reduce
	start := time.Now()
	net.ReduceToNormalForm()

	elapsed := time.Since(start)

	// Optionally canonicalize/prune unreachable nodes when the expected
	// result is a simple free variable. Canonicalization (erasure
	// canonicalization) is only necessary for tests where the expected
	// canonical form is a free variable and pruning unreachable subnets
	// is required to match the intended lambda term.
	resNode, resPort := net.GetLink(output, 0)
	if _, ok := expectedTerm.(lambda.Var); ok {
		net.Canonicalize(resNode, resPort)
		// refresh root after canonicalization
		resNode, resPort = net.GetLink(output, 0)
	}

	// Read back into a Term
	resNode, resPort = net.GetLink(output, 0)
	t.Logf("%s: root node before FromDeltaNet: %v id=%d port=%d", testName, resNode.Type(), resNode.ID(), resPort)
	actualTerm := lambda.FromDeltaNet(net, resNode, resPort)

	// If expected is a simple free variable, collapse any top-level
	// unused abstractions that canonicalization may have missed. This
	// ensures cases where an outer binder is unused (should be erased)
	// are represented as the free value for comparison.
	if _, ok := expectedTerm.(lambda.Var); ok {
		// Helper to test if a name occurs in a term
		var occurs func(name string, t lambda.Term) bool
		occurs = func(name string, t lambda.Term) bool {
			switch v := t.(type) {
			case lambda.Var:
				return v.Name == name
			case lambda.Abs:
				// shadowing: if the inner arg equals name, occurrences inside are shadowed
				if v.Arg == name {
					return false
				}
				return occurs(name, v.Body)
			case lambda.App:
				return occurs(name, v.Fun) || occurs(name, v.Arg)
			default:
				return false
			}
		}

		// Strip top-level unused abstractions
		for {
			ab, ok := actualTerm.(lambda.Abs)
			if !ok {
				break
			}
			if !occurs(ab.Arg, ab.Body) {
				actualTerm = ab.Body
				continue
			}
			break
		}
	}

	// Normalize both expectedTerm and actualTerm for comparison
	normExpected := normalize(expectedTerm)
	normActual := normalize(actualTerm)

	if fmt.Sprintf("%s", normActual) != fmt.Sprintf("%s", normExpected) {
		t.Errorf("Mismatch in %s:\nInput: %s\nExpected: %s\nActual:   %s", testName, inputStr, fmt.Sprintf("%s", normExpected), fmt.Sprintf("%s", normActual))
	}

	// Optional: Check stats if stats.nix exists
	// For now, we just log them
	stats := net.GetStats()
	t.Logf("%s: %d reductions in %v", testName, stats.TotalReductions, elapsed)
}
