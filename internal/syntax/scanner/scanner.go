// Package scanner implements the lexical scanner for .http files.
package scanner

import (
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/FollowTheProcess/req/internal/syntax/token"
)

const (
	eof = rune(-1) // eof signifies we have reached the end of the input
)

// scanFn represents the state of the scanner as a function that returns the next state.
type scanFn func(*Scanner) scanFn

// An ErrorHandler may be provided to the [Scanner]. If a syntax error is encountered and
// a non-nil handler was provided, it is called with the position info and error message.
type ErrorHandler func(pos Position, msg string)

// Scanner is the http file scanner.
type Scanner struct {
	handler   ErrorHandler     // The error handler, if any
	tokens    chan token.Token // Channel on which to emit scanned tokens
	name      string           // Name of the file
	src       []byte           // Raw source text
	start     int              // The start position of the current token
	pos       int              // Current scanner position in src (bytes, 0 indexed)
	line      int              // Current line number (1 indexed)
	lineStart int              // Offset at which the current line started
	width     int              // Width of the last rune read from input, so we can backup
}

// New returns a new [Scanner] that reads from r.
func New(name string, r io.Reader, handler ErrorHandler) (*Scanner, error) {
	// .http files are small, it's fine to just read it in one go
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not read from input: %w", err)
	}

	// TODO(@FollowTheProcess): Benchmark to find the right buffer size for the channel

	s := &Scanner{
		handler: handler,
		tokens:  make(chan token.Token),
		name:    name,
		src:     src,
		start:   0,
		pos:     0,
		line:    1,
		width:   0,
	}

	go s.run()
	return s, nil
}

// Scan scans the input and returns the next token.
func (s *Scanner) Scan() token.Token {
	return <-s.tokens
}

// advance returns, and consumes, the next character in the input or [eof].
func (s *Scanner) advance() rune {
	if s.pos >= len(s.src) {
		s.pos = len(s.src)
		return eof
	}

	char, width := utf8.DecodeRune(s.src[s.pos:])
	if char == utf8.RuneError {
		s.errorf("invalid utf8 char: %U", char)
		// Advance to the end to prevent cascade errors
		s.pos = len(s.src)
		return eof
	}

	s.width = width
	s.pos += width
	if char == '\n' {
		s.line++
	}

	return char
}

// peek returns, but does not consume, the next character in the input or [eof].
func (s *Scanner) peek() rune {
	if s.pos >= len(s.src) {
		return eof
	}

	char, _ := utf8.DecodeRune(s.src[s.pos:])
	if char == utf8.RuneError {
		s.errorf("invalid utf8 char: %U", char)
		// Advance to the end to prevent cascade errors
		s.pos = len(s.src)
		return eof
	}

	return char
}

// char returns the character the scanner is currently sat on or [eof].
func (s *Scanner) char() rune {
	if s.pos >= len(s.src) {
		return eof
	}
	return rune(s.src[s.pos])
}

// emit passes a token over the tokens channel, using the scanner's internal
// state to populate position information.
func (s *Scanner) emit(kind token.Kind) {
	s.tokens <- token.Token{
		Kind:  kind,
		Start: s.start,
		End:   s.pos,
	}
	s.start = s.pos
}

// run starts the state machine for the scanner, it runs with each [scanFn] returning the next
// state until one returns nil (typically an error or eof), at which point the tokens channel
// is closed as a signal to the receiver that no more tokens will be sent.
func (s *Scanner) run() {
	for state := scanStart; state != nil; {
		state = state(s)
	}
	close(s.tokens)
}

// scanStart is the initial state of the scanner.
func scanStart(s *Scanner) scanFn {
	switch s.advance() {
	case eof:
		s.emit(token.EOF)
		return nil // Break the state machine
	default:
		s.errorf("unexpected token %q", string(s.char()))
		return nil
	}
}

// error calls the installed error handler using information from the state
// of the scanner to populate the error message.
func (s *Scanner) error(msg string) {
	if s.handler == nil {
		// I guess just ignore the error?
		return
	}

	// Column is the number of bytes between the last newline and the current position
	// +1 because columns are 1 indexed
	startCol := 1 + s.start - s.lineStart
	endCol := 1 + s.pos - s.lineStart

	position := Position{
		Name:     s.name,
		Line:     s.lineStart,
		StartCol: startCol,
		EndCol:   endCol,
	}

	s.handler(position, msg)
}

// errorf calls error with a formatted message.
func (s *Scanner) errorf(format string, a ...any) {
	s.error(fmt.Sprintf(format, a...))
}
