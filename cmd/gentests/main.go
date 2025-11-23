package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vic/godnet/pkg/lambda"
)

type TestCase struct {
	Name   string
	Input  string
	Output string
}

const testTemplate = `
package gentests
import _ "embed"
import "testing"
import "github.com/vic/godnet/cmd/gentests/helper"
//go:embed input.nix
var input string
//go:embed output.nix
var output string
func Test_%s_Reduction(t *testing.T) {
	gentests.CheckLambdaReduction(t, "%s", input, output)
}
`

func main() {
	tests := []TestCase{
		// Identity
		{"001_id", "x: x", "y: y"},
		{"002_id_id", "(x: x) (y: y)", "z: z"},

		// K Combinator (Erasure)
		{"003_k_1", "(x: y: x) a b", "a"},
		{"004_k_2", "(x: y: y) a b", "b"},
		{"005_erase_complex", "(x: y: x) a ((z: z) b)", "a"},

		// S Combinator (Sharing)
		{"006_s_1", "(x: y: z: x z (y z)) (a: b: a) (c: d: c) e", "e"},
		{"007_s_2", "(x: y: z: x z (y z)) (a: b: b) (c: d: c) e", "e"},

		// Church Numerals
		{"010_zero", "(f: x: x) f x", "x"},
		{"011_one", "(f: x: f x) f x", "f x"},
		{"012_two", "(f: x: f (f x)) f x", "f (f x)"},
		{"013_succ_0", "(n: f: x: f (n f x)) (f: x: x) f x", "f x"},
		//{"014_succ_1", "(n: f: x: f (n f x)) (f: x: f x) f x", "f (f x)"},
		//{"015_add_1_1", "(m: n: f: x: m f (n f x)) (f: x: f x) (f: x: f x) f x", "f (f x)"},
		//{"016_mul_2_2", "(m: n: f: m (n f)) (f: x: f (f x)) (f: x: f (f x)) f x", "f (f (f (f x)))"},

		// Logic
		{"020_true", "(x: y: x) a b", "a"},
		{"021_false", "(x: y: y) a b", "b"},
		{"022_not_true", "(b: b (x: y: y) (x: y: x)) (x: y: x) a b", "b"},
		{"023_not_false", "(b: b (x: y: y) (x: y: x)) (x: y: y) a b", "a"},
		{"024_and_true_true", "(p: q: p q p) (x: y: x) (x: y: x) a b", "a"},
		{"025_and_true_false", "(p: q: p q p) (x: y: x) (x: y: y) a b", "b"},

		// Pairs
		{"030_pair_fst", "(p: p (x: y: x)) ((x: y: f: f x y) a b)", "a"},
		{"031_pair_snd", "(p: p (x: y: y)) ((x: y: f: f x y) a b)", "b"},

		// Let bindings
		{"040_let_simple", "let x = a; in x", "a"},
		{"041_let_id", "let i = x: x; in i a", "a"},
		//{"042_let_nested", "let x = a; in let y = b; in x", "a"},
		//{"043_let_shadow", "let x = a; in let x = b; in x", "b"},

		// Complex / Stress
		//{"050_deep_app", "(x: x x x) (y: y)", "y: y"},
		{"051_share_app", "(f: f (f x)) (y: y)", "x"},

		//{"060_pow_2_3", "(b: e: e b) (f: x: f (f x)) (f: x: f (f (f x))) f x", "f (f (f (f (f (f (f (f x)))))))"},

		// Replicator tests
		{"070_share_complex", "(x: x (x a)) (y: y)", "a"},

		// Erasure of shared term
		{"071_erase_shared", "(x: y: y) ((z: z) a) b", "b"},

		// Commutation
		//{"072_self_app", "(x: x x) (y: y)", "y: y"},

		// Nested Lambdas
		//{"080_nested_1", "x: y: z: x y z", "x: y: z: x y z"},
		{"081_nested_app", "(x: y: x y) a b", "a b"},

		// Free variables
		{"090_free_1", "x", "x"},
		{"091_free_app", "x y", "x y"},
		//{"092_free_abs", "y: x y", "y: x y"},

		// Mixed
		{"100_mixed_1", "(x: x) ((y: y) a)", "a"},
	}

	baseDir := "cmd/gentests/generated"
	os.MkdirAll(baseDir, 0755)

	for _, tc := range tests {
		dir := filepath.Join(baseDir, tc.Name)
		os.MkdirAll(dir, 0755)

		// Normalize Input
		inTerm, err := lambda.Parse(tc.Input)
		if err != nil {
			fmt.Printf("Error parsing input for %s: %v\n", tc.Name, err)
			continue
		}

		// Normalize Output
		outTerm, err := lambda.Parse(tc.Output)
		if err != nil {
			fmt.Printf("Error parsing output for %s: %v\n", tc.Name, err)
			continue
		}

		testGo := fmt.Sprintf(testTemplate, tc.Name, tc.Name)

		os.WriteFile(filepath.Join(dir, "input.nix"), []byte(inTerm.String()), 0644)
		os.WriteFile(filepath.Join(dir, "output.nix"), []byte(outTerm.String()), 0644)
		os.WriteFile(filepath.Join(dir, "reduction_test.go"), []byte(testGo), 0644)
	}

	fmt.Printf("Generated %d tests\n", len(tests))
}

/*
func church(n int) string {
	body := "x"
	for i := 0; i < n; i++ {
		body = fmt.Sprintf("f (%s)", body)
	}
	return fmt.Sprintf("(f: x: %s)", body)
}

func churchBody(n int) string {
	body := "x"
	for i := 0; i < n; i++ {
		body = fmt.Sprintf("f (%s)", body)
	}
	return body
}
*/
