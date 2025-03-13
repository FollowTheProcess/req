// Package parser implements the http file parser.
package parser

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/FollowTheProcess/req/internal/syntax"
	"github.com/FollowTheProcess/req/internal/syntax/scanner"
	"github.com/FollowTheProcess/req/internal/syntax/token"
)

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
	file.Vars = p.parseVars()

	// Everything else should just be parsing requests
	for p.current.Kind != token.EOF && p.current.Kind != token.Error {
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
	if !slices.Contains(kinds, p.next.Kind) {
		p.errorf("expected one of %v, got %s", kinds, p.next.Kind)
		return
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
	if p.handler == nil {
		// I guess ignore?
		return
	}

	p.handler(p.position(), msg)
	p.hadErrors = true
}

// errorf calls error with a formatted message.
func (p *Parser) errorf(format string, a ...any) {
	p.error(fmt.Sprintf(format, a...))
}

// text returns the chunk of source text described by the p.current token.
func (p *Parser) text() string {
	return string(p.src[p.current.Start:p.current.End])
}

// parseVars parses a run of variable declarations.
//
// If p.current is anything other than '@', parseVars returns nil.
func (p *Parser) parseVars() map[string]string {
	if p.current.Kind != token.At {
		return nil
	}

	vars := make(map[string]string)

	// TODO(@FollowTheProcess): Make the predefined tags special like keywords, these are:
	// @timeout = <time.Duration> (in string form)
	// @connection-timeout = <time.Duration> (in string form)
	// @no-redirect (no value, if it's present set the bool)
	//
	// Everything else can just be a user variable

	// TODO(@FollowTheProcess): Also handle dynamic variables that occur in {{}} blocks
	// See https://www.jetbrains.com/help/idea/exploring-http-syntax.html#dynamic-variables
	// would be fun if we could support all of these

	for p.current.Kind == token.At {
		p.expect(token.Ident)
		key := p.text()
		p.expect(token.Eq)
		p.expect(token.URL, token.Number, token.Text)
		value := p.text()

		vars[key] = value
		p.advance()
	}

	return vars
}

// parseRequest parses a single request in a http file.
func (p *Parser) parseRequest() syntax.Request {
	// TODO(@FollowTheProcess): Request variables
	// TODO(@FollowTheProcess): I think we're going to actually have to do most of the validation here
	// as this will be the last place we have access to the raw src and position info so this is where
	// we can point to source ranges and highlight errors.
	if p.current.Kind != token.RequestSeparator {
		p.errorf("expected %s, got %s", token.RequestSeparator, p.current.Kind)
		return syntax.Request{}
	}

	request := syntax.Request{
		Headers: make(map[string]string),
	}

	// Does it have a name as in "### {name}"
	if p.next.Kind == token.Text {
		p.advance()
		request.Name = p.text()
	}

	if !token.IsMethod(p.next.Kind) {
		p.errorf("request separators must be followed by either a name or a HTTP method, got %s", p.next.Kind)
		return syntax.Request{}
	}

	p.advance()
	request.Method = p.text()

	// TODO(@FollowTheProcess): Validate URL. We need to do that in two places
	// 1) In any global variables declaring a URL like "@base = <url>", this one must be strict and enforce an absolute URL
	// 2) Here which could either be a full URL, or use "{{base}}/items/1", in which case, we can assume base is valid
	// 	  as it's gone through 1, maybe substitute it here? And validate the whole thing
	p.expect(token.URL)
	request.URL = p.text()

	if p.next.Kind == token.HTTPVersion {
		p.advance()
		request.HTTPVersion = p.text()
	}

	// Parse any headers
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
	}

	return request
}
