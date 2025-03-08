// Package token provides the set of lexical tokens for a .http file.
package token

import "fmt"

// Kind is the kind of a token.
type Kind int

//go:generate stringer -type Kind -linecomment
const (
	EOF   Kind = iota // EOF
	Error             // Error
	Hash              // Hash
	Slash             // Slash
)

// Token is a lexical token in a .http file.
type Token struct {
	Kind  Kind // The kind of token this is
	Start int  // Byte offset from the start of the file to the first char in this token
	End   int  // Byte offset from the start of the file to the last char in this token
}

// String returns a string representation of a [Token].
func (t Token) String() string {
	return fmt.Sprintf("<Token::%s start=%d, end=%d>", t.Kind, t.Start, t.End)
}
