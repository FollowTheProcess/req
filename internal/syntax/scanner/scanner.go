// Package scanner implement a lexical scanner for .http files, reading the raw source text
// and emitting a stream of tokens.
//
// Unlike parsing a general purpose programming language, the .http file syntax is very context
// dependent and there's not a lot of symbols or structure to distinguish these contexts from
// one another e.g. a programming language may have braces, brackets etc. where as in .http files
// it's a bit more implicit.
//
// For instance a HTTP header just looks like regular text, there are no quotes to distinguish
// it as a string.
//
// Because of this, the scanner implemented here is context dependent and will only allow certain
// tokens in certain contexts. This adds a bit of complexity but reduces it in the parser, as with
// everything, this is a trade off.
//
// In the [spec] it treats certain whitespace as significant e.g. a GET <url> must be followed by a single
// newline '\n' or '\r\n'. In this implementation whitespace is entirely ignored. This produces a more
// robust implementation as it can handle discrepancies in formatting.
//
// [spec]: https://github.com/JetBrains/http-request-in-editor-spec
package scanner

import (
	"bytes"
	"fmt"
	"iter"
	"slices"
	"unicode"
	"unicode/utf8"

	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/req/internal/syntax/token"
)

const (
	eof        = rune(-1) // eof signifies we have reached the end of the input.
	bufferSize = 32       // benchmarks suggest this is the optimum token channel buffer size
)

// scanFn represents the state of the scanner as a function that does the work
// associated with the current state, then returns the next state.
type scanFn func(*Scanner) scanFn

// Scanner is the http file scanner.
type Scanner struct {
	handler           syntax.ErrorHandler // The installed error handler, to be called in response to scanning errors
	tokens            chan token.Token    // Channel on which to emit scanned tokens
	name              string              // Name of the file
	src               []byte              // Raw source text
	start             int                 // The start position of the current token
	pos               int                 // Current scanner position in src (bytes, 0 indexed)
	line              int                 // Current line number, 1 indexed
	currentLineOffset int                 // Offset at which the current line started
}

// New returns a new [Scanner] and kicks off the state machine in a goroutine.
func New(name string, src []byte, handler syntax.ErrorHandler) *Scanner {
	s := &Scanner{
		handler: handler,
		tokens:  make(chan token.Token, bufferSize),
		name:    name,
		src:     src,
		line:    1,
	}

	// run terminates when the scanning state machine is finished and all the
	// tokens are drained from s.tokens, so no other synchronisation needed here
	go s.run()

	return s
}

// Scan scans the input and returns the next token.
func (s *Scanner) Scan() token.Token {
	return <-s.tokens
}

// All returns an iterator over the tokens in the file, stopping at EOF or Error.
//
// The final token will still be yielded.
func (s *Scanner) All() iter.Seq[token.Token] {
	return func(yield func(token.Token) bool) {
		for {
			tok, ok := <-s.tokens
			if !ok || !yield(tok) {
				return
			}
		}
	}
}

// next returns the next utf8 rune in the input, or [eof], and advances the scanner
// over that rune such that successive calls to [Scanner.next] iterate through
// src one rune at a time.
func (s *Scanner) next() rune {
	if s.pos >= len(s.src) {
		return eof
	}

	char, width := utf8.DecodeRune(s.src[s.pos:])
	s.pos += width

	if char == '\n' {
		s.line++
		s.currentLineOffset = s.pos
	}

	return char
}

// peek returns the next utf8 rune in the input, or [eof], but does not
// advance the scanner.
//
// Successive calls to peek simply return the same rune again and again.
func (s *Scanner) peek() rune {
	if s.pos >= len(s.src) {
		return eof
	}

	char, _ := utf8.DecodeRune(s.src[s.pos:])

	return char
}

// skip ignores any characters for which the predicate returns true, stopping at the
// first one that returns false such that after it returns, [Scanner.next] returns the
// first 'false' char.
//
// The scanner start position is brought up to the current position before returning, effectively
// ignoring everything it's travelled over in the meantime.
func (s *Scanner) skip(predicate func(r rune) bool) {
	for predicate(s.peek()) {
		s.next()
	}

	s.start = s.pos
}

// takeWhile consumes characters so long as the predicate returns true, stopping at the
// first one that returns false such that after it returns, [Scanner.next] returns the first 'false' rune.
func (s *Scanner) takeWhile(predicate func(r rune) bool) {
	for predicate(s.peek()) {
		s.next()
	}
}

// takeUntil consumes characters until it hits any of the specified runes.
//
// It stops before it consumes the first specified rune such that after it returns,
// the next call to [Scanner.next] returns the offending rune.
//
//	s.takeUntil('\n', '\t') // Consume runes until you hit a newline or a tab
func (s *Scanner) takeUntil(runes ...rune) {
	for {
		next := s.peek()
		if slices.Contains(runes, next) {
			return
		}
		// Otherwise, advance the scanner
		s.next()
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
// state until one returns nil (typically in response to an error or eof), at which point the tokens channel
// is closed as a signal to the receiver that no more tokens will be sent.
func (s *Scanner) run() {
	for state := scanStart; state != nil; {
		state = state(s)
	}

	close(s.tokens)
}

// error calculates the position information and calls the installed error handler
// with the information, emitting an error token in the process.
func (s *Scanner) error(msg string) {
	// So that even if there is no handler installed, we still know something
	// went wrong
	s.emit(token.Error)

	if s.handler == nil {
		// Nothing more to do
		return
	}

	// Column is the number of bytes between the last newline and the current position
	// +1 because columns are 1 indexed
	startCol := 1 + s.start - s.currentLineOffset
	endCol := 1 + s.pos - s.currentLineOffset

	position := syntax.Position{
		Name:     s.name,
		Offset:   s.pos,
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

// scanStart is the initial state of the scanner.
//
// At the start (or top) state, the only things that can be encountered in a valid .http
// file are:
//
//   - '#' for comments and request separators
//   - '/' for comments
//   - '@' for global variables
//
// Everything else must only appear in certain contexts e.g. HTTP methods may *only* appear
// immediately after a separator. HTTP versions may *only* appear after a URL etc.
//
// Whitespace is ignored.
func scanStart(s *Scanner) scanFn {
	s.skip(unicode.IsSpace)

	switch char := s.next(); char {
	case eof:
		s.emit(token.EOF)
		return nil
	case utf8.RuneError:
		s.errorf("invalid utf8 character: %U", char)
		return nil
	case '#':
		return scanHash
	case '/':
		return scanSlash
	case '@':
		return scanAt
	default:
		switch {
		case isIdent(char):
			return scanText
		default:
			s.errorf("unrecognised character: %q", char)
			return nil
		}
	}
}

// scanHash scans a '#' character, either as the beginning token for a comment
// or as the first character of a '###' request separator.
func scanHash(s *Scanner) scanFn {
	if s.peek() == '#' {
		// It's a request separator
		return scanSeparator
	}

	return scanComment
}

// scanComment scans a line comment started by either a '#' or '//'.
//
// The comment opening character(s) have already been consumed.
func scanComment(s *Scanner) scanFn {
	// It must be a comment, skip any leading whitespace between the marker
	// and the comment text
	s.skip(isLineSpace)

	// Requests may have '{//|#} @ident [=] <text>' to set request-scoped
	// variables
	if s.peek() == '@' {
		s.next() // Consume the '@'
		return scanAt
	}

	// Absorb everything until the end of the line or eof
	s.takeUntil('\n', eof)

	s.emit(token.Comment)

	return scanStart
}

// scanSlash scans a '/' character, either as the beginning of a '//' style comment
// or as part of some other text we don't care about.
func scanSlash(s *Scanner) scanFn {
	if s.peek() != '/' {
		// Ignore
		s.next()
		return scanStart
	}

	s.next() // Consume the second '/'

	return scanComment
}

// scanSeparator scans the literal '###' used as a request separator.
func scanSeparator(s *Scanner) scanFn {
	// Absorb no more than 3 '#'
	count := 0

	const sepLength = 3 // len("###")

	for s.peek() == '#' {
		count++

		s.next()

		if count == sepLength {
			break
		}
	}

	s.emit(token.Separator)

	// If there is text on the same line as the separator it is a request comment
	s.skip(isLineSpace)

	if s.peek() != '\n' && s.peek() != eof {
		return scanComment
	}

	return scanStart
}

// scanAt scans an '@' character, used to declare a variable either globally
// scoped at the top level `@name = thing` or request scoped `# @name = thing`.
func scanAt(s *Scanner) scanFn {
	s.emit(token.At)

	if bytes.HasPrefix(s.src[s.pos:], []byte("http")) {
		return scanURL
	}

	if isAlpha(s.peek()) {
		// @ident [=] <value>
		return scanIdent
	}

	return scanStart
}

// scanIdent scans an ident, that is; a continuous sequence of
// characters that are valid as an identifier.
func scanIdent(s *Scanner) scanFn {
	s.takeWhile(isIdent)

	// Is it a keyword? If so token.Keyword will return it's
	// proper token type, else [token.Ident].
	// Either way we need to emit it and then check for an optional '='
	text := string(s.src[s.start:s.pos])
	kind, _ := token.Keyword(text)
	s.emit(kind)
	s.skip(isLineSpace)

	switch {
	case kind == token.Prompt:
		// Prompts are handled in a special way as you may have e.g.
		// @prompt username <Arbitrary description on a single line>
		return scanPrompt
	case s.peek() == '=':
		// @var = value
		return scanEq
	case isAlphaNumeric(s.peek()):
		// @var value
		return scanText
	default:
		return scanStart
	}
}

// scanPrompt scans a variable prompt e.g. @prompt username [description]\n.
//
// The '@' and the 'prompt' have already been consumed and their tokens emitted.
func scanPrompt(s *Scanner) scanFn {
	// The first thing up should be the name of the variable we're prompting
	// for, which follows standard ident rules
	s.takeWhile(isIdent)
	s.emit(token.Ident)

	// Arbitrary space is allowed so long as it's on the same line
	s.skip(isLineSpace)

	// If the next thing looks like regular text, this is the optional
	// description for the prompt
	if isAlphaNumeric(s.peek()) {
		s.takeUntil('\n', eof)
		s.emit(token.Text)
	}

	return scanStart
}

// scanEq scans a '=' character, as used in a variable declaration.
func scanEq(s *Scanner) scanFn {
	s.next()
	s.emit(token.Eq)

	s.skip(isLineSpace)

	if bytes.HasPrefix(s.src[s.pos:], []byte("http")) {
		return scanURL
	}

	if isAlphaNumeric(s.peek()) {
		return scanText
	}

	return scanStart
}

// scanText scans a series of continuous text characters (no whitespace).
func scanText(s *Scanner) scanFn {
	s.takeWhile(isText)

	// Is it a HTTP Method? If so token.Method will return it's
	// proper token type, else [token.Text].
	text := string(s.src[s.start:s.pos])
	kind, wasMethod := token.Method(text)
	s.emit(kind)
	s.skip(isLineSpace)

	// If it was a HTTP method, we should now have a url following it
	if wasMethod {
		return scanURL
	}

	return scanStart
}

// scanURL scans a series continuous characters (no whitespace) and emits a URL token.
func scanURL(s *Scanner) scanFn {
	// It might also be an interpolation
	if !bytes.HasPrefix(s.src[s.pos:], []byte("http")) && !bytes.HasPrefix(s.src[s.pos:], []byte("{{")) {
		s.errorf("HTTP methods must be followed by a valid URL")
		return nil
	}

	s.takeWhile(isText)
	s.emit(token.URL)

	// Does it have a http version after it?
	s.skip(isLineSpace)

	if bytes.HasPrefix(s.src[s.pos:], []byte("HTTP/")) {
		return scanHTTPVersion
	}

	// Is the next thing headers?
	s.skip(unicode.IsSpace)

	if isAlpha(s.peek()) {
		return scanHeaders
	}

	// Either another request or the end
	if s.peek() == '#' || s.peek() == eof {
		return scanStart
	}

	return scanBody
}

// scanHTTPVersion scans a HTTP/<version> literal.
//
// The next characters are known to be 'HTTP/', this function consumes the entire
// thing e.g. 'HTTP/1.2' or 'HTTP/2'.
func scanHTTPVersion(s *Scanner) scanFn {
	const httpLen = 5 // len("HTTP/")
	for range httpLen {
		s.next()
	}

	for isDigit(s.peek()) {
		s.next()

		if s.peek() == '.' {
			s.next() // Consume the '.'
			// Now what follows *must* be a digit or it's malformed
			if !isDigit(s.peek()) {
				s.errorf("bad number literal in HTTP version, illegal char %q", s.peek())
				return nil
			}
			// Consume any remaining digits
			s.takeWhile(isDigit)
		}
	}

	s.emit(token.HTTPVersion)

	// The only thing allowed to follow a HTTP Version is a list of headers
	// or a request body
	s.skip(unicode.IsSpace)

	if isAlpha(s.peek()) {
		// Headers
		return scanHeaders
	}

	// Either another request or the end
	if s.peek() == '#' || s.peek() == eof {
		return scanStart
	}

	// Scan the body as just raw text, we then use the 'Content-Type' header to
	// figure out what it should actually be later on during parsing/eval.
	return scanBody
}

// scanHeaders scans a series of HTTP headers, one per line, emitting
// tokens as it goes.
//
// It stops when it sees the next character is not a valid ident character and so
// could not be another header.
func scanHeaders(s *Scanner) scanFn {
	s.takeWhile(isIdent)

	// Header without a colon or value e.g. 'Content-Type'
	// this is unfinished so is an error, like an unterminated string literal almost.
	if s.peek() == eof {
		s.error("unexpected eof")
		return nil
	}

	s.emit(token.Header)

	if s.peek() != ':' {
		s.errorf("expected ':' got %q", s.peek())
		return nil
	}

	s.next() // Consume the ':' we now know exists
	s.emit(token.Colon)
	s.skip(isLineSpace)

	// The value is just arbitrary text until the end of the line
	s.takeUntil('\n', eof)
	s.emit(token.Text)

	// Now for the fun bit, call itself if there are more headers
	s.skip(unicode.IsSpace)

	if isAlpha(s.peek()) {
		return scanHeaders
	}

	// After headers is either a body, another request separator, or eof
	if s.peek() == '#' || s.peek() == eof {
		return scanStart
	}

	// Must be a body
	return scanBody
}

// scanBody scans a HTTP request body, in a variety of forms:
//
//   - '< {filepath}' (Reading the request body from the file)
//   - raw text body
func scanBody(s *Scanner) scanFn {
	if s.peek() == '<' {
		return scanLeftAngle
	}

	// Are we redirecting the response, without specifying a body
	// e.g. in a GET request, there is no body but we still might redirect
	// the response
	if s.peek() == '>' {
		return scanRightAngle
	}

	// Scan raw body as a single token
	s.takeUntil('#', eof, '>')
	s.emit(token.Body)

	s.skip(unicode.IsSpace)

	// Are we redirecting the response *after* a body has been specified
	// e.g. in a POST request, there may be a body *and* a redirect
	if s.peek() == '>' {
		return scanRightAngle
	}

	return scanStart
}

// scanLeftAngle scans a '<' literal in the context of a request body
// read from file.
func scanLeftAngle(s *Scanner) scanFn {
	s.next() // Consume the '<'
	s.emit(token.LeftAngle)

	s.skip(isLineSpace)

	if isFilePath(s.peek()) {
		s.takeWhile(isText)
		s.emit(token.Text)
	}

	s.skip(unicode.IsSpace)

	// Are we redirecting the response *after* a body has been specified by a file
	if s.peek() == '>' {
		return scanRightAngle
	}

	return scanStart
}

// scanRightAngle scans a '>' literal in the context of a response redirect
// to a local file.
func scanRightAngle(s *Scanner) scanFn {
	s.next() // Consume the '>'
	s.emit(token.RightAngle)

	s.skip(isLineSpace)

	if isFilePath(s.peek()) {
		s.takeWhile(isText)
		s.emit(token.Text)
	}

	return scanStart
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

// isAlphaNumeric reports whether r is an alpha-numeric character.
func isAlphaNumeric(r rune) bool {
	return isAlpha(r) || isDigit(r)
}

// isIdent reports whether r is a valid identifier character.
func isIdent(r rune) bool {
	return isAlpha(r) || isDigit(r) || r == '_' || r == '-'
}

// isText reports whether r is valid in a continuous string of text.
func isText(r rune) bool {
	return !unicode.IsSpace(r) && r != eof
}

// isDigit reports whether r is a valid ASCII digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isFilePath reports whether r could be a valid first character in a filepath.
func isFilePath(r rune) bool {
	return isIdent(r) || r == '.' || r == '/' || r == '\\'
}
