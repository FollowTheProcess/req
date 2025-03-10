// Package scanner implements the lexical scanner for .http files.
package scanner

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/token"
)

const (
	eof = rune(-1) // eof signifies we have reached the end of the input
)

// scanFn represents the state of the scanner as a function that returns the next state.
type scanFn func(*Scanner) scanFn

// Scanner is the http file scanner.
type Scanner struct {
	handler   syntax.ErrorHandler // The error handler, if any
	tokens    chan token.Token    // Channel on which to emit scanned tokens
	name      string              // Name of the file
	src       []byte              // Raw source text
	start     int                 // The start position of the current token
	pos       int                 // Current scanner position in src (bytes, 0 indexed)
	line      int                 // Current line number (1 indexed)
	lineStart int                 // Offset at which the current line started
	width     int                 // Width of the last rune read from input, so we can backup
	wg        sync.WaitGroup      // handler gets run in a goroutine so it doesn't block the main state machine
}

// New returns a new [Scanner] that reads from r.
func New(name string, r io.Reader, handler syntax.ErrorHandler) (*Scanner, error) {
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

// rest returns the rest of src, starting from the current position.
func (s *Scanner) rest() []byte {
	if s.pos >= len(s.src) {
		return nil
	}
	return s.src[s.pos:]
}

// skip ignores any characters for which the predicate returns true, stopping at the
// first one that returns false such that after it returns, s.advance returns the
// first 'false' char.
//
// The scanner start position is brought up to the current position before returning.
func (s *Scanner) skip(predicate func(r rune) bool) {
	for predicate(s.char()) {
		s.advance()
	}
	s.start = s.pos
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

	position := syntax.Position{
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
	case '@':
		return scanAt
	case '=':
		return scanEq
	case ':':
		return scanColon
	default:
		switch {
		case bytes.HasPrefix(s.rest(), []byte("HTTP")):
			return scanHTTPVersion
		case bytes.HasPrefix(s.rest(), []byte("http")):
			return scanURL
		case isAlpha(char):
			return scanText
		case isDigit(char):
			return scanNumber
		default:
			s.emit(token.Error)
			s.errorf("unexpected token %q", string(s.char()))
			return nil
		}
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
		s.advance()
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
	kind, method := token.Method(text)
	if method {
		// GET {space but not \n} <url> [HTTP Version]
		s.emit(kind)
		s.skip(isLineSpace)
		return scanStart
	}

	s.emit(kind)
	s.skip(unicode.IsSpace)
	return scanStart
}

// scanURL scans a URL, which for now we assume is anything that isn't
// whitespace.
func scanURL(s *Scanner) scanFn {
	for !unicode.IsSpace(s.char()) && s.char() != eof {
		s.advance()
	}

	s.emit(token.URL)
	s.skip(unicode.IsSpace)
	return scanStart
}

// scanRequestSep scans the literal '###' request separator. No '#'
// have been consumed yet but by the time this is called we know that:
//   - s.char() == '#'
//   - s.peek() == '#'
//
// A request separator may either be followed by a '\n' or
// a line of arbitrary text which is the name of the request.
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

	// If we have any text on the same line, it's the request name
	s.skip(isLineSpace)

	if isAlpha(s.char()) {
		return scanRequestName
	}

	s.skip(unicode.IsSpace)
	return scanStart
}

// scanRequestName scans the name of a request after the separator '###'.
func scanRequestName(s *Scanner) scanFn {
	s.skip(unicode.IsSpace)

	// Scan the request name which is any char up until
	// the next '\n' or eof.
	for s.char() != '\n' && s.char() != eof {
		s.advance()
	}

	s.emit(token.Text)
	s.skip(unicode.IsSpace)
	return scanStart
}

// scanAt scans a '@' character.
func scanAt(s *Scanner) scanFn {
	s.advance() // Consume the '@'
	s.emit(token.At)

	if isAlpha(s.char()) {
		return scanIdent
	}

	return scanStart
}

// scanIdent scans an identifier.
func scanIdent(s *Scanner) scanFn {
	for isIdent(s.char()) {
		s.advance()
	}

	s.emit(token.Ident)
	s.skip(unicode.IsSpace)
	return scanStart
}

// scanEq scans a '=' character.
func scanEq(s *Scanner) scanFn {
	s.advance() // Consume the '='
	s.emit(token.Eq)
	s.skip(isLineSpace)
	return scanStart
}

// scanColon scans a ':' character.
func scanColon(s *Scanner) scanFn {
	s.advance() // ':'
	s.emit(token.Colon)
	s.skip(isLineSpace)
	return scanStart
}

// scanNumber scans a number literal.
func scanNumber(s *Scanner) scanFn {
	for isDigit(s.char()) {
		s.advance()

		if s.char() == '.' {
			s.advance() // Consume the '.'
			if !isDigit(s.char()) {
				s.error("bad number literal")
				return nil
			}
			for isDigit(s.char()) {
				s.advance()
			}
		}
	}

	s.emit(token.Number)
	s.skip(unicode.IsSpace)
	return scanStart
}

// scanHTTPVersion scans a HTTP version declaration.
//
// The next characters in s.src are known to be 'HTTP'.
func scanHTTPVersion(s *Scanner) scanFn {
	const httpLen = 4 // len("HTTP")
	for range httpLen {
		s.advance()
	}

	if s.char() != '/' {
		s.errorf("bad HTTP version character. expected %q got %q", "/", string(s.char()))
		return nil
	}

	s.advance() // Consume the '/'
	s.emit(token.HTTPVersion)
	return scanNumber
}

// isLineSpace reports whether r is a non line terminating whitespace character,
// imagine [unicode.IsSpace] but without '\n' or '\r'.
func isLineSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isAlpha reports whether r is an alpha character.
func isAlpha(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// isIdent reports whether r is a valid identifier character.
func isIdent(r rune) bool {
	return isAlpha(r) || isDigit(r) || r == '_' || r == '-'
}

// isDigit reports whether r is a valid ASCII digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
