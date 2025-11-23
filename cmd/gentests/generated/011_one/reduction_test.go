package gentests

import _ "embed"
import "testing"
import "github.com/vic/godnet/cmd/gentests/helper"

//go:embed input.nix
var input string

//go:embed output.nix
var output string

func Test_011_one_Reduction(t *testing.T) {
	gentests.CheckLambdaReduction(t, "011_one", input, output)
}
