// Package scanner implements the lexical scanner for .http files.
package scanner

import (
	"fmt"
	"io"
	"sync"
	"unicode"
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
	wg        sync.WaitGroup   // handler gets run in a goroutine so it doesn't block the main state machine
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

	// run terminates when the scanning state machine is finished and all the tokens
	// drained from s.tokens so no wg.Add needed here
	go s.run()
	return s, nil
}

// Scan scans the input and returns the next token.
func (s *Scanner) Scan() token.Token {
	return <-s.tokens
}

// advance returns, and consumes, the next character in the input or [eof].
func (s *Scanner) advance() rune { //nolint: unparam // We will use this, just not yet
	if s.pos >= len(s.src) {
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
		s.lineStart = s.pos
	}

	return char
}

// peek returns, but does not consume, the next character in the input or [eof].
func (s *Scanner) peek() rune {
	if s.pos >= len(s.src) {
		return eof
	}

	_, width := utf8.DecodeRune(s.src[s.pos:])

	peekPos := s.pos + width
	if peekPos >= len(s.src) {
		return eof
	}

	peekChar, _ := utf8.DecodeRune(s.src[peekPos:])

	return peekChar
}

// char returns the character the scanner is currently sat on or [eof].
func (s *Scanner) char() rune {
	if s.pos >= len(s.src) {
		return eof
	}
	char, _ := utf8.DecodeRune(s.src[s.pos:])
	return char
}

// skip consumes any characters for which the predicate returns true, stopping at the
// first one that returns false such that after it returns, s.advance returns the
// first 'false' char.
func (s *Scanner) skip(predicate func(r rune) bool) {
	for predicate(s.char()) {
		s.advance()
	}
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
	s.tokens <- token.Token{Kind: token.EOF, Start: s.pos, End: s.pos}
	close(s.tokens)

	s.wg.Wait() // Ensure we wait for error handlers to finish
}

// error calculates the position information and arranges for s.handler to be called
// with the information.
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

	s.wg.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		s.handler(position, msg)
	}(&s.wg)
}

// errorf calls error with a formatted message.
func (s *Scanner) errorf(format string, a ...any) {
	s.error(fmt.Sprintf(format, a...))
}

// scanStart is the initial state of the scanner.
func scanStart(s *Scanner) scanFn {
	switch char := s.char(); char {
	case eof:
		return nil // Break the state machine
	case '#':
		return scanHash
	case '/':
		return scanSlash
	default:
		if unicode.IsLetter(char) {
			return scanText
		}
		s.emit(token.Error)
		s.errorf("unexpected token %q", string(s.char()))
		return nil
	}
}

// scanHash scans a '#' character.
func scanHash(s *Scanner) scanFn {
	if s.peek() == '#' {
		return scanRequestSep
	}

	s.advance() // Consume the '#'

	// Ignore any (non line terminating) whitespace between the
	// '#' and the comment text
	s.skip(isLineSpace)

	// Now absorb any text until the the end of the line or eof
	for s.char() != '\n' && s.char() != eof {
		s.advance()
	}

	s.emit(token.Comment)
	s.skip(unicode.IsSpace) // Whitespace after a comment doesn't matter
	return scanStart
}

// scanSlash scans a '/' character.
func scanSlash(s *Scanner) scanFn {
	if s.peek() != '/' {
		return scanStart
	}

	// It's a '//' style comment, consume both '//'
	s.advance()
	s.advance()

	// Ignore any (non line terminating) whitespace between the
	// '//' and the comment text
	s.skip(isLineSpace)

	// Now absorb any text until the the end of the line or eof
	for s.char() != '\n' && s.char() != eof {
		s.advance()
	}

	s.emit(token.Comment)
	s.skip(unicode.IsSpace) // Whitespace after a comment doesn't matter
	return scanStart
}

// scanText scans a string of continuous characters, stopping at the first
// whitespace character.
func scanText(s *Scanner) scanFn {
	for !unicode.IsSpace(s.char()) && s.char() != eof {
		s.advance()
	}

	text := string(s.src[s.start:s.pos])
	kind, _ := token.Method(text)
	s.emit(kind) // Method returns either the Method or Text so safe to emit either
	return scanStart
}

// scanRequestSep scans the literal '###' request separator. No '#'
// have been consumed yet but by the time this is called we know that:
//   - s.char() == '#'
//   - s.peek() == '#'
func scanRequestSep(s *Scanner) scanFn {
	// Absorb no more than 3 '#'
	count := 0
	const sepLength = 3 // len("###")
	for s.char() == '#' {
		count++
		s.advance()
		if count == sepLength {
			break
		}
	}

	s.emit(token.RequestSeparator)
	return scanStart
}

// isLineSpace reports whether r is a non line terminating whitespace character,
// imagine [unicode.IsSpace] but without '\n' or '\r'.
func isLineSpace(r rune) bool {
	return r == ' ' || r == '\t'
}
