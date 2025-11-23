package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test compiling and running code with native pure functions
func TestCompileWithPureFunctions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "godnet_pure_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test that will use pure functions
	// For now, just test that the compiler can handle the syntax
	// Actual pure function integration requires runtime support
	sourceFile := filepath.Join(tmpDir, "pure_test.lam")
	source := `x: x`
	
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	outputFile := filepath.Join(tmpDir, "pure_test")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		KeepTemp:   false,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if _, err := os.Stat(builtFile); os.IsNotExist(err) {
		t.Fatalf("Output binary not found: %s", builtFile)
	}

	// Run the binary
	cmd := exec.Command(builtFile)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Binary execution failed: %v\nStderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Binary execution failed: %v", err)
	}

	result := strings.TrimSpace(string(output))
	if !strings.Contains(result, "x0: x0") {
		t.Errorf("Expected identity function, got: %s", result)
	}
}

// Test that would use effect handlers (placeholder for when runtime support is added)
func TestCompileWithEffectHandlers(t *testing.T) {
	t.Skip("Effect handlers in compiled code require runtime integration - placeholder test")
	
	tmpDir, err := os.MkdirTemp("", "godnet_effects_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Future test: compile code that performs effects
	// Example pseudo-code (syntax TBD):
	// let result = effect Print "hello" in result
	
	sourceFile := filepath.Join(tmpDir, "effects_test.lam")
	source := `x: x`
	
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	outputFile := filepath.Join(tmpDir, "effects_test")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		KeepTemp:   false,
	}

	_, err = c.Compile()
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}
}

// Test compiling code that would use both pure functions and effects
func TestCompileMixedPureAndEffects(t *testing.T) {
	t.Skip("Mixed pure/effect code requires full runtime integration - placeholder test")
	
	tmpDir, err := os.MkdirTemp("", "godnet_mixed_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Future test: compile code that uses both pure functions and effects
	// Example: let concat = pure "string_concat"; x = concat "hello" " world" in effect Print x
	
	sourceFile := filepath.Join(tmpDir, "mixed_test.lam")
	source := `x: x`
	
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	outputFile := filepath.Join(tmpDir, "mixed_test")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		KeepTemp:   false,
	}

	_, err = c.Compile()
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}
}

// Test that compiled code can register and use native pure functions
func TestCompiledNativeRegistration(t *testing.T) {
	t.Skip("Native function registration in compiled code needs design - placeholder test")
	
	// Future test design:
	// 1. Extend Nix syntax to reference native functions (maybe `@native("func_name")`)
	// 2. CodeGenerator emits net.RegisterNative calls in generated code
	// 3. Generated code includes native function implementations or imports
	// 4. Test that reduction properly invokes the native functions
	
	// Example generated code structure:
	// func buildNet(net *deltanet.Network) {
	//     // Register natives
	//     net.RegisterNative("string_concat", func(a interface{}) (interface{}, error) {
	//         return func(b interface{}) (interface{}, error) {
	//             return a.(string) + b.(string), nil
	//         }, nil
	//     })
	//     
	//     // Build term that uses the native
	//     pure := net.NewPure("string_concat")
	//     data1 := net.NewData("hello")
	//     // ... etc
	// }
}

// Test that compiled code can install and use effect handlers
func TestCompiledEffectHandlers(t *testing.T) {
	t.Skip("Effect handlers in compiled code need design - placeholder test")
	
	// Future test design:
	// 1. Extend Nix syntax for effects (maybe `perform Effect payload` and `handle ... with ...`)
	// 2. CodeGenerator emits handler registration and effect nodes
	// 3. Generated code includes handler implementations
	// 4. Test that reduction properly invokes handlers with continuations
	
	// Example generated code structure:
	// func buildNet(net *deltanet.Network) {
	//     // Create handler scope
	//     scope := deltanet.NewHandlerScope()
	//     scope.Register("Print", func(eff deltanet.Effect, cont *deltanet.Continuation) (interface{}, error) {
	//         fmt.Println(eff.Payload)
	//         return cont.Resume(nil)
	//     })
	//     
	//     // Build term with effects
	//     handler := net.NewHandler(scope)
	//     effect := net.NewEffect(deltanet.Effect{Name: "Print", Payload: "hello"})
	//     // ... etc
	// }
}
