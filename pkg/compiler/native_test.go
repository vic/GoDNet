package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Test compiling with additional Go files that provide native functions
func TestCompileWithNativeFunctions(t *testing.T) {
	// Create temp dir in a test subdirectory within the project
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a Go file with native function implementations
	nativeGoFile := filepath.Join(tmpDir, "natives.go")
	nativeGoCode := `package main

import "github.com/vic/godnet/pkg/deltanet"

func registerNatives(net *deltanet.Network) {
	// String concatenation
	net.RegisterNative("str_concat", func(a interface{}) (interface{}, error) {
		aStr, ok := a.(string)
		if !ok {
			return nil, nil // Return nil for non-string
		}
		return func(b interface{}) (interface{}, error) {
			bStr, ok := b.(string)
			if !ok {
				return nil, nil
			}
			return aStr + bStr, nil
		}, nil
	})
	
	// String length
	net.RegisterNative("str_len", func(a interface{}) (interface{}, error) {
		aStr, ok := a.(string)
		if !ok {
			return 0, nil
		}
		return len(aStr), nil
	})
}
`
	if err := os.WriteFile(nativeGoFile, []byte(nativeGoCode), 0644); err != nil {
		t.Fatalf("Failed to write natives.go: %v", err)
	}

	// Create lambda source that references these natives
	sourceFile := filepath.Join(tmpDir, "test.lam")
	source := `x: x` // Simple test - actual native invocation syntax TBD

	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile with the native Go file included (must be in same directory)
	outputFile := filepath.Join(tmpDir, "test_with_natives")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		GoFlags:    []string{nativeGoFile}, // Will be copied to output dir
		KeepTemp:   false,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation with natives failed: %v", err)
	}

	if _, err := os.Stat(builtFile); os.IsNotExist(err) {
		t.Fatalf("Output binary not found: %s", builtFile)
	}

	// Verify binary runs
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

// Test compiling with additional Go files that provide effect handlers
func TestCompileWithEffectHandlers(t *testing.T) {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a Go file with effect handler implementations
	handlersGoFile := filepath.Join(tmpDir, "handlers.go")
	handlersGoCode := `package main

import (
	"fmt"
	"github.com/vic/godnet/pkg/deltanet"
)

func installHandlers(net *deltanet.Network) *deltanet.HandlerScope {
	scope := deltanet.NewHandlerScope()
	
	// Print effect handler
	scope.Register("Print", func(eff deltanet.Effect, cont *deltanet.Continuation) (interface{}, error) {
		fmt.Println(eff.Payload)
		return cont.Resume(nil)
	})
	
	// FileRead effect handler (mock)
	scope.Register("FileRead", func(eff deltanet.Effect, cont *deltanet.Continuation) (interface{}, error) {
		path, ok := eff.Payload.(string)
		if !ok {
			return cont.Resume("invalid path")
		}
		// Mock file read
		content := fmt.Sprintf("contents of %s", path)
		return cont.Resume(content)
	})
	
	return scope
}
`
	if err := os.WriteFile(handlersGoFile, []byte(handlersGoCode), 0644); err != nil {
		t.Fatalf("Failed to write handlers.go: %v", err)
	}

	// Create lambda source
	sourceFile := filepath.Join(tmpDir, "test.lam")
	source := `x: x` // Simple test - actual effect syntax TBD

	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile with the handlers Go file included
	outputFile := filepath.Join(tmpDir, "test_with_handlers")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		GoFlags:    []string{handlersGoFile}, // Link with handlers.go
		KeepTemp:   false,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation with handlers failed: %v", err)
	}

	if _, err := os.Stat(builtFile); os.IsNotExist(err) {
		t.Fatalf("Output binary not found: %s", builtFile)
	}

	// Verify binary runs
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

// Test compiling with both native functions and effect handlers
func TestCompileMixedNativesAndHandlers(t *testing.T) {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create Go file with both natives and handlers
	runtimeGoFile := filepath.Join(tmpDir, "runtime.go")
	runtimeGoCode := `package main

import (
	"fmt"
	"github.com/vic/godnet/pkg/deltanet"
)

func setupRuntime(net *deltanet.Network) *deltanet.HandlerScope {
	// Register native functions
	net.RegisterNative("add", func(a interface{}) (interface{}, error) {
		aInt, ok := a.(int)
		if !ok {
			return nil, fmt.Errorf("add: expected int")
		}
		return func(b interface{}) (interface{}, error) {
			bInt, ok := b.(int)
			if !ok {
				return nil, fmt.Errorf("add: expected int")
			}
			return aInt + bInt, nil
		}, nil
	})
	
	// Register effect handlers
	scope := deltanet.NewHandlerScope()
	scope.Register("Log", func(eff deltanet.Effect, cont *deltanet.Continuation) (interface{}, error) {
		fmt.Printf("[LOG] %v\n", eff.Payload)
		return cont.Resume(nil)
	})
	
	return scope
}
`
	if err := os.WriteFile(runtimeGoFile, []byte(runtimeGoCode), 0644); err != nil {
		t.Fatalf("Failed to write runtime.go: %v", err)
	}

	// Create lambda source
	sourceFile := filepath.Join(tmpDir, "test.lam")
	source := `x: x` // Simple test - actual native/effect syntax TBD

	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile with runtime file
	outputFile := filepath.Join(tmpDir, "test_mixed")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		GoFlags:    []string{runtimeGoFile}, // Link with runtime.go
		KeepTemp:   false,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if _, err := os.Stat(builtFile); os.IsNotExist(err) {
		t.Fatalf("Output binary not found: %s", builtFile)
	}

	// Verify binary runs
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

// Test multiple Go files can be linked together
func TestCompileWithMultipleRuntimeFiles(t *testing.T) {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create separate files for natives and handlers
	nativesFile := filepath.Join(tmpDir, "natives.go")
	nativesCode := `package main

import "github.com/vic/godnet/pkg/deltanet"

func setupNatives(net *deltanet.Network) {
	net.RegisterNative("mul", func(a interface{}) (interface{}, error) {
		aInt := a.(int)
		return func(b interface{}) (interface{}, error) {
			bInt := b.(int)
			return aInt * bInt, nil
		}, nil
	})
}
`
	if err := os.WriteFile(nativesFile, []byte(nativesCode), 0644); err != nil {
		t.Fatalf("Failed to write natives: %v", err)
	}

	handlersFile := filepath.Join(tmpDir, "handlers.go")
	handlersCode := `package main

import (
	"fmt"
	"github.com/vic/godnet/pkg/deltanet"
)

func setupHandlers() *deltanet.HandlerScope {
	scope := deltanet.NewHandlerScope()
	scope.Register("Debug", func(eff deltanet.Effect, cont *deltanet.Continuation) (interface{}, error) {
		fmt.Printf("[DEBUG] %v\n", eff.Payload)
		return cont.Resume(nil)
	})
	return scope
}
`
	if err := os.WriteFile(handlersFile, []byte(handlersCode), 0644); err != nil {
		t.Fatalf("Failed to write handlers: %v", err)
	}

	// Create lambda source
	sourceFile := filepath.Join(tmpDir, "test.lam")
	source := `x: x`

	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source: %v", err)
	}

	// Compile with multiple runtime files
	outputFile := filepath.Join(tmpDir, "test_multi")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		GoFlags:    []string{nativesFile, handlersFile},
		KeepTemp:   false,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation with multiple files failed: %v", err)
	}

	if _, err := os.Stat(builtFile); os.IsNotExist(err) {
		t.Fatalf("Output binary not found: %s", builtFile)
	}

	// Verify binary runs
	cmd := exec.Command(builtFile)
	if output, err := cmd.Output(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Binary failed: %v\nStderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Binary failed: %v", err)
	} else {
		result := strings.TrimSpace(string(output))
		if !strings.Contains(result, "x0: x0") {
			t.Errorf("Expected identity, got: %s", result)
		}
	}
}
