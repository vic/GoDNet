package gentests

import _ "embed"
import "testing"
import "github.com/vic/godnet/cmd/gentests/helper"

//go:embed input.nix
var input string

//go:embed output.nix
var output string

func Test_025_and_true_false_Reduction(t *testing.T) {
	gentests.CheckLambdaReduction(t, "025_and_true_false", input, output)
}
