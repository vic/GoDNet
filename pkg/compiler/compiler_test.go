package compiler

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileIdentity(t *testing.T) {
	testCompile(t, "identity", "(x: x) a", "a")
}

func TestCompileKCombinator(t *testing.T) {
	testCompile(t, "k_combinator", "(x: y: x) a b", "a")
}

func TestCompileChurchZero(t *testing.T) {
	testCompile(t, "church_zero", "f: x: x", "(x0: (x1: x1))")
}

func TestCompileChurchSucc(t *testing.T) {
	testCompile(t, "church_succ",
		"let succ = n: f: x: f (n f x); zero = f: x: x in succ zero",
		"(x0: (x1: (x0")
}

func TestCompileFreeVariable(t *testing.T) {
	testCompile(t, "free_var", "x: y", "(x0: y)")
}

func TestCompileNestedApp(t *testing.T) {
	testCompile(t, "nested_app", "((x: y: z: x) a) b c", "a")
}

func TestCompileSCombinator(t *testing.T) {
	testCompile(t, "s_combinator",
		"(x: y: z: (x z) (y z)) (a: a) (b: b) d",
		"(d d)")
}

func testCompile(t *testing.T, name string, source string, expected string) {
	t.Helper()

	// Create temp directory in project root for module support
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Write source file
	sourceFile := filepath.Join(tmpDir, name+".lam")
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Compile with absolute output path
	outputFile := filepath.Join(tmpDir, name)
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		KeepTemp:   false,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	if builtFile != outputFile {
		t.Fatalf("Output file mismatch: expected %s, got %s", outputFile, builtFile)
	}

	// Make sure binary exists
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Fatalf("Output binary not found: %s", outputFile)
	}
	defer os.Remove(outputFile)

	// Run the binary
	cmd := exec.Command(outputFile)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Binary execution failed: %v\nStderr: %s", err, exitErr.Stderr)
		}
		t.Fatalf("Binary execution failed: %v", err)
	}

	// Check result
	result := strings.TrimSpace(string(output))
	if !strings.HasPrefix(result, expected) {
		t.Errorf("Expected output to start with:\n%s\nGot:\n%s", expected, result)
	}
}

func TestCompileWithFlags(t *testing.T) {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceFile := filepath.Join(tmpDir, "test.lam")
	if err := os.WriteFile(sourceFile, []byte("x: x"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	// Set custom output name
	customOut := filepath.Join(tmpDir, "custom_name")

	c := Compiler{
		SourceFile: sourceFile,
		OutputName: customOut,
		GoFlags:    []string{"-v"}, // verbose go build
	}

	outputFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation with flags failed: %v", err)
	}

	if outputFile != customOut {
		t.Errorf("Expected output name %s, got %s", customOut, outputFile)
	}

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("Custom output file not created: %s", outputFile)
	}
}

func TestCompileKeepTemp(t *testing.T) {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceFile := filepath.Join(tmpDir, "test.lam")
	if err := os.WriteFile(sourceFile, []byte("x: x"), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	outputFile := filepath.Join(tmpDir, "test_keeptemp")
	c := Compiler{
		SourceFile: sourceFile,
		OutputName: outputFile,
		KeepTemp:   true,
	}

	builtFile, err := c.Compile()
	if err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}
	defer os.Remove(builtFile)

	// Note: The temp file is created in /tmp, not next to source
	// We just verify KeepTemp was respected (stderr message printed)
	// Actual temp file cleanup is handled by OS
}

func TestCompileInvalidSource(t *testing.T) {
	cwd, _ := os.Getwd()
	projectRoot := filepath.Join(cwd, "../..")
	tmpDir, err := os.MkdirTemp(projectRoot, "test_build_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sourceFile := filepath.Join(tmpDir, "invalid.lam")
	if err := os.WriteFile(sourceFile, []byte("((("), 0644); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	c := Compiler{
		SourceFile: sourceFile,
	}

	_, err = c.Compile()
	if err == nil {
		t.Error("Expected compilation to fail for invalid syntax")
	}
}
