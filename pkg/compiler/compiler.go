package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vic/godnet/pkg/lambda"
)

// Compiler translates lambda terms to Go code and invokes go build.
type Compiler struct {
	SourceFile string
	OutputName string
	GoFlags    []string // Passed directly to go build
	KeepTemp   bool     // For debugging
}

// Compile translates the lambda source to Go code and builds it.
// Returns the output binary path on success.
func (c *Compiler) Compile() (string, error) {
	// Read and parse source
	source, err := os.ReadFile(c.SourceFile)
	if err != nil {
		return "", fmt.Errorf("failed to read source: %w", err)
	}

	term, err := lambda.Parse(string(source))
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	// Generate Go code
	gen := CodeGenerator{
		SourceFile: c.SourceFile,
		SourceText: string(source),
	}
	goCode := gen.Generate(term)

	// Write to temporary file
	tmpFile, err := os.CreateTemp("", "godnet-*.go")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if !c.KeepTemp {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.WriteString(goCode); err != nil {
		return "", fmt.Errorf("failed to write generated code: %w", err)
	}
	tmpFile.Close()

	// Determine output name
	outputName := c.OutputName
	if outputName == "" {
		// Default: strip .lam extension
		outputName = strings.TrimSuffix(filepath.Base(c.SourceFile), filepath.Ext(c.SourceFile))
	}

	// Build with go build
	args := []string{"build", "-o", outputName}
	args = append(args, c.GoFlags...)
	args = append(args, tmpPath)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build failed: %w", err)
	}

	if c.KeepTemp {
		fmt.Fprintf(os.Stderr, "Generated code kept at: %s\n", tmpPath)
	}

	return outputName, nil
}
