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

	// Determine output name first (needed for temp file location)
	outputName := c.OutputName
	if outputName == "" {
		// Default: strip .lam extension
		outputName = strings.TrimSuffix(filepath.Base(c.SourceFile), filepath.Ext(c.SourceFile))
	}

	// Write to temporary file in same directory as output (required by go build)
	outputDir := filepath.Dir(outputName)
	if outputDir == "." || outputDir == "" {
		outputDir, _ = os.Getwd()
	}

	tmpFile, err := os.CreateTemp(outputDir, "godnet-*.go")
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

	// Copy user-provided .go files to output directory (required by go build)
	var copiedFiles []string
	for _, flag := range c.GoFlags {
		if strings.HasSuffix(flag, ".go") {
			srcData, err := os.ReadFile(flag)
			if err != nil {
				return "", fmt.Errorf("failed to read %s: %w", flag, err)
			}
			dstPath := filepath.Join(outputDir, filepath.Base(flag))
			if err := os.WriteFile(dstPath, srcData, 0644); err != nil {
				return "", fmt.Errorf("failed to copy %s: %w", flag, err)
			}
			copiedFiles = append(copiedFiles, dstPath)
			defer func(path string) {
				if !c.KeepTemp {
					os.Remove(path)
				}
			}(dstPath)
		}
	}

	// Find go.mod directory to set module context
	goModDir := findGoModDir(c.SourceFile)

	// Build with go build
	buildDir := outputDir
	if goModDir != "" {
		// If we found go.mod, build from module root for proper dependency resolution
		buildDir = goModDir
	}

	args := []string{"build", "-o", outputName}

	// Add non-.go flags
	for _, flag := range c.GoFlags {
		if !strings.HasSuffix(flag, ".go") {
			args = append(args, flag)
		}
	}

	// Add all Go files (use full paths if building from different directory)
	if buildDir != outputDir {
		args = append(args, tmpPath)
		args = append(args, copiedFiles...)
	} else {
		args = append(args, filepath.Base(tmpPath))
		for _, copied := range copiedFiles {
			args = append(args, filepath.Base(copied))
		}
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = buildDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Debug: show what we're running
	if c.KeepTemp {
		fmt.Fprintf(os.Stderr, "Build dir: %s\n", buildDir)
		fmt.Fprintf(os.Stderr, "Build cmd: go %s\n", strings.Join(args, " "))
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go build failed: %w", err)
	}

	// Return absolute path to output
	if !filepath.IsAbs(outputName) {
		outputName = filepath.Join(outputDir, filepath.Base(outputName))
	}

	if c.KeepTemp {
		fmt.Fprintf(os.Stderr, "Generated code kept at: %s\n", tmpPath)
	}

	return outputName, nil
}

// findGoModDir searches for go.mod starting from the given path
func findGoModDir(startPath string) string {
	dir := filepath.Dir(startPath)
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // Reached root
		}
		dir = parent
	}

	return "" // Not found
}
