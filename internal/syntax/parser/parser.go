// Package parser implements the .http file parser.
package parser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"go.followtheprocess.codes/req/internal/syntax"
	"go.followtheprocess.codes/req/internal/syntax/scanner"
	"go.followtheprocess.codes/req/internal/syntax/token"
)

// ErrParse is a generic parsing error, details on the error are passed
// to the parser's [syntax.ErrorHandler] at the moment it occurs.
var ErrParse = errors.New("parse error")

// Parser is the http file parser.
type Parser struct {
	handler   syntax.ErrorHandler // The installed error handler, to be called in response to parse errors
	scanner   *scanner.Scanner    // Scanner to produce tokens
	name      string              // Name of the file being parsed
	src       []byte              // Raw source text
	current   token.Token         // Current token under inspection
	next      token.Token         // Next token in the stream
	hadErrors bool                // Whether we encountered parse errors
}

// New initialises and returns a new [Parser] that reads from r.
func New(name string, r io.Reader, handler syntax.ErrorHandler) (*Parser, error) {
	// .http files are small, it's okay to read the whole thing
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read from input: %w", err)
	}

	p := &Parser{
		handler: handler,
		scanner: scanner.New(name, src, handler),
		name:    name,
		src:     src,
	}

	// Read 2 tokens so current and next are set
	p.advance()
	p.advance()

	return p, nil
}

// Parse parses the file to completion returning a [syntax.File] and any parsing errors.
//
// The returned error will simply signify whether or not there were parse errors,
// the installed error handler passed to [New] will have the full detail and should
// be preferred.
func (p *Parser) Parse() (syntax.File, error) {
	file := syntax.File{
		Name: p.name,
	}

	// Parse any global at the top of the file
	file = p.parseGlobals(file)

	for !p.current.Is(token.EOF) {
		if p.current.Is(token.Error) {
			// An error from the scanner
			return syntax.File{}, ErrParse
		}

		request := p.parseRequest()

		// If it's name is missing, name it after it's position in the file (1 indexed)
		if request.Name == "" {
			request.Name = fmt.Sprintf("#%d", 1+len(file.Requests))
		}

		file.Requests = append(file.Requests, request)

		p.advance()
	}

	if p.hadErrors {
		return syntax.File{}, ErrParse
	}

	return file, nil
}

// advance advances the parser by a single token.
func (p *Parser) advance() {
	p.current = p.next
	p.next = p.scanner.Scan()
}

// expect asserts that the next token is one of the given kinds, emitting a syntax error if not.
//
// The parser is advanced only if the next token is of one of these kinds such that after returning
// p.current will be one of the kinds.
func (p *Parser) expect(kinds ...token.Kind) {
	if p.next.Is(token.Error) {
		// Nobody expects an error!
		// But seriously, this means the scanner has emitted an error and has already
		// passed it to the error handler
		return
	}

	switch len(kinds) {
	case 0:
		return
	case 1:
		if !p.next.Is(kinds[0]) {
			p.errorf("expected %s, got %s", kinds[0], p.next.Kind)
			return
		}
	default:
		if !p.next.Is(kinds...) {
			p.errorf("expected one of %v, got %s", kinds, p.next.Kind)
			return
		}
	}

	p.advance()
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
	if p.next.Is(token.EOF) {
		start = p.current.End
	}

	end := p.current.End

	// The column is therefore the number of bytes between the end of the last newline
	// and the current position, +1 because editors columns start at 1. Applying this
	// correction here means you can click a syntax error in the terminal and be
	// taken to a precise location in an editor which is probably what we want to happen
	startCol := 1 + start - lastNewLineOffset
	endCol := 1 + end - lastNewLineOffset

	return syntax.Position{
		Name:     p.name,
		Offset:   p.current.Start,
		Line:     line,
		StartCol: startCol,
		EndCol:   endCol,
	}
}

// error calculates the current position and calls the installed error handler
// with the correct information.
func (p *Parser) error(msg string) {
	p.hadErrors = true

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

// text returns the chunk of source text described by the p.current token.
func (p *Parser) text() string {
	return strings.TrimSpace(string(p.src[p.current.Start:p.current.End]))
}

// parseGlobals parses a run of variable declarations at the top of the file, returning
// the modified [syntax.File].
//
// If p.current is anything other than '@', the input file is returned as is.
func (p *Parser) parseGlobals(file syntax.File) syntax.File {
	if !p.current.Is(token.At) {
		return file
	}

	for p.current.Is(token.At) {
		switch p.next.Kind {
		case token.Timeout:
			file.Timeout = p.parseDuration()
		case token.ConnectionTimeout:
			file.ConnectionTimeout = p.parseDuration()
		case token.NoRedirect:
			p.advance()

			file.NoRedirect = true
		case token.Name:
			file.Name = p.parseName()
		case token.Prompt:
			file.Prompts = append(file.Prompts, p.parsePrompt())
		case token.Ident:
			// Generic variable, shove it in the map which we now lazily initialise
			// because not every request will have vars
			key, value := p.parseVar()

			if file.Vars == nil {
				file.Vars = make(map[string]string)
			}

			file.Vars[key] = value
		default:
			p.expect(
				token.Timeout,
				token.ConnectionTimeout,
				token.NoRedirect,
				token.Name,
				token.Prompt,
				token.Ident,
			)
		}

		p.advance()
	}

	return file
}

// parseRequest parses a single request in a http file.
func (p *Parser) parseRequest() syntax.Request {
	if !p.current.Is(token.Separator) {
		p.errorf("expected %s, got %s", token.Separator, p.current.Kind)
		return syntax.Request{}
	}

	request := syntax.Request{}

	// Does it have a comment as in "### [comment]"
	if p.next.Is(token.Comment) {
		p.advance()
		request.Comment = p.text()
	}

	p.advance()
	request = p.parseRequestVars(request)

	if !token.IsMethod(p.current.Kind) {
		p.errorf("request separators must be followed by either a comment or a HTTP method, got %s: %q", p.current.Kind, p.text())
		return syntax.Request{}
	}

	request.Method = p.text()

	p.expect(token.URL)
	p.validateURL(p.text())

	request.URL = p.text()

	if p.next.Is(token.HTTPVersion) {
		p.advance()
		request.HTTPVersion = p.text()
	}

	// Now any headers, initialising the map lazily although in fairness
	// its likely that most requests will have headers
	if p.next.Is(token.Header) {
		if request.Headers == nil {
			request.Headers = make(map[string]string)
		}
	}

	for p.next.Is(token.Header) {
		p.advance()
		key := p.text()
		p.expect(token.Colon)
		p.expect(token.Text)
		value := p.text()
		request.Headers[key] = value
	}

	// Do we have a request body inline?
	if p.next.Is(token.Body) {
		p.advance()
		request.Body = bytes.TrimSpace(p.src[p.current.Start:p.current.End])
	}

	// Might be a '< ./body.json'
	if p.next.Is(token.LeftAngle) {
		p.advance()
		p.expect(token.Text)
		request.BodyFile = p.text()
	}

	// We could now also have a response redirect
	// e.g '> ./response.json'
	if p.next.Is(token.RightAngle) {
		p.advance()
		p.expect(token.Text)
		request.ResponseFile = p.text()
	}

	return request
}

// parseRequestVars parses a run of variable declarations in a request. Returning
// the modified [syntax.Request].
//
// If p.current is anything other than '@', the request is returned as is.
func (p *Parser) parseRequestVars(request syntax.Request) syntax.Request {
	if !p.current.Is(token.At) {
		return request
	}

	for p.current.Is(token.At) {
		switch p.next.Kind {
		case token.Timeout:
			request.Timeout = p.parseDuration()
		case token.ConnectionTimeout:
			request.ConnectionTimeout = p.parseDuration()
		case token.NoRedirect:
			p.advance()

			request.NoRedirect = true
		case token.Name:
			request.Name = p.parseName()
		case token.Prompt:
			request.Prompts = append(request.Prompts, p.parsePrompt())
		case token.Ident:
			// Generic variable, shove it in the map which we now lazily initialise
			// because not every request will have vars
			key, value := p.parseVar()

			if request.Vars == nil {
				request.Vars = make(map[string]string)
			}

			request.Vars[key] = value
		default:
			p.expect(
				token.Timeout,
				token.ConnectionTimeout,
				token.NoRedirect,
				token.Name,
				token.Prompt,
				token.Ident,
			)
		}

		p.advance()
	}

	return request
}

// parseDuration parses a duration declaration e.g. in a global or request variable.
func (p *Parser) parseDuration() time.Duration {
	p.advance()
	// Can either be @timeout = 20s or @timeout 20s
	if p.next.Is(token.Eq) {
		p.advance()
	}

	p.expect(token.Text)

	duration, err := time.ParseDuration(p.text())
	if err != nil {
		p.errorf("bad timeout value: %v", err)
	}

	return duration
}

// parseName parses a name declaration e.g. in a global or request variable.
func (p *Parser) parseName() string {
	p.advance()
	// Can either be @name = MyName or @name MyName
	if p.next.Is(token.Eq) {
		p.advance()
	}

	p.expect(token.Text)

	return p.text()
}

// parsePrompt parses a prompt declaration e.g. in a global or request variable.
func (p *Parser) parsePrompt() syntax.Prompt {
	p.advance()

	p.expect(token.Ident)
	name := p.text()

	p.expect(token.Text)
	description := p.text()

	prompt := syntax.Prompt{
		Name:        name,
		Description: description,
	}

	return prompt
}

// parseVar parses a generic '@ident = <value>' in either global or request scope.
func (p *Parser) parseVar() (key, value string) {
	p.advance()
	key = p.text()
	// Can either be @ident = value or @ident value
	if p.next.Is(token.Eq) {
		p.advance()
	}

	p.expect(token.URL, token.Text)

	if p.current.Is(token.URL) {
		p.validateURL(p.text())
	}

	value = p.text()

	return key, value
}

// validateURL validates a (possibly templated) URL. The validation is on
// a best effort basis.
func (p *Parser) validateURL(raw string) {
	if strings.Contains(raw, "{{") {
		// It's a partially templated URL, so we can't be too strict
		if _, err := url.Parse(raw); err != nil {
			p.errorf("invalid URL: %v", err)
		}
	} else {
		// If it's not templated it must be a fully valid URL
		if _, err := url.ParseRequestURI(raw); err != nil {
			p.errorf("invalid URL: %v", err)
		}
	}
}
