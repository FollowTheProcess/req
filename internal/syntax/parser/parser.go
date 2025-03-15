// Package parser implements the http file parser.
package parser

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/scanner"
	"github.com/FollowTheProcess/req/internal/syntax/token"
)

// TODO(@FollowTheProcess): Also handle dynamic variables that occur in {{}} blocks
// See https://www.jetbrains.com/help/idea/exploring-http-syntax.html#dynamic-variables
// would be fun if we could support all of these

// ErrParse is a generic parsing error, details on the error are passed
// to the parsers [syntax.ErrorHandler] at the moment it occurs.
var ErrParse = errors.New("parse error")

// Parser is the http file parser.
type Parser struct {
	handler   syntax.ErrorHandler // The error handler
	scanner   *scanner.Scanner    // Scanner to generate tokens
	name      string              // Name of the file being parsed
	src       []byte              // Raw source text
	current   token.Token         // Current token under inspection
	next      token.Token         // Next token in the stream
	hadErrors bool                // Whether we encountered parse errors
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
	file := syntax.File{
		Name: p.name,
	}

	// Parse any globals at the top of the file
	file = p.parseGlobals(file)

	// Everything else should just be parsing requests
	for p.current.Kind != token.EOF {
		request := p.parseRequest()
		// If it's name is missing, name it after its position in the file (1 indexed)
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
	switch len(kinds) {
	case 0:
		return
	case 1:
		if p.next.Kind != kinds[0] {
			p.errorf("expected %s, got %s", kinds[0], p.next.Kind)
			return
		}
	default:
		if !slices.Contains(kinds, p.next.Kind) {
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
	return string(p.src[p.current.Start:p.current.End])
}

// parseDuration parses a duration declaration e.g. in a global or request variable.
//
// It assumes the '@ident' has already been consumed.
func (p *Parser) parseDuration() syntax.Duration {
	p.advance()
	// Can either be @timeout = 20s or @timeout 20s
	if p.next.Kind == token.Eq {
		p.advance()
	}
	p.expect(token.Text)

	duration, err := time.ParseDuration(p.text())
	if err != nil {
		p.errorf("bad timeout value: %v", err)
	}

	return syntax.Duration(duration)
}

// parseName parses a name declaration e.g. in a global or request variable.
//
// It assumes the '@name' has already been consumed.
func (p *Parser) parseName() string {
	p.advance()
	// Can either be @name = MyName or @name MyName
	if p.next.Kind == token.Eq {
		p.advance()
	}
	p.expect(token.Text)

	return p.text()
}

// parseVar parses a generic @ident = <value> in either global or request scope.
//
// It assumes the '@ident' has already been consumed.
func (p *Parser) parseVar() (key, value string, ok bool) {
	p.advance()
	key = p.text()
	p.expect(token.Eq)
	p.expect(token.URL, token.Text)
	if p.current.Kind == token.URL {
		p.validateURL(p.text())
	}
	value = p.text()

	return key, value, true
}

// validateURL validates a (possibly templated URL). The validation is on
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

// parseGlobals parses a run of variable declarations at the top of the file. Returning
// the modified syntax.File.
//
// If p.current is anything other than '@', parseGlobals returns the input file as is.
func (p *Parser) parseGlobals(file syntax.File) syntax.File {
	if p.current.Kind != token.At {
		return file
	}

	for p.current.Kind == token.At {
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
		case token.Ident:
			// Generic variable, shove it in the map, initialise the map
			// lazily as not all files will have vars
			key, value, ok := p.parseVar()
			if !ok {
				return file
			}
			if file.Vars == nil {
				file.Vars = make(map[string]string)
			}
			file.Vars[key] = value
		default:
			p.errorf(
				"unexpected token %s, expected one of %s, %s, %s, %s or %s",
				p.next.Kind,
				token.Timeout,
				token.ConnectionTimeout,
				token.NoRedirect,
				token.Name,
				token.Ident,
			)
		}

		p.advance()
	}

	return file
}

// parseRequestVars parses a run of variable declarations in a request. Returning
// the modified syntax.Request.
//
// If p.current is anything other than '@', parseRequestVars returns the request as is.
func (p *Parser) parseRequestVars(request syntax.Request) syntax.Request {
	if p.current.Kind != token.At {
		return request
	}

	for p.current.Kind == token.At {
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
		case token.Ident:
			// Generic variable, shove it in the map, initialise the map
			// lazily as not all requests will have vars
			key, value, ok := p.parseVar()
			if !ok {
				return request
			}
			if request.Vars == nil {
				request.Vars = make(map[string]string)
			}
			request.Vars[key] = value
		default:
			p.errorf(
				"unexpected token %s, expected one of %s, %s, %s or %s",
				p.next.Kind,
				token.Timeout,
				token.ConnectionTimeout,
				token.NoRedirect,
				token.Ident,
			)
		}
		p.advance()
	}

	return request
}

// parseRequest parses a single request in a http file.
func (p *Parser) parseRequest() syntax.Request {
	if p.current.Kind != token.RequestSeparator {
		p.errorf("expected %s, got %s", token.RequestSeparator, p.current.Kind)
		return syntax.Request{}
	}

	request := syntax.Request{}

	// Does it have a name as in "### {name}"
	if p.next.Kind == token.Text {
		p.advance()
		request.Name = p.text()
	}

	p.advance()
	request = p.parseRequestVars(request)

	if !token.IsMethod(p.current.Kind) {
		p.errorf("request separators must be followed by either a name or a HTTP method, got %s", p.current.Kind)
		return syntax.Request{}
	}

	request.Method = p.text()

	p.expect(token.URL)
	p.validateURL(p.text())

	request.URL = p.text()

	if p.next.Kind == token.HTTPVersion {
		p.advance()
		request.HTTPVersion = p.text()
	}

	// Parse any headers, again initialising the map lazily
	// although in fairness most requests will likely have headers
	if p.next.Kind == token.Header {
		if request.Headers == nil {
			request.Headers = make(map[string]string)
		}
	}

	for p.next.Kind == token.Header {
		p.advance()
		key := p.text()
		p.expect(token.Colon)
		p.expect(token.Text)
		value := p.text()
		request.Headers[key] = value
	}

	// Only things allowed now are:
	// - Body (in which case request.Body gets the raw bytes)
	// - LeftAngle (in which case the next thing must be Text and is BodyFile)
	// - LeftAngle then RightAngle (in which case it's a response reference)
	if p.next.Kind == token.Body {
		p.advance()
		request.Body = p.src[p.current.Start:p.current.End]
	}

	// Might be a < ./input.json in a POST request
	// Or it could be a <> ./previous.200.json in a request with no body
	if p.next.Kind == token.LeftAngle {
		p.advance()
		if p.next.Kind == token.RightAngle {
			p.advance()
			p.expect(token.Text)
			request.ResponseRef = p.text()
		} else {
			p.expect(token.Text)
			request.BodyFile = p.text()
		}
	}

	// We have to check for the <> ./previous.200.json case again in case
	// the body was set with < ./input.json *and* we want a response reference
	if p.next.Kind == token.LeftAngle {
		p.advance()
		p.expect(token.RightAngle)
		p.expect(token.Text)
		request.ResponseRef = p.text()
	}

	if request.Body != nil && request.BodyFile != "" {
		p.error("cannot have both an inline body and an input body file")
		return syntax.Request{}
	}

	return request
}
