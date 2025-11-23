package gentests

import _ "embed"
import "testing"
import "github.com/vic/godnet/cmd/gentests/helper"

//go:embed input.nix
var input string

//go:embed output.nix
var output string

func Test_010_zero_Reduction(t *testing.T) {
	gentests.CheckLambdaReduction(t, "010_zero", input, output)
}
