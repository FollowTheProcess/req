package token_test

import (
	"fmt"
	"testing"
	"testing/quick"

	"github.com/FollowTheProcess/req/internal/syntax/token"
	"github.com/FollowTheProcess/test"
)

func TestString(t *testing.T) {
	// All we really care about is the format, let's let quick handle it!
	f := func(tok token.Token) bool {
		return tok.String() == fmt.Sprintf("<Token::%s start=%d, end=%d>", tok.Kind.String(), tok.Start, tok.End)
	}
	if err := quick.Check(f, nil); err != nil {
		t.Fatal(err)
	}
}

func TestMethod(t *testing.T) {
	tests := []struct {
		text string     // Text input
		want token.Kind // Expected token Kind return
		ok   bool       // Expected ok return
	}{
		{text: "GET", want: token.MethodGet, ok: true},
		{text: "HEAD", want: token.MethodHead, ok: true},
		{text: "POST", want: token.MethodPost, ok: true},
		{text: "PUT", want: token.MethodPut, ok: true},
		{text: "DELETE", want: token.MethodDelete, ok: true},
		{text: "CONNECT", want: token.MethodConnect, ok: true},
		{text: "PATCH", want: token.MethodPatch, ok: true},
		{text: "OPTIONS", want: token.MethodOptions, ok: true},
		{text: "TRACE", want: token.MethodTrace, ok: true},
		{text: "word", want: token.Text, ok: false},
		{text: "patch", want: token.Text, ok: false},
		{text: "get", want: token.Text, ok: false},
		{text: "post", want: token.Text, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, ok := token.Method(tt.text)
			test.Equal(t, ok, tt.ok)
			test.Equal(t, got, tt.want)
		})
	}
}

func TestIsMethod(t *testing.T) {
	tests := []struct {
		kind token.Kind // Kind under test
		want bool       // Expected IsMethod return value
	}{
		{kind: token.MethodGet, want: true},
		{kind: token.MethodHead, want: true},
		{kind: token.MethodPost, want: true},
		{kind: token.MethodPut, want: true},
		{kind: token.MethodDelete, want: true},
		{kind: token.MethodConnect, want: true},
		{kind: token.MethodPatch, want: true},
		{kind: token.MethodOptions, want: true},
		{kind: token.MethodTrace, want: true},
		{kind: token.EOF, want: false},
		{kind: token.Error, want: false},
		{kind: token.Comment, want: false},
		{kind: token.Text, want: false},
		{kind: token.URL, want: false},
		{kind: token.Header, want: false},
		{kind: token.Body, want: false},
		{kind: token.Ident, want: false},
		{kind: token.RequestSeparator, want: false},
		{kind: token.At, want: false},
		{kind: token.Eq, want: false},
		{kind: token.Colon, want: false},
		{kind: token.LeftAngle, want: false},
		{kind: token.RightAngle, want: false},
		{kind: token.HTTPVersion, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			test.Equal(t, token.IsMethod(tt.kind), tt.want, test.Context("IsMethod(%s) mismatch", tt.kind))
		})
	}
}

func TestKeyword(t *testing.T) {
	tests := []struct {
		text string     // Text input
		want token.Kind // Expected token Kind return
		ok   bool       // Expected ok return
	}{
		{text: "name", want: token.Name, ok: true},
		{text: "timeout", want: token.Timeout, ok: true},
		{text: "connection-timeout", want: token.ConnectionTimeout, ok: true},
		{text: "no-redirect", want: token.NoRedirect, ok: true},
		{text: "something-else", want: token.Ident, ok: false},
		{text: "base", want: token.Ident, ok: false},
		{text: "myVar", want: token.Ident, ok: false},
		{text: "lots of random crap", want: token.Ident, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			got, ok := token.Keyword(tt.text)
			test.Equal(t, ok, tt.ok)
			test.Equal(t, got, tt.want)
		})
	}
}

func TestIsKeyword(t *testing.T) {
	tests := []struct {
		kind token.Kind // Kind under test
		want bool       // Expected IsKeyword return value
	}{
		{kind: token.Name, want: true},
		{kind: token.Timeout, want: true},
		{kind: token.ConnectionTimeout, want: true},
		{kind: token.NoRedirect, want: true},
		{kind: token.MethodGet, want: false},
		{kind: token.MethodHead, want: false},
		{kind: token.MethodPost, want: false},
		{kind: token.MethodPut, want: false},
		{kind: token.MethodDelete, want: false},
		{kind: token.MethodConnect, want: false},
		{kind: token.MethodPatch, want: false},
		{kind: token.MethodOptions, want: false},
		{kind: token.MethodTrace, want: false},
		{kind: token.EOF, want: false},
		{kind: token.Error, want: false},
		{kind: token.Comment, want: false},
		{kind: token.Text, want: false},
		{kind: token.URL, want: false},
		{kind: token.Header, want: false},
		{kind: token.Body, want: false},
		{kind: token.Ident, want: false},
		{kind: token.RequestSeparator, want: false},
		{kind: token.At, want: false},
		{kind: token.Eq, want: false},
		{kind: token.Colon, want: false},
		{kind: token.LeftAngle, want: false},
		{kind: token.RightAngle, want: false},
		{kind: token.HTTPVersion, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			test.Equal(t, token.IsKeyword(tt.kind), tt.want, test.Context("IsKeyword(%s) mismatch", tt.kind))
		})
	}
}
