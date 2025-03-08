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
