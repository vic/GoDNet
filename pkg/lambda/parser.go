package lambda

import (
	"fmt"
	"unicode"
)

type TokenType int

const (
	TokenEOF TokenType = iota
	TokenIdent
	TokenColon
	TokenEqual
	TokenSemicolon
	TokenLParen
	TokenRParen
	TokenLet
	TokenIn
)

type Token struct {
	Type    TokenType
	Literal string
}

type Parser struct {
	input   string
	pos     int
	current Token
}

func NewParser(input string) *Parser {
	p := &Parser{input: input}
	p.next()
	return p
}

func (p *Parser) next() {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		p.current = Token{Type: TokenEOF}
		return
	}

	ch := p.input[p.pos]
	switch {
	case isLetter(ch):
		start := p.pos
		for p.pos < len(p.input) && (isLetter(p.input[p.pos]) || isDigit(p.input[p.pos])) {
			p.pos++
		}
		lit := p.input[start:p.pos]
		if lit == "let" {
			p.current = Token{Type: TokenLet, Literal: lit}
		} else if lit == "in" {
			p.current = Token{Type: TokenIn, Literal: lit}
		} else {
			p.current = Token{Type: TokenIdent, Literal: lit}
		}
	case ch == ':':
		p.current = Token{Type: TokenColon, Literal: ":"}
		p.pos++
	case ch == '=':
		p.current = Token{Type: TokenEqual, Literal: "="}
		p.pos++
	case ch == ';':
		p.current = Token{Type: TokenSemicolon, Literal: ";"}
		p.pos++
	case ch == '(':
		p.current = Token{Type: TokenLParen, Literal: "("}
		p.pos++
	case ch == ')':
		p.current = Token{Type: TokenRParen, Literal: ")"}
		p.pos++
	default:
		// Treat unknown chars as identifiers for now (e.g. +)
		// Or maybe just single char symbols
		p.current = Token{Type: TokenIdent, Literal: string(ch)}
		p.pos++
	}
}

func (p *Parser) skipWhitespace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func (p *Parser) Parse() (Term, error) {
	return p.parseTerm()
}

// Term ::= Abs | Let | App
func (p *Parser) parseTerm() (Term, error) {
	if p.current.Type == TokenLet {
		return p.parseLet()
	}

	// Try to parse an abstraction or application
	// Since application is left-associative and abstraction extends to the right,
	// we need to be careful.
	// Nix syntax: x: Body
	// App: M N

	// We parse a list of "atoms" and combine them as application.
	// If we see an identifier followed by colon, it's an abstraction.
	// But we need lookahead or backtracking.
	// Actually, `x: ...` starts with ident then colon.
	// `x y` starts with ident then ident.

	// Let's parse "Atom" first.
	// Atom ::= Ident | ( Term )

	// If current is Ident:
	// Check next token. If Colon, it's Abs.
	// Else, it's an Atom (Var), and we continue parsing more Atoms for App.

	if p.current.Type == TokenIdent {
		// Lookahead
		savePos := p.pos
		saveTok := p.current

		// Peek next
		p.next()
		if p.current.Type == TokenColon {
			// It's an abstraction
			arg := saveTok.Literal
			p.next() // consume colon
			body, err := p.parseTerm()
			if err != nil {
				return nil, err
			}
			return Abs{Arg: arg, Body: body}, nil
		}

		// Not an abstraction, backtrack
		p.pos = savePos
		p.current = saveTok
	}

	return p.parseApp()
}

func (p *Parser) parseApp() (Term, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}

	for {
		if p.current.Type == TokenEOF || p.current.Type == TokenRParen || p.current.Type == TokenSemicolon || p.current.Type == TokenIn {
			break
		}
		// Also stop if we see something that looks like the start of an abstraction?
		// `x: ...` inside an app? `(x: x) y` is valid. `x y: z` -> `x (y: z)`?
		// Usually lambda extends as far right as possible.
		// So `x y: z` parses as `x (y: z)`.
		// If we see Ident Colon, we should parse it as Abs and append to App.

		if p.current.Type == TokenIdent {
			// Check for colon
			savePos := p.pos
			saveTok := p.current
			p.next()
			if p.current.Type == TokenColon {
				// It's an abstraction `arg: body`
				// This abstraction is the argument to the current application
				argName := saveTok.Literal
				p.next() // consume colon
				body, err := p.parseTerm()
				if err != nil {
					return nil, err
				}
				left = App{Fun: left, Arg: Abs{Arg: argName, Body: body}}
				// After parsing an abstraction (which consumes everything to the right),
				// we are done with this application chain?
				// Yes, because `x y: z a` -> `x (y: z a)`.
				return left, nil
			}
			// Backtrack
			p.pos = savePos
			p.current = saveTok
		}

		right, err := p.parseAtom()
		if err != nil {
			// If we can't parse an atom, maybe we are done
			break
		}
		left = App{Fun: left, Arg: right}
	}

	return left, nil
}

func (p *Parser) parseAtom() (Term, error) {
	switch p.current.Type {
	case TokenIdent:
		name := p.current.Literal
		p.next()
		return Var{Name: name}, nil
	case TokenLParen:
		p.next()
		term, err := p.parseTerm()
		if err != nil {
			return nil, err
		}
		if p.current.Type != TokenRParen {
			return nil, fmt.Errorf("expected ')'")
		}
		p.next()
		return term, nil
	default:
		return nil, fmt.Errorf("unexpected token: %v", p.current)
	}
}

func (p *Parser) parseLet() (Term, error) {
	p.next() // consume 'let'

	// Parse bindings: x = M; y = N; ...
	type binding struct {
		name string
		val  Term
	}
	var bindings []binding

	for {
		if p.current.Type != TokenIdent {
			return nil, fmt.Errorf("expected identifier in let binding")
		}
		name := p.current.Literal
		p.next()

		if p.current.Type != TokenEqual {
			return nil, fmt.Errorf("expected '='")
		}
		p.next()

		val, err := p.parseTerm()
		if err != nil {
			return nil, err
		}

		bindings = append(bindings, binding{name, val})

		if p.current.Type == TokenSemicolon {
			p.next()
			// Check if next is 'in' or another ident
			if p.current.Type == TokenIn {
				p.next()
				break
			}
			// Continue to next binding
		} else if p.current.Type == TokenIn {
			p.next()
			break
		} else {
			return nil, fmt.Errorf("expected ';' or 'in'")
		}
	}

	body, err := p.parseTerm()
	if err != nil {
		return nil, err
	}

	// Desugar: let x=M; y=N in B -> (\x. (\y. B) N) M
	// We iterate backwards
	term := body
	for i := len(bindings) - 1; i >= 0; i-- {
		b := bindings[i]
		term = App{
			Fun: Abs{Arg: b.name, Body: term},
			Arg: b.val,
		}
	}

	return term, nil
}

// Parse parses a lambda term from a string.
func Parse(input string) (Term, error) {
	p := NewParser(input)
	return p.Parse()
}
