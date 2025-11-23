package deltanet

// Effect represents a single algebraic effect to be performed.
type Effect struct {
	Name    string      // Effect name: "Print", "Exception", "State.Get", etc.
	Payload interface{} // Effect-specific data
}

// EffectRow is the set of effects a computation may perform.
// Tracked at value level - surface languages provide type-level tracking.
type EffectRow []string

// Contains checks if an effect is in the row.
func (row EffectRow) Contains(effectName string) bool {
	for _, name := range row {
		if name == effectName {
			return true
		}
	}
	return false
}

// Remove returns a new EffectRow without the specified effect.
func (row EffectRow) Remove(effectName string) EffectRow {
	result := make(EffectRow, 0, len(row))
	for _, name := range row {
		if name != effectName {
			result = append(result, name)
		}
	}
	return result
}

// Union combines two effect rows.
func (row EffectRow) Union(other EffectRow) EffectRow {
	seen := make(map[string]bool)
	result := make(EffectRow, 0, len(row)+len(other))

	for _, name := range row {
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}

	for _, name := range other {
		if !seen[name] {
			result = append(result, name)
			seen[name] = true
		}
	}

	return result
}

// Continuation represents a delimited continuation.
// Can be invoked 0-n times (reentrant).
type Continuation struct {
	capturedState interface{} // Captured computation state
	resume        func(interface{}) (interface{}, error)
}

// Resume invokes the continuation with a value.
// Can be called multiple times (multi-shot continuations).
func (k *Continuation) Resume(value interface{}) (interface{}, error) {
	if k.resume == nil {
		return nil, nil
	}
	return k.resume(value)
}

// EffectHandler interprets an effect and manages its continuation.
// The handler decides how many times to call the continuation: 0, 1, or n times.
//
// Examples:
//   - Exception: call 0 times (short-circuit)
//   - Normal: call 1 time (resume)
//   - Retry: call n times until success
//   - Choice: call n times with different values
type EffectHandler func(effect Effect, resume *Continuation) (interface{}, error)

// HandlerScope manages a set of effect handlers.
// Handlers are applied innermost-first during reduction.
type HandlerScope struct {
	Handlers map[string]EffectHandler // Effect name -> handler
	Handled  EffectRow                // Effects this scope handles
}

// NewHandlerScope creates a new handler scope.
func NewHandlerScope() *HandlerScope {
	return &HandlerScope{
		Handlers: make(map[string]EffectHandler),
		Handled:  make(EffectRow, 0),
	}
}

// Register adds a handler for an effect.
func (hs *HandlerScope) Register(effectName string, handler EffectHandler) {
	hs.Handlers[effectName] = handler
	if !hs.Handled.Contains(effectName) {
		hs.Handled = append(hs.Handled, effectName)
	}
}

// CanHandle checks if this scope handles the given effect.
func (hs *HandlerScope) CanHandle(effectName string) bool {
	_, ok := hs.Handlers[effectName]
	return ok
}

// Handle invokes the handler for an effect.
func (hs *HandlerScope) Handle(effect Effect, resume *Continuation) (interface{}, error) {
	handler, ok := hs.Handlers[effect.Name]
	if !ok {
		return nil, nil // Effect not handled by this scope
	}
	return handler(effect, resume)
}
