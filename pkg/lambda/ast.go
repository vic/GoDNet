package lambda

import "fmt"

// Term represents a lambda calculus term.
type Term interface {
	String() string
}

// Var represents a variable usage.
type Var struct {
	Name string
}

func (v Var) String() string {
	return v.Name
}

// Abs represents an abstraction (lambda).
type Abs struct {
	Arg  string
	Body Term
}

func (a Abs) String() string {
	return fmt.Sprintf("(%s: %s)", a.Arg, a.Body)
}

// App represents an application.
type App struct {
	Fun Term
	Arg Term
}

func (a App) String() string {
	return fmt.Sprintf("(%s %s)", a.Fun, a.Arg)
}

// Let represents a let binding (sugar for application).
// let x = Val in Body -> (\x. Body) Val
type Let struct {
	Name string
	Val  Term
	Body Term
}

func (l Let) String() string {
	return fmt.Sprintf("let %s = %s; %s", l.Name, l.Val, l.Body)
}
