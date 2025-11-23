package deltanet

import (
	"fmt"
	"testing"
)

// TestStringConcat tests concatenating two strings using native functions
func TestStringConcat(t *testing.T) {
	net := NewNetwork()

	// Register concat native function
	net.RegisterNative("concat", func(a interface{}) (interface{}, error) {
		// This returns a function that captures 'a' (first string)
		s1, ok := a.(string)
		if !ok {
			return nil, fmt.Errorf("concat: first arg must be string, got %T", a)
		}

		// Return a native function that takes the second argument
		return func(b interface{}) (interface{}, error) {
			s2, ok := b.(string)
			if !ok {
				return nil, fmt.Errorf("concat: second arg must be string, got %T", b)
			}
			return s1 + s2, nil
		}, nil
	})

	// Build net structure for: concat "hello" "world"
	// This is: (concat "hello") "world"
	// Structure: Fan(App) where:
	//   Fan.0 -> connects to another Fan(App) for (concat "hello")
	//   Fan.2 -> Data("world")
	//   Fan.1 -> result

	// Inner application: concat "hello"
	innerFan := net.NewFan()
	concatNode := net.NewNative("concat")
	helloData := net.NewData("hello")

	net.Link(innerFan, 0, concatNode, 0) // Function
	net.Link(innerFan, 2, helloData, 0)  // Argument
	// innerFan.1 is the result of (concat "hello")

	// Outer application: (concat "hello") "world"
	outerFan := net.NewFan()
	worldData := net.NewData("world")

	net.Link(outerFan, 0, innerFan, 1)  // Function (result of inner app)
	net.Link(outerFan, 2, worldData, 0) // Argument
	// outerFan.1 is the final result

	// Connect result to output var
	output := net.NewVar()
	net.Link(outerFan, 1, output, 0)

	// Reduce
	net.ReduceAll()

	// Check result
	resultNode, resultPort := net.GetLink(output, 0)
	if resultNode == nil {
		t.Fatal("Result node is nil")
	}
	if resultNode.Type() != NodeTypeData {
		t.Errorf("Expected result to be Data node, got %v", resultNode.Type())
	}

	result := resultNode.GetValue()
	expected := "helloworld"
	if result != expected {
		t.Errorf("Expected %q, got %v", expected, result)
	}

	t.Logf("Result: %v (port %d)", result, resultPort)
}

// TestStringLength tests computing the length of a string
func TestStringLength(t *testing.T) {
	net := NewNetwork()

	// Register length native function
	net.RegisterNative("length", func(v interface{}) (interface{}, error) {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("length: arg must be string, got %T", v)
		}
		return len(s), nil
	})

	// Build net structure for: length "hello"
	// Structure: Fan(App) where:
	//   Fan.0 -> Native("length")
	//   Fan.2 -> Data("hello")
	//   Fan.1 -> result

	fan := net.NewFan()
	lengthNode := net.NewNative("length")
	helloData := net.NewData("hello")

	net.Link(fan, 0, lengthNode, 0) // Function
	net.Link(fan, 2, helloData, 0)  // Argument

	// Connect result to output var
	output := net.NewVar()
	net.Link(fan, 1, output, 0)

	// Reduce
	net.ReduceAll()

	// Check result
	resultNode, resultPort := net.GetLink(output, 0)
	if resultNode == nil {
		t.Fatal("Result node is nil")
	}
	if resultNode.Type() != NodeTypeData {
		t.Errorf("Expected result to be Data node, got %v", resultNode.Type())
	}

	result := resultNode.GetValue()
	expected := 5
	if result != expected {
		t.Errorf("Expected %d, got %v", expected, result)
	}

	t.Logf("Result: %v (port %d)", result, resultPort)
}

// TestConcatLength tests composing concat and length
// length (concat "hello" "world") should return 10
func TestConcatLength(t *testing.T) {
	net := NewNetwork()

	// Register both natives
	net.RegisterNative("concat", func(a interface{}) (interface{}, error) {
		s1, ok := a.(string)
		if !ok {
			return nil, fmt.Errorf("concat: first arg must be string, got %T", a)
		}
		return func(b interface{}) (interface{}, error) {
			s2, ok := b.(string)
			if !ok {
				return nil, fmt.Errorf("concat: second arg must be string, got %T", b)
			}
			return s1 + s2, nil
		}, nil
	})

	net.RegisterNative("length", func(v interface{}) (interface{}, error) {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("length: arg must be string, got %T", v)
		}
		return len(s), nil
	})

	// Build: length (concat "hello" "world")
	// Structure:
	// 1. Inner: concat "hello" -> partial
	// 2. Middle: partial "world" -> "helloworld"
	// 3. Outer: length "helloworld" -> 10

	// Build concat application
	concatInnerFan := net.NewFan()
	concatNode := net.NewNative("concat")
	helloData := net.NewData("hello")
	net.Link(concatInnerFan, 0, concatNode, 0)
	net.Link(concatInnerFan, 2, helloData, 0)

	concatOuterFan := net.NewFan()
	worldData := net.NewData("world")
	net.Link(concatOuterFan, 0, concatInnerFan, 1)
	net.Link(concatOuterFan, 2, worldData, 0)
	// concatOuterFan.1 is "helloworld"

	// Build length application
	lengthFan := net.NewFan()
	lengthNode := net.NewNative("length")
	net.Link(lengthFan, 0, lengthNode, 0)
	net.Link(lengthFan, 2, concatOuterFan, 1) // Apply to concat result

	// Connect result
	output := net.NewVar()
	net.Link(lengthFan, 1, output, 0)

	// Reduce
	net.ReduceAll()

	// Check result
	resultNode, _ := net.GetLink(output, 0)
	if resultNode == nil {
		t.Fatal("Result node is nil")
	}
	if resultNode.Type() != NodeTypeData {
		t.Errorf("Expected result to be Data node, got %v", resultNode.Type())
	}

	result := resultNode.GetValue()
	expected := 10
	if result != expected {
		t.Errorf("Expected %d, got %v", expected, result)
	}

	t.Logf("length (concat \"hello\" \"world\") = %v", result)
}
