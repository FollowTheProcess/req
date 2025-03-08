// Package scanner implements the lexical scanner for .http files.
package scanner

import (
	"fmt"
	"io"
	"unicode"
	"unicode/utf8"

	"github.com/FollowTheProcess/req/internal/syntax/token"
)

const (
	eof = rune(-1) // eof signifies we have reached the end of the input
	bom = 0xFEFF   // byte order mark, only permitted as very first character
)

// An ErrorHandler may be provided to the [Scanner]. If a syntax error is encountered and
// a non-nil handler was provided, it is called with the position info and error message.
type ErrorHandler func(pos Position, msg string)

// Scanner is the http file scanner.
type Scanner struct {
	handler   ErrorHandler // The error handler, if any
	name      string       // Name of the file
	src       []byte       // Raw source text
	start     int          // The start position of the current token
	pos       int          // Current scanner position in src (bytes, 0 indexed)
	line      int          // Current line number (1 indexed)
	lineStart int          // Offset at which the current line began
	char      rune         // The character the scanner is currently sat on
}

// New returns a new [Scanner] that reads from r.
func New(name string, r io.Reader, handler ErrorHandler) (*Scanner, error) {
	// .http files are small, it's fine to just read it in one go
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not read from input: %w", err)
	}

	s := &Scanner{
		name:    name,
		src:     src,
		line:    1,
		handler: handler,
		char:    ' ',
	}

	// Read the first char, and ignore it if it's the bom
	s.advance()
	if s.char == bom {
		s.advance()
		s.start = s.pos
	}

	return s, nil
}

// Scan scans the input and returns the next token.
func (s *Scanner) Scan() token.Token {
	char := s.char // The current char before advancing
	s.advance()

	switch char {
	case eof:
		return s.token(token.EOF)
	case '#':
		return s.scanComment()
	case '/':
		return s.scanComment()
	default:
		if unicode.IsLetter(char) {
			return s.scanText()
		}
		s.errorf("unexpected char %q", char)
		return s.token(token.Error)
	}
}

// advance advances the scanner by a single character.
func (s *Scanner) advance() {
	if s.pos >= len(s.src) {
		s.char = eof
		s.pos = len(s.src)
		return
	}

	char, width := utf8.DecodeRune(s.src[s.pos:])
	if char == utf8.RuneError {
		s.errorf("invalid utf8 char: %U", char)
	}

	if char == '\n' {
		s.line++
		s.lineStart = s.pos
	}

	// Move the scanner forward
	s.pos += width
	s.char = char
}

// advanceWhile advances the scanner as long as the predicate returns true, stopping at
// the first character for which it returns false.
func (s *Scanner) advanceWhile(predicate func(r rune) bool) {
	for predicate(s.char) {
		s.advance()
	}
}

// token returns a token of a particular kind, using the scanner state
// to fill in position info.
func (s *Scanner) token(kind token.Kind) token.Token {
	tok := token.Token{Kind: kind, Start: s.start, End: s.pos}

	// Bring start up to pos
	s.start = s.pos
	return tok
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
		Line:     s.line,
		StartCol: startCol,
		EndCol:   endCol,
	}

	s.handler(position, msg)
}

// errorf calls error with a formatted message.
func (s *Scanner) errorf(format string, a ...any) {
	s.error(fmt.Sprintf(format, a...))
}

// scanText scans a string of continuous characters, stopping at the first
// whitespace character.
func (s *Scanner) scanText() token.Token {
	for !unicode.IsSpace(s.char) && s.char != eof {
		s.advance()
	}

	text := string(s.src[s.start:s.pos])
	kind, method := token.Method(text)
	if method {
		return s.token(kind)
	}

	return s.token(token.Text)
}

// scanComment scans either a '#' or '//' style comment.
//
// The '#' or first '/' has been consumed.
func (s *Scanner) scanComment() token.Token {
	s.advanceWhile(isLineSpace)

	// Consume until the end of the line or eof
	for s.char != '\n' && s.char != eof {
		s.advance()
	}

	tok := s.token(token.Comment)

	// Any trailing space after the comment doesn't matter
	s.advanceWhile(unicode.IsSpace)

	return tok
}

// isLineSpace reports whether r is a non line terminating whitespace. Imagine
// [unicode.IsSpace] but without '\n' and '\r\n'.
func isLineSpace(r rune) bool {
	return r == ' ' || r == '\t'
}
