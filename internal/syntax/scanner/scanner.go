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
	bom = 0xFEFF   // byte order mark, only permitted as very first character
)

// Scanner is the http file scanner.
type Scanner struct {
	name    string // Name of the file
	src     []byte // Raw source text
	start   int    // The start position of the current token
	pos     int    // Current scanner position in src (bytes, 0 indexed)
	nextPos int    // Position of next character
	line    int    // Current line number (1 indexed)
	char    rune   // The character the scanner is currently sat on
}

// New returns a new [Scanner] that reads from r.
func New(name string, r io.Reader) (*Scanner, error) {
	// .http files are small, it's fine to just read it in one go
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("could not read from input: %w", err)
	}

	scanner := &Scanner{
		name: name,
		src:  src,
		line: 1,
	}

	// Read the first char, and ignore it if it's the bom
	scanner.advance()
	if scanner.char == bom {
		scanner.advance()
	}

	return scanner, nil
}

// advance advances the scanner by a single character.
func (s *Scanner) advance() {
	if s.nextPos >= len(s.src) {
		s.char = eof
		s.pos = len(s.src)
		return
	}

	char, width := utf8.DecodeRune(s.src[s.pos:])
	if char == utf8.RuneError {
		// TODO(@FollowTheProcess): Error
		return
	}

	// Move the scanner forward
	s.pos = s.nextPos
	s.nextPos += width
	s.char = char
}

// token returns a token of a particular kind, using the scanner state
// to fill in position info.
func (s *Scanner) token(kind token.Kind) token.Token {
	return token.Token{Kind: kind, Start: s.start, End: s.pos}
}

// Scan scans the input and returns the next token.
func (s *Scanner) Scan() token.Token {
	s.start = s.pos // Start position of current token
	char := s.char  // The current char before advancing
	s.advance()

	switch char {
	case eof:
		return s.token(token.EOF)
	case '#':
		return s.token(token.Hash)
	case '/':
		return s.token(token.Slash)
	default:
		// TODO(@FollowTheProcess): Error properly
		return s.token(token.Error)
	}
}
