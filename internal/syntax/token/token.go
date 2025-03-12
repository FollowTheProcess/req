// Package token provides the set of lexical tokens for a .http file.
package token

import "fmt"

// Kind is the kind of a token.
type Kind int

//go:generate stringer -type Kind -linecomment
const (
	EOF              Kind = iota // EOF
	Error                        // Error
	Comment                      // Comment
	Text                         // Text
	Number                       // Number
	URL                          // URL
	Header                       // Header
	Body                         // Body
	Ident                        // Ident
	RequestSeparator             // RequestSeparator
	At                           // At
	Eq                           // Eq
	Colon                        // Colon
	LeftAngle                    // LeftAngle
	RightAngle                   // RightAngle
	HTTPVersion                  // HTTPVersion
	MethodGet                    // MethodGet
	MethodHead                   // MethodHead
	MethodPost                   // MethodPost
	MethodPut                    // MethodPut
	MethodDelete                 // MethodDelete
	MethodConnect                // MethodConnect
	MethodPatch                  // MethodPatch
	MethodOptions                // MethodOptions
	MethodTrace                  // MethodTrace
)

// Token is a lexical token in a .http file.
type Token struct {
	Kind  Kind // The kind of token this is
	Start int  // Byte offset from the start of the file to the start of this token
	End   int  // Byte offset from the start of the file to the end of this token
}

// String returns a string representation of a [Token].
func (t Token) String() string {
	return fmt.Sprintf("<Token::%s start=%d, end=%d>", t.Kind, t.Start, t.End)
}

// Method reports whether a string refers to a HTTP method, returning it's
// [Kind] and true if it is. Otherwise [Text] and false are returned.
func Method(text string) (kind Kind, ok bool) {
	switch text {
	case "GET":
		return MethodGet, true
	case "HEAD":
		return MethodHead, true
	case "POST":
		return MethodPost, true
	case "PUT":
		return MethodPut, true
	case "DELETE":
		return MethodDelete, true
	case "CONNECT":
		return MethodConnect, true
	case "PATCH":
		return MethodPatch, true
	case "OPTIONS":
		return MethodOptions, true
	case "TRACE":
		return MethodTrace, true
	default:
		return Text, false
	}
}

// IsMethod reports whether the given kind is a HTTP Method.
func IsMethod(kind Kind) bool {
	return kind >= MethodGet && kind <= MethodTrace
}
