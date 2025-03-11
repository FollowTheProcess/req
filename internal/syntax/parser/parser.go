// Package parser implements the http file parser.
package parser

import (
	"fmt"
	"io"

	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/scanner"
	"github.com/FollowTheProcess/req/internal/syntax/token"
)

// Parser is the http file parser.
type Parser struct {
	handler syntax.ErrorHandler // The error handler
	scanner *scanner.Scanner    // Scanner to generate tokens
	name    string              // Name of the file being parsed
	src     []byte              // Raw source text
	current token.Token         // Current token under inspection
	next    token.Token         // Next token in the stream
}

// New returns a new [Parser].
func New(name string, r io.Reader, handler syntax.ErrorHandler) (*Parser, error) {
	// .http files are smol, it's okay to read the whole thing
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from input: %w", err)
	}

	p := &Parser{
		handler: handler,
		name:    name,
		src:     src,
		scanner: scanner.New(name, src, handler),
	}

	// Read 2 tokens so current and next are set
	p.advance()
	p.advance()

	return p, nil
}

// Parse parses the file to completion returning a [syntax.File] and any parsing
// errors encountered.
//
// The returned error will simply signify whether or not there were parse errors,
// the error handler passed to [New] should be preferred.
func (p *Parser) Parse() (syntax.File, error) {
	// TODO(@FollowTheProcess): This
	return syntax.File{}, nil
}

// advance advances the parser by a single token.
func (p *Parser) advance() {
	p.current = p.next
	p.next = p.scanner.Scan()
}

// position returns the parser's current position in the input as a [syntax.Position].
//
// The position is calculated based on the start offset of the current token.
func (p *Parser) position() syntax.Position {
	line := 1              // Line counter
	lastNewLineOffset := 0 // The byte offset of the (end of the) last newline seen
	for index, byt := range p.src {
		if index >= p.current.Start {
			break
		}

		if byt == '\n' {
			lastNewLineOffset = index + 1 // +1 to account for len("\n")
			line++
		}
	}

	// If the next token is EOF, we use the end of the current token as the syntax
	// error is likely to be unexpected EOF so we want to point to the end of the
	// current token as in "something should have gone here"
	start := p.current.Start
	if p.next.Kind == token.EOF {
		start = p.current.End
	}
	end := p.current.End

	// The column is therefore the number of bytes between the end of the last newline
	// and the current position, +1 because editors columns start at 1. Applying this
	// correction here means you can click a glox syntax error in the terminal and be
	// taken to a precise location in an editor which is probably what we want to happen
	startCol := 1 + start - lastNewLineOffset
	endCol := 1 + end - lastNewLineOffset

	return syntax.Position{
		Name:     p.name,
		Line:     line,
		StartCol: startCol,
		EndCol:   endCol,
	}
}

// error calculates the current position and calls the installed error handler
// with the correct information.
func (p *Parser) error(msg string) {
	if p.handler == nil {
		// I guess ignore?
		return
	}

	p.handler(p.position(), msg)
}

// errorf calls error with a formatted message.
func (p *Parser) errorf(format string, a ...any) {
	p.error(fmt.Sprintf(format, a...))
}
