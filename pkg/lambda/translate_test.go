package lambda

import (
	"github.com/vic/godnet/pkg/deltanet"
	"os"
	"testing"
)

// helper: roundtrip a term through ToDeltaNet -> FromDeltaNet (no reduction)
func roundtrip(t *testing.T, term Term) Term {
	net := deltanet.NewNetwork()
	rootNode, rootPort := ToDeltaNet(term, net)
	// ensure deterministic tiny timeout for network workers
	// Ensure any pending interactions are processed (no-op if none)
	net.ReduceAll()
	res := FromDeltaNet(net, rootNode, rootPort)
	return res
}

func TestRoundtripIdentity(t *testing.T) {
	orig := Abs{Arg: "x", Body: Var{Name: "x"}}
	res := roundtrip(t, orig)
	if _, ok := res.(Abs); !ok {
		t.Fatalf("Identity roundtrip: expected Abs, got %T: %#v", res, res)
	}
}

func TestRoundtripNestedApp(t *testing.T) {
	// (x. (y. (z. ((x y) z))))
	orig := Abs{Arg: "x", Body: Abs{Arg: "y", Body: Abs{Arg: "z", Body: App{Fun: App{Fun: Var{Name: "x"}, Arg: Var{Name: "y"}}, Arg: Var{Name: "z"}}}}}
	res := roundtrip(t, orig)
	// We expect an Abs at top-level
	if _, ok := res.(Abs); !ok {
		t.Fatalf("NestedApp roundtrip: expected Abs, got %T: %#v", res, res)
	}
}

func TestRoundtripFreeVar(t *testing.T) {
	orig := Var{Name: "a"}
	res := roundtrip(t, orig)
	// free variables lose name (deltanet doesn't store names) but structure should be Var
	if _, ok := res.(Var); !ok {
		t.Fatalf("FreeVar roundtrip: expected Var, got %T: %#v", res, res)
	}
}

func TestRoundtripSharedVar(t *testing.T) {
	//
	// (. (f f)) where f is bound and shared
	orig := Abs{Arg: "f", Body: App{Fun: Var{Name: "f"}, Arg: Var{Name: "f"}}}
	res := roundtrip(t, orig)
	if _, ok := res.(Abs); !ok {
		t.Fatalf("SharedVar roundtrip: expected Abs, got %T: %#v", res, res)
	}
}

func TestTranslatorDiagnostics(t *testing.T) {
	// This ensures DELTA_DEBUG can be toggled without breaking behavior.
	old := os.Getenv("DELTA_DEBUG")
	defer os.Setenv("DELTA_DEBUG", old)
	os.Setenv("DELTA_DEBUG", "1")
	orig := Abs{Arg: "x", Body: Var{Name: "x"}}
	_ = roundtrip(t, orig)
}
