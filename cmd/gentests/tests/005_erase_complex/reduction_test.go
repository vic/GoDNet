package gentests

import _ "embed"
import "testing"
import "github.com/vic/godnet/cmd/gentests/helper"

//go:embed input.nix
var input string

//go:embed output.nix
var output string

func Test_005_erase_complex_Reduction(t *testing.T) {
	gentests.CheckLambdaReduction(t, "005_erase_complex", input, output)
}
