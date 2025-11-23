package deltanet

import (
	"fmt"
	"testing"
)

// TestAlgebraicEffectBasic tests basic effect performance and handling
func TestAlgebraicEffectBasic(t *testing.T) {
	net := NewNetwork()

	// Create handler scope for Print effect
	scope := NewHandlerScope()
	var printed []string

	scope.Register("Print", func(effect Effect, resume *Continuation) (interface{}, error) {
		msg := effect.Payload.(string)
		printed = append(printed, msg)
		// Resume once with nil (print returns nothing)
		return resume.Resume(nil)
	})

	// Create IO node for Print effect
	printEffect := &Effect{
		Name:    "Print",
		Payload: "Hello, Algebraic Effects!",
	}
	effectRow := EffectRow{"Print"}

	ioNode := net.NewIO(printEffect, effectRow)
	handler := net.NewHandler(scope)

	// Connect: Handler.0 (computation) -> IO node
	net.Link(handler, 0, ioNode, 0)

	// Output var
	output := net.NewVar()
	net.Link(handler, 1, output, 0)

	// For now, we just test structure creation
	if ioNode.Type() != NodeTypeEffect {
		t.Errorf("Expected NodeTypeEffect, got %v", ioNode.Type())
	}

	if handler.Type() != NodeTypeHandler {
		t.Errorf("Expected NodeTypeHandler, got %v", handler.Type())
	}

	eff := ioNode.GetEffect()
	if eff == nil || eff.Name != "Print" {
		t.Errorf("Effect not stored correctly")
	}

	row := ioNode.GetEffectRow()
	if !row.Contains("Print") {
		t.Errorf("Effect row should contain 'Print'")
	}

	t.Logf("Created IO node with effect: %v", eff.Name)
	t.Logf("Effect row: %v", row)
}

// TestEffectRowOperations tests effect row manipulation
func TestEffectRowOperations(t *testing.T) {
	row1 := EffectRow{"Print", "FileRead"}
	row2 := EffectRow{"FileRead", "HTTP"}

	// Test Contains
	if !row1.Contains("Print") {
		t.Error("Should contain Print")
	}
	if row1.Contains("HTTP") {
		t.Error("Should not contain HTTP")
	}

	// Test Remove
	row3 := row1.Remove("Print")
	if row3.Contains("Print") {
		t.Error("Print should be removed")
	}
	if !row3.Contains("FileRead") {
		t.Error("FileRead should remain")
	}

	// Test Union
	row4 := row1.Union(row2)
	if !row4.Contains("Print") || !row4.Contains("FileRead") || !row4.Contains("HTTP") {
		t.Error("Union should contain all effects")
	}

	t.Logf("row1: %v", row1)
	t.Logf("row1.Remove(Print): %v", row3)
	t.Logf("row1.Union(row2): %v", row4)
}

// TestHandlerScope tests handler registration and lookup
func TestHandlerScope(t *testing.T) {
	scope := NewHandlerScope()

	scope.Register("TestEffect", func(effect Effect, resume *Continuation) (interface{}, error) {
		return resume.Resume("handled")
	})

	if !scope.CanHandle("TestEffect") {
		t.Error("Should be able to handle TestEffect")
	}

	if scope.CanHandle("NonExistent") {
		t.Error("Should not handle non-existent effect")
	}

	if !scope.Handled.Contains("TestEffect") {
		t.Error("Handled row should contain TestEffect")
	}

	t.Logf("Handler scope created with effects: %v", scope.Handled)
}

// TestContinuationReentrant tests multi-shot continuations
func TestContinuationReentrant(t *testing.T) {
	// Create a continuation that can be called multiple times
	callCount := 0
	k := &Continuation{
		capturedState: "initial",
		resume: func(v interface{}) (interface{}, error) {
			callCount++
			return fmt.Sprintf("call_%d:%v", callCount, v), nil
		},
	}

	// Call continuation multiple times (multi-shot)
	r1, _ := k.Resume("first")
	r2, _ := k.Resume("second")
	r3, _ := k.Resume("third")

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got %d", callCount)
	}

	t.Logf("Continuation results: %v, %v, %v", r1, r2, r3)
}

// TestExceptionHandlerZeroShot tests exception handling (0-shot continuation)
func TestExceptionHandlerZeroShot(t *testing.T) {
	net := NewNetwork()

	// Exception handler that short-circuits (doesn't call continuation)
	scope := NewHandlerScope()

	scope.Register("Exception", func(effect Effect, resume *Continuation) (interface{}, error) {
		// DON'T call resume - short circuit!
		return effect.Payload, nil
	})

	// Create exception effect
	exceptionEffect := &Effect{
		Name:    "Exception",
		Payload: "Error: Something went wrong!",
	}

	ioNode := net.NewIO(exceptionEffect, EffectRow{"Exception"})

	if ioNode.GetEffect().Name != "Exception" {
		t.Error("Effect name should be Exception")
	}

	// Test that handler is set up correctly
	if !scope.CanHandle("Exception") {
		t.Error("Should handle Exception")
	}

	t.Logf("Exception handler registered (0-shot continuation)")
}

// TestRetryHandlerMultiShot tests retry logic (n-shot continuation)
func TestRetryHandlerMultiShot(t *testing.T) {
	// Retry handler that calls continuation up to n times
	attemptCount := 0

	retryHandler := func(effect Effect, resume *Continuation) (interface{}, error) {
		maxRetries := effect.Payload.(int)

		for i := 0; i < maxRetries; i++ {
			attemptCount++
			result, err := resume.Resume(attemptCount)
			if err == nil {
				return result, nil
			}
			// Continue retrying
		}

		return nil, fmt.Errorf("max retries exceeded")
	}

	// Simulate a retry effect
	k := &Continuation{
		resume: func(v interface{}) (interface{}, error) {
			attempt := v.(int)
			if attempt < 3 {
				return nil, fmt.Errorf("attempt %d failed", attempt)
			}
			return "success", nil
		},
	}

	retryEffect := &Effect{
		Name:    "Retry",
		Payload: 5, // Max 5 retries
	}

	result, err := retryHandler(*retryEffect, k)

	if err != nil {
		t.Errorf("Should succeed after retries: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}

	t.Logf("Retry succeeded after %d attempts", attemptCount)
}

// TestChoiceHandlerMultiShot tests non-deterministic choice (multiple calls)
func TestChoiceHandlerMultiShot(t *testing.T) {
	// Choice handler explores all branches
	choiceHandler := func(effect Effect, resume *Continuation) (interface{}, error) {
		choices := effect.Payload.([]int)
		var results []interface{}

		// Call continuation with each choice
		for _, choice := range choices {
			result, err := resume.Resume(choice)
			if err == nil {
				results = append(results, result)
			}
		}

		return results, nil
	}

	// Continuation that processes each choice
	k := &Continuation{
		resume: func(v interface{}) (interface{}, error) {
			choice := v.(int)
			return choice * 2, nil // Double each choice
		},
	}

	choiceEffect := &Effect{
		Name:    "Choice",
		Payload: []int{1, 2, 3, 4},
	}

	result, err := choiceHandler(*choiceEffect, k)

	if err != nil {
		t.Errorf("Choice handler failed: %v", err)
	}

	results := result.([]interface{})
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	t.Logf("Choice results: %v", results)
}
