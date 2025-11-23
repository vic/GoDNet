package gentests

import _ "embed"
import "testing"
import "github.com/vic/godnet/cmd/gentests/helper"

//go:embed input.nix
var input string

//go:embed output.nix
var output string

// Test_101_no_optimal_Reduction tests the reduction of a lambda term that has no optimal
// reduction strategy in standard lambda calculus but can be optimally reduced in Delta-Nets.
// This term is from the paper: ((λg.(g (g λx.x))) (λh.((λf.(f (f λz.z))) (λw.(h (w λy.y))))))
// It demonstrates Delta-Nets' ability to handle sharing optimally without unnecessary reductions.
func Test_101_no_optimal_Reduction(t *testing.T) {
	gentests.CheckLambdaReduction(t, "101_no_optimal", input, output)
}
